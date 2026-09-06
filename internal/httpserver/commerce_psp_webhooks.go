package httpserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// mountCommercePSPWebhooks registers native PSP IPN/callback routes on /v1 (public, no Bearer).
func mountCommercePSPWebhooks(r chi.Router, app *api.HTTPApplication, cfg *config.Config, abuse *AbuseProtection, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Commerce == nil || cfg == nil || app.TelemetryStore == nil || app.PaymentProviders == nil {
		return
	}
	if abuse == nil {
		abuse = &AbuseProtection{}
	}
	if writeRL == nil {
		writeRL = func(next http.Handler) http.Handler { return next }
	}
	r.Route("/commerce/webhooks", func(r chi.Router) {
		r.With(writeRL, abuse.WebhookPOST()).Post("/momo", MoMoNativeIPNHandler(app, cfg))
		r.With(writeRL, abuse.WebhookPOST()).Post("/zalopay", ZaloPayNativeCallbackHandler(app, cfg))
		r.With(writeRL, abuse.WebhookPOST()).Post("/shopeepay", ShopeePayNativeCallbackHandler(app, cfg))
		r.With(abuse.WebhookPOST()).Get("/vnpay/return", VNPayNativeReturnHandler(app, cfg))
	})
}

// MoMoNativeIPNHandler handles POST /v1/commerce/webhooks/momo (exported for legacy alias mounts).
func MoMoNativeIPNHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid body", "resultCode": 1001})
			return
		}
		prov, ok := app.PaymentProviders.Get("momo").(*platformpayments.MoMoProvider)
		if !ok || prov == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"message": "momo not configured", "resultCode": 1001})
			return
		}
		log := observability.LoggerFromContext(r.Context(), zap.NewNop())
		log.Info("PSP_WEBHOOK_RECEIVED",
			zap.String("provider", "momo"),
			zap.Int("body_bytes", len(body)),
		)
		_, status, _, event, err := prov.VerifyAndParseIPN(body)
		if err != nil {
			log.Warn("PSP_WEBHOOK_SIGNATURE_INVALID",
				zap.String("provider", "momo"),
				zap.Error(err),
			)
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error(), "resultCode": 1001})
			return
		}
		logPSPWebhookParsed(log, "momo", event, status)
		handleNativePSPWebhookResponse(w, r.Context(), app, cfg, log, "momo", event, status,
			func() { writeJSON(w, http.StatusOK, map[string]any{"message": "Success", "resultCode": 0}) },
			func(msg string) { writeJSON(w, http.StatusOK, map[string]any{"message": msg, "resultCode": 1001}) },
		)
	}
}

// ZaloPayNativeCallbackHandler handles POST /v1/commerce/webhooks/zalopay.
func ZaloPayNativeCallbackHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"return_code": -1, "return_message": "invalid body"})
			return
		}
		prov, ok := app.PaymentProviders.Get("zalopay").(*platformpayments.ZaloPayProvider)
		if !ok || prov == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"return_code": -1, "return_message": "zalopay not configured"})
			return
		}
		log := observability.LoggerFromContext(r.Context(), zap.NewNop())
		log.Info("PSP_WEBHOOK_RECEIVED",
			zap.String("provider", "zalopay"),
			zap.Int("body_bytes", len(body)),
		)
		_, status, _, event, err := prov.VerifyAndParseCallback(body)
		if err != nil {
			log.Warn("PSP_WEBHOOK_SIGNATURE_INVALID",
				zap.String("provider", "zalopay"),
				zap.Error(err),
			)
			writeJSON(w, http.StatusBadRequest, map[string]any{"return_code": -1, "return_message": err.Error()})
			return
		}
		logPSPWebhookParsed(log, "zalopay", event, status)
		handleNativePSPWebhookResponse(w, r.Context(), app, cfg, log, "zalopay", event, status,
			func() { writeJSON(w, http.StatusOK, map[string]any{"return_code": 1, "return_message": "success"}) },
			func(msg string) { writeJSON(w, http.StatusOK, map[string]any{"return_code": -1, "return_message": msg}) },
		)
	}
}

// ShopeePayNativeCallbackHandler handles POST /v1/commerce/webhooks/shopeepay.
func ShopeePayNativeCallbackHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		softACK := func() {
			writeJSON(w, http.StatusOK, map[string]any{"errcode": 0, "debug_msg": "success"})
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			softACK()
			return
		}
		prov, ok := app.PaymentProviders.Get("shopeepay").(*platformpayments.ShopeePayProvider)
		if !ok || prov == nil {
			softACK()
			return
		}
		log := observability.LoggerFromContext(r.Context(), zap.NewNop())
		log.Info("PSP_WEBHOOK_RECEIVED",
			zap.String("provider", "shopeepay"),
			zap.Int("body_bytes", len(body)),
		)
		clientID := strings.TrimSpace(r.Header.Get("X-Airpay-ClientId"))
		signature := strings.TrimSpace(r.Header.Get("X-Airpay-Req-H"))
		_, status, _, event, err := prov.VerifyAndParseCallback(body, clientID, signature, clientIP(r))
		if err != nil {
			log.Warn("PSP_WEBHOOK_SIGNATURE_INVALID",
				zap.String("provider", "shopeepay"),
				zap.Error(err),
			)
			softACK()
			return
		}
		logPSPWebhookParsed(log, "shopeepay", event, status)
		handleNativePSPWebhookResponse(w, r.Context(), app, cfg, log, "shopeepay", event, status, softACK, func(_ string) { softACK() })
	}
}

// VNPayNativeReturnHandler handles GET /v1/commerce/webhooks/vnpay/return.
func VNPayNativeReturnHandler(app *api.HTTPApplication, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prov, ok := app.PaymentProviders.Get("vnpay").(*platformpayments.VNPayProvider)
		if !ok || prov == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"RspCode": "99", "Message": "vnpay not configured"})
			return
		}
		log := observability.LoggerFromContext(r.Context(), zap.NewNop())
		log.Info("PSP_WEBHOOK_RECEIVED",
			zap.String("provider", "vnpay"),
			zap.String("query", r.URL.RawQuery),
		)
		event, err := prov.ParseReturnQuery(r.URL.Query())
		if err != nil {
			log.Warn("PSP_WEBHOOK_SIGNATURE_INVALID",
				zap.String("provider", "vnpay"),
				zap.Error(err),
			)
			writeJSON(w, http.StatusBadRequest, map[string]any{"RspCode": "99", "Message": err.Error()})
			return
		}
		logPSPWebhookParsed(log, "vnpay", event, event.NormalizedPaymentState)
		handleNativePSPWebhookResponse(w, r.Context(), app, cfg, log, "vnpay", event, event.NormalizedPaymentState,
			func() { writeJSON(w, http.StatusOK, map[string]any{"RspCode": "00", "Message": "Confirm Success"}) },
			func(msg string) { writeJSON(w, http.StatusOK, map[string]any{"RspCode": "99", "Message": msg}) },
		)
	}
}

func logPSPWebhookParsed(log *zap.Logger, provider string, event platformpayments.CommerceWebhookEventJSON, status string) {
	if log == nil {
		return
	}
	log.Info("PSP_WEBHOOK_RECEIVED",
		zap.String("provider", provider),
		zap.String("provider_reference", strings.TrimSpace(event.ProviderReference)),
		zap.String("normalized_state", strings.TrimSpace(status)),
		zap.Bool("signature_valid", true),
	)
}

func handleNativePSPWebhookResponse(
	_ http.ResponseWriter,
	ctx context.Context,
	app *api.HTTPApplication,
	cfg *config.Config,
	log *zap.Logger,
	provider string,
	event platformpayments.CommerceWebhookEventJSON,
	status string,
	onSuccess func(),
	onFailure func(string),
) {
	applied, applyErr := applyNativePSPWebhook(ctx, app, cfg, event, status)
	if applyErr != nil {
		if errors.Is(applyErr, pgx.ErrNoRows) {
			onFailure("order not found")
			return
		}
		reason := appcommerce.ClassifyApplyRejectReason(applyErr)
		log.Warn("PSP_WEBHOOK_APPLY_REJECTED",
			zap.String("provider", provider),
			zap.String("provider_reference", strings.TrimSpace(event.ProviderReference)),
			zap.String("reject_reason", reason),
			zap.Error(applyErr),
		)
		onFailure("apply failed")
		return
	}
	if applied {
		log.Info("PSP_WEBHOOK_APPLIED",
			zap.String("provider", provider),
			zap.String("provider_reference", strings.TrimSpace(event.ProviderReference)),
			zap.String("normalized_state", strings.TrimSpace(status)),
		)
	}
	onSuccess()
}

func applyNativePSPWebhook(ctx context.Context, app *api.HTTPApplication, cfg *config.Config, event platformpayments.CommerceWebhookEventJSON, normalizedState string) (bool, error) {
	if app == nil || app.Commerce == nil || app.TelemetryStore == nil {
		return false, errors.New("commerce webhook not configured")
	}
	ref := strings.TrimSpace(event.ProviderReference)
	if ref == "" {
		return false, errors.New("empty provider_reference")
	}
	row, err := app.TelemetryStore.GetPaymentByProviderReference(ctx, ref)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			productionmetrics.RecordPaymentWebhookLookupMiss(strings.TrimSpace(event.Provider))
			if app.TelemetryStore != nil {
				provider := strings.TrimSpace(event.Provider)
				if provider == "" {
					provider = "unknown"
				}
				_, _ = app.TelemetryStore.UpsertReconciliationCase(ctx, domaincommerce.ReconciliationCaseInput{
					CaseType:       "payment_webhook_lookup_miss",
					Severity:       "critical",
					Reason:         "PSP webhook provider_reference did not match any payment row",
					Provider:       &provider,
					CorrelationKey: "webhook_lookup_miss:" + provider + ":" + ref,
					Metadata:       []byte(`{"provider_reference":"` + ref + `"}`),
				})
			}
			observability.LoggerFromContext(ctx, zap.NewNop()).Warn("PSP_WEBHOOK_LOOKUP_MISS",
				zap.String("provider", strings.TrimSpace(event.Provider)),
				zap.String("provider_reference", ref),
			)
		}
		return false, err
	}
	state := strings.ToLower(strings.TrimSpace(normalizedState))
	if state == "" {
		state = strings.ToLower(strings.TrimSpace(event.NormalizedPaymentState))
	}
	switch state {
	case "captured", "failed":
	default:
		// Pending / unknown: acknowledge without mutating payment state.
		return false, nil
	}
	resolvedProvider := reconcileNativeWebhookProvider(event.Provider, row.Provider)
	if resolvedProvider != strings.TrimSpace(event.Provider) && strings.TrimSpace(event.Provider) != "" {
		observability.LoggerFromContext(ctx, zap.NewNop()).Info("PSP_WEBHOOK_PROVIDER_RECONCILED",
			zap.String("event_provider", strings.TrimSpace(event.Provider)),
			zap.String("stored_provider", strings.TrimSpace(row.Provider)),
			zap.String("resolved_provider", resolvedProvider),
		)
	}
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrderID:                 row.OrderID,
		PaymentID:               row.PaymentID,
		Provider:                resolvedProvider,
		ProviderReference:       ref,
		WebhookEventID:          strings.TrimSpace(event.WebhookEventID),
		EventType:               strings.TrimSpace(event.EventType),
		NormalizedPaymentState:  state,
		Payload:                 event.PayloadJSON,
		ProviderAmountMinor:     event.ProviderAmountMinor,
		Currency:                event.Currency,
		WebhookValidationStatus: "provider_native_verified",
		ProviderMetadata:        []byte(`{"delivery":{"mode":"provider_native"}}`),
	}
	if in.WebhookEventID == "" {
		in.WebhookEventID = ref
	}
	if in.EventType == "" {
		in.EventType = "provider.native"
	}
	attachPaymentWebhookOutbox(&in, cfg)
	_, err = app.Commerce.ApplyPaymentProviderWebhook(ctx, in)
	if err != nil {
		return false, err
	}
	return true, nil
}

// reconcileNativeWebhookProvider maps a verified PSP callback to the payment row's provider key.
// VietQR sessions are stored as provider=vietqr but ZaloPay callbacks tag provider=zalopay.
func reconcileNativeWebhookProvider(eventProvider, storedProvider string) string {
	eventProv := strings.TrimSpace(eventProvider)
	storedProv := strings.TrimSpace(storedProvider)
	switch {
	case storedProv != "" && (eventProv == "" || strings.EqualFold(eventProv, storedProv)):
		return storedProv
	case storedProv != "" && eventProv != "" && platformpayments.SameWebhookAdapterFamily(eventProv, storedProv):
		return storedProv
	case eventProv != "":
		return eventProv
	default:
		return storedProv
	}
}

func callbackURLHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}

// LookupPaymentByProviderReference is a thin alias for tests/handlers that already hold the store.
func LookupPaymentByProviderReference(ctx context.Context, store *postgres.Store, ref string) (postgres.PaymentByProviderRef, error) {
	if store == nil {
		return postgres.PaymentByProviderRef{}, errors.New("store not configured")
	}
	return store.GetPaymentByProviderReference(ctx, ref)
}
