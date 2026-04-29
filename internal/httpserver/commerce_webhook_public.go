package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// mountCommercePublicWebhookPost registers PSP callbacks on /v1 without Bearer JWT (HMAC-only when configured).
func mountCommercePublicWebhookPost(r chi.Router, app *api.HTTPApplication, cfg *config.Config, abuse *AbuseProtection, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Commerce == nil || cfg == nil || app.TelemetryStore == nil {
		return
	}
	if abuse == nil {
		abuse = &AbuseProtection{}
	}
	if writeRL == nil {
		writeRL = func(next http.Handler) http.Handler { return next }
	}
	r.With(writeRL, abuse.WebhookPOST()).Post("/commerce/orders/{orderId}/payments/{paymentId}/webhooks", commercePublicPaymentWebhookHandler(app, cfg))
}

func commerceWebhookProviderMetadataJSON(unsignedDevelopment bool) []byte {
	if unsignedDevelopment {
		return []byte(`{"delivery":{"mode":"unsigned_development"}}`)
	}
	return []byte(`{}`)
}

func attachPaymentWebhookOutbox(in *appcommerce.ApplyPaymentProviderWebhookInput, cfg *config.Config) {
	if in == nil || cfg == nil {
		return
	}
	eventType := ""
	switch strings.TrimSpace(strings.ToLower(in.NormalizedPaymentState)) {
	case "captured":
		eventType = domainreliability.OutboxEventPaymentConfirmed
	case "failed":
		eventType = domainreliability.OutboxEventPaymentFailed
	default:
		return
	}
	topic := strings.TrimSpace(cfg.Commerce.PaymentOutboxTopic)
	aggregateType := strings.TrimSpace(cfg.Commerce.PaymentOutboxAggregateType)
	if topic == "" || aggregateType == "" {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"source":           "payment_webhook",
		"order_id":         in.OrderID.String(),
		"payment_id":       in.PaymentID.String(),
		"provider":         strings.TrimSpace(in.Provider),
		"webhook_event_id": strings.TrimSpace(in.WebhookEventID),
		"payment_state":    strings.TrimSpace(in.NormalizedPaymentState),
	})
	if err != nil {
		payload = []byte(`{}`)
	}
	in.OutboxTopic = topic
	in.OutboxEventType = eventType
	in.OutboxPayload = payload
	in.OutboxAggregateType = aggregateType
	in.OutboxAggregateID = in.PaymentID
	in.OutboxIdempotencyKey = strings.Join([]string{
		"payment_webhook",
		strings.TrimSpace(in.Provider),
		strings.TrimSpace(in.WebhookEventID),
		eventType,
	}, ":")
}

func commerceOrderMachinePtr(ord db.Order) *uuid.UUID {
	if ord.MachineID == uuid.Nil {
		return nil
	}
	mid := ord.MachineID
	return &mid
}

func auditPaymentWebhookRejected(ctx context.Context, rec compliance.EnterpriseRecorder, orgID uuid.UUID, paymentID uuid.UUID, machineID *uuid.UUID, meta map[string]any) error {
	if rec == nil || orgID == uuid.Nil {
		return nil
	}
	pid := paymentID.String()
	md, err := json.Marshal(meta)
	if err != nil {
		md = []byte("{}")
	}
	md = compliance.SanitizeJSONBytes(md)
	return rec.RecordCritical(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: orgID,
		ActorType:      compliance.ActorPaymentProvider,
		Action:         compliance.ActionPaymentWebhookRejected,
		ResourceType:   "commerce.payment",
		ResourceID:     &pid,
		MachineID:      machineID,
		Metadata:       md,
		Outcome:        compliance.OutcomeFailure,
	})
}

func auditPaymentWebhookIdempotencyConflict(ctx context.Context, rec compliance.EnterpriseRecorder, orgID uuid.UUID, paymentID uuid.UUID, machineID *uuid.UUID, meta map[string]any) error {
	if rec == nil || orgID == uuid.Nil {
		return nil
	}
	pid := paymentID.String()
	md, err := json.Marshal(meta)
	if err != nil {
		md = []byte("{}")
	}
	md = compliance.SanitizeJSONBytes(md)
	return rec.RecordCritical(ctx, compliance.EnterpriseAuditRecord{
		OrganizationID: orgID,
		ActorType:      compliance.ActorPaymentProvider,
		Action:         compliance.ActionPaymentWebhookIdempotencyConflict,
		ResourceType:   "commerce.payment",
		ResourceID:     &pid,
		MachineID:      machineID,
		Metadata:       md,
		Outcome:        compliance.OutcomeFailure,
	})
}

func upsertWebhookReconciliationCase(ctx context.Context, q *db.Queries, audit compliance.EnterpriseRecorder, orgID uuid.UUID, ord db.Order, paymentID uuid.UUID, provider string, providerEventID int64, caseType, severity, reason string, meta map[string]any) error {
	if q == nil || orgID == uuid.Nil {
		return errors.New("webhook reconciliation case: persistence unavailable")
	}
	md, err := json.Marshal(meta)
	if err != nil {
		md = []byte("{}")
	}
	md = compliance.SanitizeJSONBytes(md)
	var oid pgtype.UUID
	if ord.ID != uuid.Nil {
		oid = pgtype.UUID{Bytes: ord.ID, Valid: true}
	}
	var mid pgtype.UUID
	if ord.MachineID != uuid.Nil {
		mid = pgtype.UUID{Bytes: ord.MachineID, Valid: true}
	}
	var pid pgtype.UUID
	if paymentID != uuid.Nil {
		pid = pgtype.UUID{Bytes: paymentID, Valid: true}
	}
	var peid pgtype.Int8
	if providerEventID > 0 {
		peid = pgtype.Int8{Int64: providerEventID, Valid: true}
	}
	sev := strings.TrimSpace(severity)
	if sev == "" {
		sev = "warning"
	}
	if _, err := q.UpsertCommerceReconciliationCase(ctx, db.UpsertCommerceReconciliationCaseParams{
		OrganizationID:  orgID,
		CaseType:        caseType,
		Severity:        sev,
		Reason:          reason,
		Metadata:        md,
		OrderID:         oid,
		PaymentID:       pid,
		MachineID:       mid,
		Provider:        pgtype.Text{String: strings.TrimSpace(provider), Valid: strings.TrimSpace(provider) != ""},
		ProviderEventID: peid,
		CorrelationKey:  pgtype.Text{},
	}); err != nil {
		return err
	}
	if audit != nil {
		rid := paymentID.String()
		metaAudit, mErr := json.Marshal(map[string]any{"case_type": caseType, "detail": meta})
		if mErr != nil {
			metaAudit = []byte("{}")
		}
		metaAudit = compliance.SanitizeJSONBytes(metaAudit)
		mach := commerceOrderMachinePtr(ord)
		if err := audit.RecordCritical(ctx, compliance.EnterpriseAuditRecord{
			OrganizationID: orgID,
			ActorType:      compliance.ActorPaymentProvider,
			Action:         compliance.ActionCommerceReconciliationCaseCreated,
			ResourceType:   "commerce.payment",
			ResourceID:     &rid,
			MachineID:      mach,
			Metadata:       metaAudit,
			Outcome:        compliance.OutcomeSuccess,
		}); err != nil {
			return err
		}
	}
	return nil
}

func commercePublicPaymentWebhookHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	if app == nil || cfg == nil {
		return func(w http.ResponseWriter, r *http.Request) {
			recordCommercePaymentWebhookResult("handler_misconfigured")
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "misconfigured webhook handler")
		}
	}
	svc := app.Commerce
	var q *db.Queries
	if app.TelemetryStore != nil {
		q = db.New(app.TelemetryStore.Pool())
	}
	return func(w http.ResponseWriter, r *http.Request) {
		outcome := "unknown"
		defer func() { recordCommercePaymentWebhookResult(outcome) }()

		restricted := cfg.AppEnv.RestrictsUnsignedCommercePaymentWebhooks()
		devUnsignedOk := cfg.AppEnv.AllowsUnsignedCommerceWebhooksInDevelopment() && cfg.Commerce.PaymentWebhookAllowUnsigned
		globalSecret := strings.TrimSpace(cfg.Commerce.PaymentWebhookHMACSecret)
		hasSecretMaterial := globalSecret != "" || len(cfg.Commerce.PaymentWebhookProviderSecrets) > 0

		var validationStatus string
		var verifyHMAC bool
		switch {
		case hasSecretMaterial:
			verifyHMAC = true
			validationStatus = "hmac_verified"
		case restricted && cfg.Commerce.PaymentWebhookUnsafeAllowUnsignedProduction:
			verifyHMAC = false
			validationStatus = "unsigned_development"
		case devUnsignedOk:
			verifyHMAC = false
			validationStatus = "unsigned_development"
		case restricted:
			outcome = "hmac_required_policy"
			writeAPIError(w, r.Context(), http.StatusForbidden, "webhook_hmac_required",
				"payment webhooks require COMMERCE_PAYMENT_WEBHOOK_SECRET and/or COMMERCE_PAYMENT_WEBHOOK_SECRETS_JSON in staging/production; unsigned delivery is rejected unless COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION=true (documented unsafe)")
			return
		default:
			outcome = "hmac_not_configured"
			writeCapabilityNotConfigured(w, r.Context(), "v1.commerce.payment_webhook.hmac",
				"set COMMERCE_PAYMENT_WEBHOOK_SECRET or COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=true (development/test only)")
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			outcome = "invalid_body"
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_body", "could not read body")
			return
		}
		if verifyHMAC {
			providerPeek := platformpayments.PeekWebhookProvider(body)
			secret := platformpayments.ResolveWebhookHMACSecret(globalSecret, cfg.Commerce.PaymentWebhookProviderSecrets, providerPeek)
			if strings.TrimSpace(secret) == "" {
				outcome = "hmac_provider_secret_missing"
				writeAPIError(w, r.Context(), http.StatusForbidden, "webhook_hmac_required",
					"no HMAC secret configured for this provider label; set COMMERCE_PAYMENT_WEBHOOK_SECRET or a matching COMMERCE_PAYMENT_WEBHOOK_SECRETS_JSON entry")
				return
			}
			if err := platformpayments.VerifyCommerceWebhookHMAC(secret, r.Header.Get("X-AVF-Webhook-Timestamp"), r.Header.Get("X-AVF-Webhook-Signature"), body, cfg.Commerce.PaymentWebhookTimestampSkew); err != nil {
				els := strings.ToLower(err.Error())
				if strings.Contains(els, "outside allowed skew") || strings.Contains(els, "invalid x-avf-webhook-timestamp") {
					outcome = "webhook_timestamp_skew"
					writeAPIError(w, r.Context(), http.StatusBadRequest, "webhook_timestamp_skew", err.Error())
					return
				}
				outcome = "webhook_hmac_invalid"
				writeAPIError(w, r.Context(), http.StatusUnauthorized, "webhook_auth_failed", err.Error())
				return
			}
		}

		if q == nil {
			outcome = "persistence_not_configured"
			writeCapabilityNotConfigured(w, r.Context(), "v1.commerce.payment_webhook.persistence",
				"DATABASE_URL-backed store is required for payment webhook handling")
			return
		}
		if svc == nil {
			outcome = "commerce_not_configured"
			writeCapabilityNotConfigured(w, r.Context(), "v1.commerce.payment_webhook.service",
				"commerce service is required for payment webhook handling")
			return
		}

		orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
		if err != nil {
			outcome = "invalid_order_id"
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
			return
		}
		paymentID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "paymentId")))
		if err != nil {
			outcome = "invalid_payment_id"
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_payment_id", "invalid paymentId")
			return
		}

		ctx := r.Context()
		pay, err := q.GetPaymentByID(ctx, paymentID)
		if err != nil {
			outcome = "store_error"
			writeCommerceStoreError(w, r, err)
			return
		}
		if pay.OrderID != orderID {
			outcome = "payment_order_mismatch"
			writeAPIError(w, r.Context(), http.StatusBadRequest, "payment_order_mismatch", "payment does not belong to order")
			return
		}
		ord, err := q.GetOrderByID(ctx, orderID)
		if err != nil {
			outcome = "store_error"
			writeCommerceStoreError(w, r, err)
			return
		}
		auditOrg := ord.OrganizationID

		var wh commerceWebhookRequest
		if err := json.Unmarshal(body, &wh); err != nil {
			outcome = "invalid_json"
			if err := auditPaymentWebhookRejected(ctx, app.EnterpriseAudit, auditOrg, paymentID, commerceOrderMachinePtr(ord), map[string]any{
				"reason": "invalid_json",
			}); err != nil {
				outcome = "audit_failed"
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "audit_failed", "could not record audit event")
				return
			}
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		if strings.TrimSpace(wh.Provider) == "" {
			outcome = "missing_provider"
			if err := auditPaymentWebhookRejected(ctx, app.EnterpriseAudit, auditOrg, paymentID, commerceOrderMachinePtr(ord), map[string]any{
				"reason": "missing_provider",
			}); err != nil {
				outcome = "audit_failed"
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "audit_failed", "could not record audit event")
				return
			}
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "provider is required")
			return
		}
		unsignedDev := validationStatus == "unsigned_development"
		in := appcommerce.ApplyPaymentProviderWebhookInput{
			OrganizationID:          ord.OrganizationID,
			OrderID:                 orderID,
			PaymentID:               paymentID,
			Provider:                wh.Provider,
			ProviderReference:       wh.ProviderReference,
			WebhookEventID:          wh.WebhookEventID,
			EventType:               wh.EventType,
			NormalizedPaymentState:  wh.NormalizedPaymentState,
			Payload:                 wh.PayloadJSON,
			ProviderAmountMinor:     wh.ProviderAmountMinor,
			Currency:                wh.Currency,
			WebhookValidationStatus: validationStatus,
			ProviderMetadata:        commerceWebhookProviderMetadataJSON(unsignedDev),
		}
		attachPaymentWebhookOutbox(&in, cfg)
		res, err := svc.ApplyPaymentProviderWebhook(ctx, in)
		if err != nil {
			if errors.Is(err, appcommerce.ErrWebhookIdempotencyConflict) {
				outcome = "idempotency_conflict"
				if err := auditPaymentWebhookIdempotencyConflict(ctx, app.EnterpriseAudit, auditOrg, paymentID, commerceOrderMachinePtr(ord), map[string]any{
					"reason": "conflicting_replay_fields",
				}); err != nil {
					outcome = "audit_failed"
					writeAPIError(w, r.Context(), http.StatusInternalServerError, "audit_failed", "could not record audit event")
					return
				}
				writeCommerceServiceError(w, ctx, err)
				return
			}
			outcome = "apply_error"
			var caseUpsertErr error
			switch {
			case errors.Is(err, appcommerce.ErrWebhookProviderMismatch):
				caseUpsertErr = upsertWebhookReconciliationCase(ctx, q, app.EnterpriseAudit, auditOrg, ord, paymentID, wh.Provider, 0, "webhook_provider_mismatch", "critical", "webhook provider does not match persisted payment provider", map[string]any{
					"provider": strings.TrimSpace(wh.Provider),
				})
			case errors.Is(err, appcommerce.ErrWebhookAmountCurrencyMismatch):
				caseUpsertErr = upsertWebhookReconciliationCase(ctx, q, app.EnterpriseAudit, auditOrg, ord, paymentID, wh.Provider, 0, "webhook_amount_currency_mismatch", "critical", "webhook amount or currency does not match payment row", map[string]any{
					"provider": strings.TrimSpace(wh.Provider),
				})
				if caseUpsertErr == nil {
					productionmetrics.RecordPaymentWebhookAmountCurrencyMismatch()
				}
			case errors.Is(err, appcommerce.ErrWebhookAfterTerminalOrder):
				caseUpsertErr = upsertWebhookReconciliationCase(ctx, q, app.EnterpriseAudit, auditOrg, ord, paymentID, wh.Provider, 0, "webhook_after_terminal_order", "critical", "webhook arrived after order/payment reached terminal state", map[string]any{
					"provider": strings.TrimSpace(wh.Provider),
				})
			}
			if caseUpsertErr != nil {
				outcome = "apply_error_case_persist_failed"
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", caseUpsertErr.Error())
				return
			}
			auditPayErr := auditPaymentWebhookRejected(ctx, app.EnterpriseAudit, auditOrg, paymentID, commerceOrderMachinePtr(ord), map[string]any{
				"reason": "apply_failed",
				"error":  err.Error(),
			})
			if auditPayErr != nil {
				outcome = "audit_failed"
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "audit_failed", "could not record audit event")
				return
			}
			writeCommerceServiceError(w, ctx, err)
			return
		}
		resp := commerceWebhookResponse{
			Replay:          res.Replay,
			OrderID:         res.Order.ID.String(),
			OrderStatus:     res.Order.Status,
			PaymentID:       res.Payment.ID.String(),
			PaymentState:    res.Payment.State,
			ProviderEventID: res.ProviderRowID,
		}
		if res.Replay {
			if uerr := upsertWebhookReconciliationCase(ctx, q, app.EnterpriseAudit, auditOrg, ord, paymentID, wh.Provider, res.ProviderRowID, "duplicate_provider_event", "info", "provider webhook replay was received and handled idempotently", map[string]any{
				"provider":         strings.TrimSpace(wh.Provider),
				"webhook_event_id": strings.TrimSpace(wh.WebhookEventID),
				"provider_ref":     strings.TrimSpace(wh.ProviderReference),
			}); uerr != nil {
				outcome = "replayed_case_persist_failed"
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", uerr.Error())
				return
			}
		}
		if res.Attempt.ID != uuid.Nil {
			resp.AttemptID = res.Attempt.ID.String()
		}
		if res.Replay {
			outcome = "replayed"
		} else {
			outcome = "accepted"
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// IntegrationTestCommercePublicPaymentWebhook exposes the production commerce webhook handler for
// DB-backed integration tests. It must not be used from production call sites.
func IntegrationTestCommercePublicPaymentWebhook(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return commercePublicPaymentWebhookHandler(app, cfg)
}

func writeCommerceStoreError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "resource not found")
		return
	}
	writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
}
