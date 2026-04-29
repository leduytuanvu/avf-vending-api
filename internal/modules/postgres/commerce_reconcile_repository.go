package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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
var _ domaincommerce.ReconciliationCaseWriter = (*CommerceReconcileRepository)(nil)

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
			Session:        mapVendFromStuckReconcileRow(row),
			OrganizationID: row.OrganizationID,
			OrderStatus:    row.OrderStatus,
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

func (r *CommerceReconcileRepository) ListPaidOrdersWithoutVendStart(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.PaidOrderVendStartCandidate, error) {
	rows, err := db.New(r.pool).ListPaidOrdersWithoutVendStart(ctx, db.ListPaidOrdersWithoutVendStartParams{
		UpdatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.PaidOrderVendStartCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, domaincommerce.PaidOrderVendStartCandidate{
			OrderID:        row.OrderID,
			OrganizationID: row.OrganizationID,
			MachineID:      row.MachineID,
			PaymentID:      row.PaymentID,
			Provider:       row.Provider,
			PaymentState:   row.PaymentState,
			VendSessionID:  row.VendSessionID,
			VendState:      row.VendState,
			UpdatedAt:      row.UpdatedAt,
		})
	}
	return out, nil
}

func (r *CommerceReconcileRepository) ListPaidVendFailuresForReview(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.PaidVendFailureCandidate, error) {
	rows, err := db.New(r.pool).ListPaidVendFailuresForReview(ctx, db.ListPaidVendFailuresForReviewParams{
		CompletedAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.PaidVendFailureCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, domaincommerce.PaidVendFailureCandidate{
			OrderID:        row.OrderID,
			OrganizationID: row.OrganizationID,
			MachineID:      row.MachineID,
			PaymentID:      row.PaymentID,
			Provider:       row.Provider,
			PaymentState:   row.PaymentState,
			VendSessionID:  row.VendSessionID,
			VendState:      row.VendState,
			CompletedAt:    row.CompletedAt.Time,
		})
	}
	return out, nil
}

func (r *CommerceReconcileRepository) ListRefundsPendingTooLong(ctx context.Context, before time.Time, limit int32) ([]domaincommerce.RefundPendingCandidate, error) {
	rows, err := db.New(r.pool).ListRefundsPendingTooLong(ctx, db.ListRefundsPendingTooLongParams{
		CreatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.RefundPendingCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, domaincommerce.RefundPendingCandidate{
			RefundID:       row.RefundID,
			PaymentID:      row.PaymentID,
			OrderID:        row.OrderID,
			OrganizationID: row.OrganizationID,
			Provider:       row.Provider,
			RefundState:    row.RefundState,
			AmountMinor:    row.AmountMinor,
			Currency:       row.Currency,
			CreatedAt:      row.CreatedAt,
		})
	}
	return out, nil
}

func (r *CommerceReconcileRepository) UpsertReconciliationCase(ctx context.Context, in domaincommerce.ReconciliationCaseInput) (domaincommerce.ReconciliationCase, error) {
	row, err := db.New(r.pool).UpsertCommerceReconciliationCase(ctx, db.UpsertCommerceReconciliationCaseParams{
		OrganizationID:  in.OrganizationID,
		CaseType:        in.CaseType,
		Severity:        in.Severity,
		Reason:          in.Reason,
		Metadata:        coerceJSON(in.Metadata),
		OrderID:         optionalUUIDToPg(in.OrderID),
		PaymentID:       optionalUUIDToPg(in.PaymentID),
		VendSessionID:   optionalUUIDToPg(in.VendSessionID),
		RefundID:        optionalUUIDToPg(in.RefundID),
		MachineID:       optionalUUIDToPg(in.MachineID),
		Provider:        optionalStringPtrToPgText(in.Provider),
		ProviderEventID: optionalInt64ToPgInt8(in.ProviderEventID),
		CorrelationKey:  pgtype.Text{String: strings.TrimSpace(in.CorrelationKey), Valid: true},
	})
	if err != nil {
		return domaincommerce.ReconciliationCase{}, err
	}
	return mapCommerceReconciliationCase(row), nil
}

func optionalInt64ToPgInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func pgInt8ToPtr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func mapCommerceReconciliationCase(row db.CommerceReconciliationCase) domaincommerce.ReconciliationCase {
	return domaincommerce.ReconciliationCase{
		ID:              row.ID,
		OrganizationID:  row.OrganizationID,
		CaseType:        row.CaseType,
		Status:          row.Status,
		Severity:        row.Severity,
		OrderID:         pgUUIDToPtr(row.OrderID),
		PaymentID:       pgUUIDToPtr(row.PaymentID),
		VendSessionID:   pgUUIDToPtr(row.VendSessionID),
		RefundID:        pgUUIDToPtr(row.RefundID),
		MachineID:       pgUUIDToPtr(row.MachineID),
		Provider:        pgTextToStringPtr(row.Provider),
		ProviderEventID: pgInt8ToPtr(row.ProviderEventID),
		CorrelationKey:  strings.TrimSpace(row.CorrelationKey),
		Reason:          row.Reason,
		Metadata:        row.Metadata,
		FirstDetectedAt: row.FirstDetectedAt,
		LastDetectedAt:  row.LastDetectedAt,
		ResolvedAt:      pgTimestamptzToTimePtr(row.ResolvedAt),
		ResolvedBy:      pgUUIDToPtr(row.ResolvedBy),
		ResolutionNote:  pgTextToStringPtr(row.ResolutionNote),
	}
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
