package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ appcommerce.PaymentWebhookPersistence = (*Store)(nil)

func machineUUIDPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	v := id
	return &v
}

func paymentTransitionAllowed(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case "created":
		return to == "authorized" || to == "captured" || to == "failed" || to == "expired" || to == "canceled"
	case "authorized":
		return to == "captured" || to == "failed" || to == "expired" || to == "canceled"
	case "captured":
		return to == "failed" || to == "refunded" || to == "partially_refunded"
	case "partially_refunded":
		return to == "refunded"
	case "failed", "expired", "canceled", "refunded":
		return false
	default:
		return false
	}
}

func assertWebhookReplayFieldsMatch(existing db.PaymentProviderEvent, in appcommerce.ApplyPaymentProviderWebhookInput) error {
	inRef := strings.TrimSpace(in.ProviderReference)
	if existing.ProviderRef.Valid {
		got := strings.TrimSpace(existing.ProviderRef.String)
		if got != "" && inRef != "" && got != inRef {
			return appcommerce.ErrWebhookIdempotencyConflict
		}
	}
	inEv := strings.TrimSpace(in.WebhookEventID)
	if existing.WebhookEventID.Valid {
		got := strings.TrimSpace(existing.WebhookEventID.String)
		if got != "" && inEv != "" && got != inEv {
			return appcommerce.ErrWebhookIdempotencyConflict
		}
	}
	return nil
}

func (s *Store) webhookReplayResultFromEvent(ctx context.Context, q *db.Queries, existing db.PaymentProviderEvent, in appcommerce.ApplyPaymentProviderWebhookInput) (appcommerce.ApplyPaymentProviderWebhookResult, error) {
	if !existing.PaymentID.Valid || uuid.UUID(existing.PaymentID.Bytes) != in.PaymentID {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, errors.Join(appcommerce.ErrOrgMismatch, errors.New("provider event already bound to a different payment"))
	}
	if err := assertWebhookReplayFieldsMatch(existing, in); err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	pay, err := q.GetPaymentByID(ctx, in.PaymentID)
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	if p := strings.TrimSpace(pay.Provider); p != "" && !strings.EqualFold(p, strings.TrimSpace(in.Provider)) {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, appcommerce.ErrWebhookProviderMismatch
	}
	ord, err := q.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	return appcommerce.ApplyPaymentProviderWebhookResult{
		Replay:        true,
		Order:         mapOrder(ord),
		Payment:       mapPayment(pay),
		ProviderRowID: existing.ID,
	}, nil
}

func (s *Store) auditPaymentWebhookTx(ctx context.Context, tx pgx.Tx, in appcommerce.ApplyPaymentProviderWebhookInput, replay bool, machineID *uuid.UUID) error {
	if s.enterpriseAudit == nil {
		return nil
	}
	action := compliance.ActionPaymentWebhookAccepted
	if replay {
		action = compliance.ActionPaymentWebhookReplayed
	}
	vstat := strings.TrimSpace(in.WebhookValidationStatus)
	if vstat == "" {
		vstat = "hmac_verified"
	}
	md, err := json.Marshal(map[string]any{
		"order_id":         in.OrderID.String(),
		"payment_id":       in.PaymentID.String(),
		"provider":         in.Provider,
		"webhook_event_id": strings.TrimSpace(in.WebhookEventID),
		"replay":           replay,
		"validation":       vstat,
	})
	if err != nil {
		md = []byte("{}")
	}
	pid := in.PaymentID.String()
	return s.enterpriseAudit.RecordCriticalTx(ctx, tx, compliance.EnterpriseAuditRecord{
		OrganizationID: in.OrganizationID,
		ActorType:      compliance.ActorPaymentProvider,
		Action:         action,
		ResourceType:   "commerce.payment",
		ResourceID:     &pid,
		MachineID:      machineID,
		Metadata:       md,
	})
}

func (s *Store) ApplyPaymentProviderWebhook(ctx context.Context, in appcommerce.ApplyPaymentProviderWebhookInput) (appcommerce.ApplyPaymentProviderWebhookResult, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	existingEv, err := q.GetPaymentProviderEventByProviderRef(ctx, db.GetPaymentProviderEventByProviderRefParams{
		Provider:    in.Provider,
		ProviderRef: pgtype.Text{String: in.ProviderReference, Valid: true},
	})
	if err == nil {
		res, rerr := s.webhookReplayResultFromEvent(ctx, q, existingEv, in)
		if rerr != nil {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, rerr
		}
		if err := s.auditPaymentWebhookTx(ctx, tx, in, true, machineUUIDPtr(res.Order.MachineID)); err != nil {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, err
		}
		return res, nil
	}
	if !isNoRows(err) {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}

	if strings.TrimSpace(in.WebhookEventID) != "" {
		existingByEv, err2 := q.GetPaymentProviderEventByWebhookEventID(ctx, db.GetPaymentProviderEventByWebhookEventIDParams{
			Provider:       in.Provider,
			WebhookEventID: pgtype.Text{String: strings.TrimSpace(in.WebhookEventID), Valid: true},
		})
		if err2 == nil {
			res, rerr := s.webhookReplayResultFromEvent(ctx, q, existingByEv, in)
			if rerr != nil {
				return appcommerce.ApplyPaymentProviderWebhookResult{}, rerr
			}
			if err := s.auditPaymentWebhookTx(ctx, tx, in, true, machineUUIDPtr(res.Order.MachineID)); err != nil {
				return appcommerce.ApplyPaymentProviderWebhookResult{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return appcommerce.ApplyPaymentProviderWebhookResult{}, err
			}
			return res, nil
		}
		if !isNoRows(err2) {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, err2
		}
	}

	pay, err := q.GetPaymentByID(ctx, in.PaymentID)
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	if p := strings.TrimSpace(pay.Provider); p != "" && !strings.EqualFold(p, strings.TrimSpace(in.Provider)) {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, appcommerce.ErrWebhookProviderMismatch
	}
	if pay.OrderID != in.OrderID {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, appcommerce.ErrOrgMismatch
	}
	ord, err := q.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	if ord.OrganizationID != in.OrganizationID {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, appcommerce.ErrOrgMismatch
	}

	if err := webhookAmountCurrencyMatches(pay, ord, in); err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}

	target := strings.TrimSpace(strings.ToLower(in.NormalizedPaymentState))
	if !paymentTransitionAllowed(pay.State, target) {
		if webhookLateDeliveryAgainstTerminalState(ord, pay) {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, appcommerce.ErrWebhookAfterTerminalOrder
		}
		return appcommerce.ApplyPaymentProviderWebhookResult{}, appcommerce.ErrIllegalTransition
	}

	if pay.State != target {
		pay, err = q.UpdatePaymentState(ctx, db.UpdatePaymentStateParams{ID: pay.ID, State: target})
		if err != nil {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, err
		}
	}

	var amt pgtype.Int8
	if in.ProviderAmountMinor != nil {
		amt = pgtype.Int8{Int64: *in.ProviderAmountMinor, Valid: true}
	}
	var cur pgtype.Text
	if in.Currency != nil {
		cur = pgtype.Text{String: *in.Currency, Valid: true}
	}
	var webhookEv pgtype.Text
	if w := strings.TrimSpace(in.WebhookEventID); w != "" {
		webhookEv = pgtype.Text{String: w, Valid: true}
	}
	vstat := strings.TrimSpace(in.WebhookValidationStatus)
	if vstat == "" {
		vstat = "hmac_verified"
	}
	meta := in.ProviderMetadata
	if len(meta) == 0 {
		meta = []byte(`{}`)
	}
	sigOK := vstat == "hmac_verified" || vstat == "unsigned_development"
	now := time.Now().UTC()
	applied := pgtype.Timestamptz{Time: now, Valid: true}

	ev, err := q.InsertPaymentProviderEvent(ctx, db.InsertPaymentProviderEventParams{
		PaymentID:           pgtype.UUID{Bytes: in.PaymentID, Valid: true},
		OrganizationID:      pgtype.UUID{Bytes: in.OrganizationID, Valid: true},
		Provider:            in.Provider,
		ProviderRef:         pgtype.Text{String: in.ProviderReference, Valid: true},
		WebhookEventID:      webhookEv,
		ProviderAmountMinor: amt,
		Currency:            cur,
		EventType:           in.EventType,
		Payload:             compliance.SanitizeJSONBytes(in.Payload),
		ValidationStatus:    vstat,
		ProviderMetadata:    meta,
		SignatureValid:      sigOK,
		AppliedAt:           applied,
		IngressStatus:       "applied",
		IngressError:        pgtype.Text{},
	})
	if err != nil {
		if isUniqueViolation(err) {
			_ = tx.Rollback(ctx)
			return s.webhookReplayAfterUniqueViolation(ctx, in)
		}
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}

	attemptRow, err := q.InsertPaymentAttempt(ctx, db.InsertPaymentAttemptParams{
		PaymentID:         in.PaymentID,
		ProviderReference: pgtype.Text{String: in.ProviderReference, Valid: true},
		State:             target,
		Payload:           in.Payload,
	})
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}

	if target == "captured" && (ord.Status == "created" || ord.Status == "quoted") {
		ord, err = q.UpdateOrderStatusByOrg(ctx, db.UpdateOrderStatusByOrgParams{
			ID:             ord.ID,
			OrganizationID: ord.OrganizationID,
			Status:         "paid",
		})
		if err != nil {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, err
		}
	}
	if shouldInsertWebhookOutbox(in) {
		if _, err := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
			OrganizationID: optionalUUIDToPg(&in.OrganizationID),
			Topic:          in.OutboxTopic,
			EventType:      in.OutboxEventType,
			Payload:        webhookOutboxPayload(in),
			AggregateType:  in.OutboxAggregateType,
			AggregateID:    in.OutboxAggregateID,
			IdempotencyKey: optionalStringToPgText(in.OutboxIdempotencyKey),
		}); err != nil {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, err
		}
	}

	if err := s.auditPaymentWebhookTx(ctx, tx, in, false, machineUUIDPtr(ord.MachineID)); err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}

	return appcommerce.ApplyPaymentProviderWebhookResult{
		Replay:        false,
		Order:         mapOrder(ord),
		Payment:       mapPayment(pay),
		Attempt:       mapPaymentAttemptView(attemptRow),
		ProviderRowID: ev.ID,
	}, nil
}

func shouldInsertWebhookOutbox(in appcommerce.ApplyPaymentProviderWebhookInput) bool {
	return strings.TrimSpace(in.OutboxTopic) != "" &&
		strings.TrimSpace(in.OutboxEventType) != "" &&
		strings.TrimSpace(in.OutboxAggregateType) != "" &&
		in.OutboxAggregateID != uuid.Nil &&
		strings.TrimSpace(in.OutboxIdempotencyKey) != ""
}

func webhookOutboxPayload(in appcommerce.ApplyPaymentProviderWebhookInput) []byte {
	if len(in.OutboxPayload) > 0 {
		return in.OutboxPayload
	}
	b, err := json.Marshal(map[string]any{
		"source":           "payment_webhook",
		"order_id":         in.OrderID.String(),
		"payment_id":       in.PaymentID.String(),
		"provider":         strings.TrimSpace(in.Provider),
		"webhook_event_id": strings.TrimSpace(in.WebhookEventID),
		"payment_state":    strings.TrimSpace(in.NormalizedPaymentState),
	})
	if err != nil {
		return []byte(`{}`)
	}
	return b
}

func (s *Store) webhookReplayAfterUniqueViolation(ctx context.Context, in appcommerce.ApplyPaymentProviderWebhookInput) (appcommerce.ApplyPaymentProviderWebhookResult, error) {
	if res, err := s.webhookReplayByProviderRef(ctx, in); err == nil {
		return res, nil
	} else if !isNoRows(err) {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	if strings.TrimSpace(in.WebhookEventID) == "" {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, errors.New("postgres: payment_provider_events unique violation without replay match")
	}
	if res, err := s.webhookReplayByWebhookEventID(ctx, in); err == nil {
		return res, nil
	} else if !isNoRows(err) {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	return appcommerce.ApplyPaymentProviderWebhookResult{}, errors.New("postgres: payment_provider_events unique violation without replay match")
}

func (s *Store) webhookReplayByProviderRef(ctx context.Context, in appcommerce.ApplyPaymentProviderWebhookInput) (appcommerce.ApplyPaymentProviderWebhookResult, error) {
	q := db.New(s.pool)
	existingEv, err := q.GetPaymentProviderEventByProviderRef(ctx, db.GetPaymentProviderEventByProviderRefParams{
		Provider:    in.Provider,
		ProviderRef: pgtype.Text{String: in.ProviderReference, Valid: true},
	})
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	return s.webhookReplayResultFromEvent(ctx, q, existingEv, in)
}

func (s *Store) webhookReplayByWebhookEventID(ctx context.Context, in appcommerce.ApplyPaymentProviderWebhookInput) (appcommerce.ApplyPaymentProviderWebhookResult, error) {
	q := db.New(s.pool)
	existingEv, err := q.GetPaymentProviderEventByWebhookEventID(ctx, db.GetPaymentProviderEventByWebhookEventIDParams{
		Provider:       in.Provider,
		WebhookEventID: pgtype.Text{String: strings.TrimSpace(in.WebhookEventID), Valid: true},
	})
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	return s.webhookReplayResultFromEvent(ctx, q, existingEv, in)
}

func webhookAmountCurrencyMatches(pay db.Payment, ord db.Order, in appcommerce.ApplyPaymentProviderWebhookInput) error {
	if pay.AmountMinor != ord.TotalMinor {
		return appcommerce.ErrWebhookAmountCurrencyMismatch
	}
	if strings.ToUpper(strings.TrimSpace(pay.Currency)) != strings.ToUpper(strings.TrimSpace(ord.Currency)) {
		return appcommerce.ErrWebhookAmountCurrencyMismatch
	}
	if in.ProviderAmountMinor != nil && ord.TotalMinor != *in.ProviderAmountMinor {
		return appcommerce.ErrWebhookAmountCurrencyMismatch
	}
	if in.Currency != nil && strings.TrimSpace(*in.Currency) != "" {
		if normalizeWebhookCurrency(pay.Currency) != normalizeWebhookCurrency(*in.Currency) {
			return appcommerce.ErrWebhookAmountCurrencyMismatch
		}
	}
	return nil
}

func normalizeWebhookCurrency(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func webhookLateDeliveryAgainstTerminalState(ord db.Order, pay db.Payment) bool {
	switch strings.TrimSpace(ord.Status) {
	case "completed", "failed", "cancelled":
		return true
	default:
	}
	switch strings.TrimSpace(pay.State) {
	case "refunded", "failed", "expired", "canceled":
		return true
	default:
		return false
	}
}
