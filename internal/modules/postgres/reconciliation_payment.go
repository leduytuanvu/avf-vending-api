package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ApplyReconciledPaymentTransition updates payment state only from created|authorized, records a payment_attempt row.
// DryRun returns the current payment row without mutating persistence (caller should log/audit externally).
// If the payment is already terminal, returns the current row without error (idempotent).
func (s *Store) ApplyReconciledPaymentTransition(ctx context.Context, in domaincommerce.ReconciledPaymentTransitionInput) (domaincommerce.Payment, error) {
	if in.PaymentID == uuid.Nil {
		return domaincommerce.Payment{}, errors.New("postgres: payment_id is required")
	}
	to := strings.ToLower(strings.TrimSpace(in.ToState))
	if to != "captured" && to != "failed" {
		return domaincommerce.Payment{}, fmt.Errorf("postgres: invalid reconciler target state %q", in.ToState)
	}

	q := db.New(s.pool)
	cur, err := q.GetPaymentByID(ctx, in.PaymentID)
	if err != nil {
		return domaincommerce.Payment{}, err
	}

	if in.DryRun {
		return mapPayment(cur), nil
	}

	if cur.State != "created" && cur.State != "authorized" {
		return mapPayment(cur), nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domaincommerce.Payment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := db.New(tx)

	updated, err := qtx.UpdatePaymentStateForReconciliation(ctx, db.UpdatePaymentStateForReconciliationParams{
		ID:    in.PaymentID,
		State: to,
	})
	if err != nil {
		if isNoRows(err) {
			if err := tx.Commit(ctx); err != nil {
				return domaincommerce.Payment{}, err
			}
			return mapPayment(cur), nil
		}
		return domaincommerce.Payment{}, err
	}

	attemptPayload, err := json.Marshal(map[string]any{
		"reason":        in.Reason,
		"provider_hint": json.RawMessage(coerceJSON(in.ProviderHint)),
		"source":        "reconciler.provider_probe",
	})
	if err != nil {
		return domaincommerce.Payment{}, err
	}
	ref := "reconciler:provider_probe"
	if _, err := qtx.InsertPaymentAttempt(ctx, db.InsertPaymentAttemptParams{
		PaymentID:         in.PaymentID,
		ProviderReference: &ref,
		State:             "reconciliation.probe." + to,
		Payload:           attemptPayload,
	}); err != nil {
		return domaincommerce.Payment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domaincommerce.Payment{}, err
	}
	return mapPayment(updated), nil
}

func coerceJSON(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	if json.Valid(b) {
		return b
	}
	s, _ := json.Marshal(string(b))
	return s
}
