package reliability

import (
	"context"

	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
)

// StuckPaymentFinder loads payment rows that are non-terminal and older than policy thresholds.
type StuckPaymentFinder interface {
	FindStuckPayments(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) ([]StuckPaymentCandidate, error)
}

// StuckCommandFinder loads command_ledger rows that are older than policy thresholds (publish vs ack is out of band).
type StuckCommandFinder interface {
	FindStaleCommands(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) ([]StuckCommandCandidate, error)
}

// OrphanVendFinder loads vend sessions stalled relative to order status (fulfillment vs payment is elsewhere).
type OrphanVendFinder interface {
	FindOrphanVendSessions(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) ([]OrphanVendCandidate, error)
}

// Deps wires reconciliation data sources; each may be nil if that scan path is unused.
type Deps struct {
	Payments StuckPaymentFinder
	Commands StuckCommandFinder
	Vends    OrphanVendFinder
	Outbox   domainreliability.OutboxRepository
}

// Reconciler is the worker-facing surface for batch scans and single-row decisions.
type Reconciler interface {
	ScanStuckPayments(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) (StuckPaymentScanResult, error)
	ScanStuckCommands(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) (StuckCommandScanResult, error)
	ScanOrphanVendSessions(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) (OrphanVendScanResult, error)
	PlanOutboxRepublishBatch(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) (OutboxReplayPlan, error)

	DecideOutboxReplay(run ScanRunContext, policy RecoveryPolicy, ev domainreliability.OutboxEvent) OutboxReplayDecision
	DecidePaymentRecovery(run ScanRunContext, policy RecoveryPolicy, c StuckPaymentCandidate) PaymentRecoveryDecision
	DecideCommandRecovery(run ScanRunContext, policy RecoveryPolicy, c StuckCommandCandidate) CommandRecoveryDecision
	DecideVendRecovery(run ScanRunContext, policy RecoveryPolicy, c OrphanVendCandidate) VendRecoveryDecision
}
