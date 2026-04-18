package reliability

import (
	"time"

	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/google/uuid"
)

// ReasonCode values are stable identifiers for metrics and log correlation (not end-user text).
const (
	ReasonPaymentAgedNonTerminal = "PAYMENT_AGED_NON_TERMINAL"
	ReasonCommandLedgerStale     = "COMMAND_LEDGER_STALE"
	ReasonVendStalledVsOrder     = "VEND_STALLED_VS_ORDER"
	ReasonOutboxAgedUnpublished  = "OUTBOX_AGED_UNPUBLISHED"
	ReasonOutboxDeadLettered     = "OUTBOX_DEAD_LETTERED"
	ReasonOutboxPublishBackoff   = "OUTBOX_PUBLISH_BACKOFF"
	ReasonNoopPolicy             = "NOOP_POLICY"
	ReasonNoopAlreadyTerminal    = "NOOP_ALREADY_TERMINAL"
	ReasonNoopFresh              = "NOOP_BELOW_AGE_THRESHOLD"
)

// RecoveryAction is what a worker should do next for a single candidate (no side effects in this package).
type RecoveryAction string

const (
	ActionNoop                    RecoveryAction = "noop"
	ActionRepublishOutbox         RecoveryAction = "republish_outbox"
	ActionEscalatePaymentReview   RecoveryAction = "escalate_payment_review"
	ActionSuggestCommandRepublish RecoveryAction = "suggest_command_republish"
	ActionEscalateVendRecovery    RecoveryAction = "escalate_vend_recovery"
)

// ScanRunContext carries a single reference time for one reconciliation tick (traceable, consistent batch).
type ScanRunContext struct {
	Now time.Time
}

// RecoveryPolicy holds age and state thresholds; workers may load from config and validate via ValidatePolicy.
type RecoveryPolicy struct {
	// Payments in these states older than PaymentMinAge are reported as stuck.
	PaymentStatesNonTerminal []string
	PaymentMinAge            time.Duration

	// Commands with created_at before (Now - CommandMinAge) are candidates for edge republish review.
	CommandMinAge time.Duration

	// Vend sessions in these states older than VendMinAge while order is in one of OrderStatusesOrphanVend.
	VendStatesOrphan        []string
	VendMinAge              time.Duration
	OrderStatusesOrphanVend []string

	// Unpublished outbox rows older than OutboxMinAge get republish recommendation.
	OutboxMinAge time.Duration

	// OutboxMaxPublishAttempts bounds retries: after a failed publish, if publish_attempt_count+1 >= this value,
	// Postgres quarantines the row (dead_lettered_at) and optional JetStream DLQ copy runs (see cmd/worker).
	// Default after NormalizeRecoveryPolicy is 24 unless overridden.
	OutboxMaxPublishAttempts int
	// OutboxPublishBackoffBase is the initial backoff after the first failed publish.
	OutboxPublishBackoffBase time.Duration
	// OutboxPublishBackoffMax caps exponential backoff between publish attempts.
	OutboxPublishBackoffMax time.Duration
}

// ScanLimits bounds batch size for list queries.
type ScanLimits struct {
	MaxItems int32
}

// StuckPaymentCandidate is a row-shaped finding for reconciliation (source: postgres).
type StuckPaymentCandidate struct {
	PaymentID   uuid.UUID
	OrderID     uuid.UUID
	State       string
	AmountMinor int64
	Currency    string
	CreatedAt   time.Time
}

// PaymentRecoveryDecision pairs a finding with an explicit recommended action and reason code.
type PaymentRecoveryDecision struct {
	Candidate  StuckPaymentCandidate
	Action     RecoveryAction
	ReasonCode string
	TraceNote  string
}

// StuckCommandCandidate is a ledger row that may need edge republish (command publish ≠ machine action).
type StuckCommandCandidate struct {
	CommandID   uuid.UUID
	MachineID   uuid.UUID
	Sequence    int64
	CommandType string
	CreatedAt   time.Time
}

// CommandRecoveryDecision recommends follow-up for a stale command ledger entry.
type CommandRecoveryDecision struct {
	Candidate  StuckCommandCandidate
	Action     RecoveryAction
	ReasonCode string
	TraceNote  string
}

// OrphanVendCandidate ties vend progress to order status for orphan detection.
type OrphanVendCandidate struct {
	OrderID       uuid.UUID
	MachineID     uuid.UUID
	SlotIndex     int32
	VendState     string
	OrderStatus   string
	VendCreatedAt time.Time
}

// VendRecoveryDecision recommends follow-up for a stalled vend relative to order lifecycle.
type VendRecoveryDecision struct {
	Candidate  OrphanVendCandidate
	Action     RecoveryAction
	ReasonCode string
	TraceNote  string
}

// OutboxReplayDecision describes whether an unpublished outbox row should be (re)published.
type OutboxReplayDecision struct {
	Event           domainreliability.OutboxEvent
	ShouldRepublish bool
	ReasonCode      string
	TraceNote       string
}

// StuckPaymentScanResult is the outcome of one stuck-payment scan pass.
type StuckPaymentScanResult struct {
	Run       ScanRunContext
	Decisions []PaymentRecoveryDecision
}

// StuckCommandScanResult is the outcome of one stale-command scan pass.
type StuckCommandScanResult struct {
	Run       ScanRunContext
	Decisions []CommandRecoveryDecision
}

// OrphanVendScanResult is the outcome of one orphan-vend scan pass.
type OrphanVendScanResult struct {
	Run       ScanRunContext
	Decisions []VendRecoveryDecision
}

// OutboxReplayPlan batches outbox republish recommendations.
type OutboxReplayPlan struct {
	Run       ScanRunContext
	Decisions []OutboxReplayDecision
	Pipeline  domainreliability.OutboxPipelineStats
}
