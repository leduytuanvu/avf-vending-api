package postgres

import (
	"context"
	"strings"

	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RecoveryScanRepository backs app/reliability scans with commerce and command_ledger queries.
type RecoveryScanRepository struct {
	pool *pgxpool.Pool
}

func NewRecoveryScanRepository(pool *pgxpool.Pool) *RecoveryScanRepository {
	return &RecoveryScanRepository{pool: pool}
}

var _ appreliability.StuckPaymentFinder = (*RecoveryScanRepository)(nil)
var _ appreliability.StuckCommandFinder = (*RecoveryScanRepository)(nil)
var _ appreliability.OrphanVendFinder = (*RecoveryScanRepository)(nil)

func (r *RecoveryScanRepository) FindStuckPayments(ctx context.Context, run appreliability.ScanRunContext, policy appreliability.RecoveryPolicy, limits appreliability.ScanLimits) ([]appreliability.StuckPaymentCandidate, error) {
	before := run.Now.Add(-policy.PaymentMinAge)
	rows, err := db.New(r.pool).ListPaymentsPendingTimeout(ctx, before, limits.MaxItems)
	if err != nil {
		return nil, err
	}
	out := make([]appreliability.StuckPaymentCandidate, 0, len(rows))
	for _, row := range rows {
		if !containsFold(policy.PaymentStatesNonTerminal, row.State) {
			continue
		}
		out = append(out, appreliability.StuckPaymentCandidate{
			PaymentID:   row.ID,
			OrderID:     row.OrderID,
			State:       row.State,
			AmountMinor: row.AmountMinor,
			Currency:    row.Currency,
			CreatedAt:   row.CreatedAt,
		})
	}
	return out, nil
}

func (r *RecoveryScanRepository) FindStaleCommands(ctx context.Context, run appreliability.ScanRunContext, policy appreliability.RecoveryPolicy, limits appreliability.ScanLimits) ([]appreliability.StuckCommandCandidate, error) {
	before := run.Now.Add(-policy.CommandMinAge)
	rows, err := db.New(r.pool).ListStaleCommandLedgerEntries(ctx, before, limits.MaxItems)
	if err != nil {
		return nil, err
	}
	out := make([]appreliability.StuckCommandCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, appreliability.StuckCommandCandidate{
			CommandID:   row.ID,
			MachineID:   row.MachineID,
			Sequence:    row.Sequence,
			CommandType: row.CommandType,
			CreatedAt:   row.CreatedAt,
		})
	}
	return out, nil
}

func (r *RecoveryScanRepository) FindOrphanVendSessions(ctx context.Context, run appreliability.ScanRunContext, policy appreliability.RecoveryPolicy, limits appreliability.ScanLimits) ([]appreliability.OrphanVendCandidate, error) {
	before := run.Now.Add(-policy.VendMinAge)
	rows, err := db.New(r.pool).ListVendSessionsStuckForReconciliation(ctx, before, limits.MaxItems)
	if err != nil {
		return nil, err
	}
	out := make([]appreliability.OrphanVendCandidate, 0, len(rows))
	for _, row := range rows {
		if !containsFold(policy.VendStatesOrphan, row.State) {
			continue
		}
		if !containsFold(policy.OrderStatusesOrphanVend, row.OrderStatus) {
			continue
		}
		out = append(out, appreliability.OrphanVendCandidate{
			OrderID:       row.OrderID,
			MachineID:     row.MachineID,
			SlotIndex:     row.SlotIndex,
			VendState:     row.State,
			OrderStatus:   row.OrderStatus,
			VendCreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func containsFold(set []string, v string) bool {
	for _, s := range set {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}
