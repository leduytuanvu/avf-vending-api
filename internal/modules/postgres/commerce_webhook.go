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
		if !existingEv.PaymentID.Valid || uuid.UUID(existingEv.PaymentID.Bytes) != in.PaymentID {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, errors.Join(appcommerce.ErrOrgMismatch, errors.New("provider reference already bound to a different payment"))
		}
		pay, pErr := q.GetPaymentByID(ctx, in.PaymentID)
		if pErr != nil {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, pErr
		}
		ord, oErr := q.GetOrderByID(ctx, in.OrderID)
		if oErr != nil {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, oErr
		}
		if err := tx.Commit(ctx); err != nil {
			return appcommerce.ApplyPaymentProviderWebhookResult{}, err
		}
		return appcommerce.ApplyPaymentProviderWebhookResult{
			Replay:        true,
			Order:         mapOrder(ord),
			Payment:       mapPayment(pay),
			ProviderRowID: existingEv.ID,
		}, nil
	}
	if !isNoRows(err) {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}

	pay, err := q.GetPaymentByID(ctx, in.PaymentID)
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
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
	ev, err := q.InsertPaymentProviderEvent(ctx, db.InsertPaymentProviderEventParams{
		PaymentID:           pgtype.UUID{Bytes: in.PaymentID, Valid: true},
		Provider:            in.Provider,
		ProviderRef:         pgtype.Text{String: in.ProviderReference, Valid: true},
		ProviderAmountMinor: amt,
		Currency:            cur,
		EventType:           in.EventType,
		Payload:             in.Payload,
	})
	if err != nil {
		if isUniqueViolation(err) {
			_ = tx.Rollback(ctx)
			return s.webhookReplayByProviderRef(ctx, in)
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

func (s *Store) webhookReplayByProviderRef(ctx context.Context, in appcommerce.ApplyPaymentProviderWebhookInput) (appcommerce.ApplyPaymentProviderWebhookResult, error) {
	q := db.New(s.pool)
	existingEv, err := q.GetPaymentProviderEventByProviderRef(ctx, db.GetPaymentProviderEventByProviderRefParams{
		Provider:    in.Provider,
		ProviderRef: pgtype.Text{String: in.ProviderReference, Valid: true},
	})
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	if !existingEv.PaymentID.Valid || uuid.UUID(existingEv.PaymentID.Bytes) != in.PaymentID {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, errors.Join(appcommerce.ErrOrgMismatch, errors.New("provider reference already bound to a different payment"))
	}
	pay, err := q.GetPaymentByID(ctx, in.PaymentID)
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	ord, err := q.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return appcommerce.ApplyPaymentProviderWebhookResult{}, err
	}
	return appcommerce.ApplyPaymentProviderWebhookResult{
		Replay:        true,
		Order:         mapOrder(ord),
		Payment:       mapPayment(pay),
		ProviderRowID: existingEv.ID,
	}, nil
}
