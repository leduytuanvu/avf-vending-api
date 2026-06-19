package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// emitVendSuccessFinancialLedgerTx records payment_captured for zero-loss reconciliation (GAP 7).
func emitVendSuccessFinancialLedgerTx(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	orderID uuid.UUID,
	paymentID uuid.UUID,
	amountMinor int64,
	currency string,
	correlationID uuid.UUID,
	idempotencyKey string,
) error {
	if amountMinor <= 0 {
		return nil
	}
	meta, err := json.Marshal(map[string]any{
		"idempotency_key": idempotencyKey,
		"source":          "commerce_vend_fulfillment",
	})
	if err != nil {
		return err
	}
	corr := pgtype.UUID{}
	if correlationID != uuid.Nil {
		corr = pgtype.UUID{Bytes: correlationID, Valid: true}
	}
	_, err = q.InsertFinancialLedgerEntry(ctx, db.InsertFinancialLedgerEntryParams{
		EntryType:         "payment_captured",
		SignedAmountMinor: amountMinor,
		Currency:          currency,
		OccurredAt:        time.Now().UTC(),
		MachineID:         pgtype.UUID{Bytes: machineID, Valid: true},
		OrderID:           pgtype.UUID{Bytes: orderID, Valid: true},
		PaymentID:         pgtype.UUID{Bytes: paymentID, Valid: true},
		CorrelationID:     corr,
		Metadata:          meta,
	})
	return err
}
