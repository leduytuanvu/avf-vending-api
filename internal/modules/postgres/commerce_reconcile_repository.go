package postgres

import (
	"context"
	"errors"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CommerceReconcileRepository satisfies commerce reconciliation reads and lightweight order lookups.
type CommerceReconcileRepository struct {
	pool *pgxpool.Pool
}

func NewCommerceReconcileRepository(pool *pgxpool.Pool) *CommerceReconcileRepository {
	return &CommerceReconcileRepository{pool: pool}
}

var _ domaincommerce.ReconciliationReader = (*CommerceReconcileRepository)(nil)

func (r *CommerceReconcileRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (domaincommerce.Order, error) {
	row, err := db.New(r.pool).GetOrderByID(ctx, id)
	if err != nil {
		return domaincommerce.Order{}, err
	}
	return mapOrder(row), nil
}

func (r *CommerceReconcileRepository) ListPaymentsPendingTimeout(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.Payment, error) {
	rows, err := db.New(r.pool).ListPaymentsPendingTimeout(ctx, db.ListPaymentsPendingTimeoutParams{
		CreatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.Payment, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPayment(row))
	}
	return out, nil
}

func (r *CommerceReconcileRepository) ListOrdersWithUnresolvedPayment(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.Order, error) {
	rows, err := db.New(r.pool).ListOrdersWithUnresolvedPayment(ctx, db.ListOrdersWithUnresolvedPaymentParams{
		CreatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.Order, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOrder(row))
	}
	return out, nil
}

func (r *CommerceReconcileRepository) ListVendSessionsStuckForReconciliation(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.VendReconciliationCandidate, error) {
	rows, err := db.New(r.pool).ListVendSessionsStuckForReconciliation(ctx, db.ListVendSessionsStuckForReconciliationParams{
		CreatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.VendReconciliationCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, domaincommerce.VendReconciliationCandidate{
			Session:     mapVendFromStuckReconcileRow(row),
			OrderStatus: row.OrderStatus,
		})
	}
	return out, nil
}

func (r *CommerceReconcileRepository) ListPotentialDuplicatePayments(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.Payment, error) {
	rows, err := db.New(r.pool).ListPotentialDuplicatePayments(ctx, db.ListPotentialDuplicatePaymentsParams{
		CreatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.Payment, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPayment(row))
	}
	return out, nil
}

func (r *CommerceReconcileRepository) ListPaymentsForRefundReview(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.Payment, error) {
	rows, err := db.New(r.pool).ListPaymentsForRefundReview(ctx, db.ListPaymentsForRefundReviewParams{
		CreatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.Payment, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPayment(row))
	}
	return out, nil
}

func (r *CommerceReconcileRepository) ListStaleCommandLedgerEntries(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.CommandLedgerSummary, error) {
	rows, err := db.New(r.pool).ListStaleCommandLedgerEntries(ctx, db.ListStaleCommandLedgerEntriesParams{
		CreatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.CommandLedgerSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, domaincommerce.CommandLedgerSummary{
			ID:          row.ID,
			MachineID:   row.MachineID,
			Sequence:    row.Sequence,
			CommandType: row.CommandType,
			CreatedAt:   row.CreatedAt,
		})
	}
	return out, nil
}

// TouchVendSessionCorrelation preserves the first correlation_id written for a vend session (device HTTP traceability).
func (s *Store) TouchVendSessionCorrelation(ctx context.Context, orderID uuid.UUID, slotIndex int32, correlationID uuid.UUID) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres: nil store")
	}
	if correlationID == uuid.Nil {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
UPDATE vend_sessions
SET correlation_id = COALESCE(correlation_id, $3::uuid)
WHERE order_id = $1 AND slot_index = $2`,
		orderID, slotIndex, correlationID)
	return err
}
