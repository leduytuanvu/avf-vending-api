package postgres

import (
	"context"
	"errors"
	"strings"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ appcommerce.PaymentWebhookPersistence = (*Store)(nil)

func paymentTransitionAllowed(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case "created":
		return to == "authorized" || to == "captured" || to == "failed"
	case "authorized":
		return to == "captured" || to == "failed"
	case "captured", "failed", "refunded":
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

	target := strings.TrimSpace(strings.ToLower(in.NormalizedPaymentState))
	if !paymentTransitionAllowed(pay.State, target) {
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
	ev, err := q.InsertPaymentProviderEvent(ctx, db.InsertPaymentProviderEventParams{
		PaymentID:           pgtype.UUID{Bytes: in.PaymentID, Valid: true},
		Provider:            in.Provider,
		ProviderRef:         pgtype.Text{String: in.ProviderReference, Valid: true},
		WebhookEventID:      webhookEv,
		ProviderAmountMinor: amt,
		Currency:            cur,
		EventType:           in.EventType,
		Payload:             in.Payload,
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
