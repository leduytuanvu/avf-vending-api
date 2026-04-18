// Long-running reconciliation follow-up should use the optional Temporal workflow boundary
// (internal/app/workfloworch) when enabled; this package stays synchronous batch/scan logic only.
package reliability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
)

// Service implements batch reconciliation scans and explicit per-row recovery decisions.
type Service struct {
	payments StuckPaymentFinder
	commands StuckCommandFinder
	vends    OrphanVendFinder
	outbox   domainreliability.OutboxRepository
}

// NewService constructs a reconciler; individual scan methods require their corresponding dependency.
func NewService(d Deps) *Service {
	return &Service{
		payments: d.Payments,
		commands: d.Commands,
		vends:    d.Vends,
		outbox:   d.Outbox,
	}
}

var _ Reconciler = (*Service)(nil)

// DefaultRecoveryPolicy returns conservative operator-oriented thresholds (adjust per environment).
func DefaultRecoveryPolicy() RecoveryPolicy {
	return RecoveryPolicy{
		PaymentStatesNonTerminal: []string{"created", "authorized"},
		PaymentMinAge:            30 * time.Minute,
		CommandMinAge:            15 * time.Minute,
		VendStatesOrphan:         []string{"pending", "in_progress"},
		VendMinAge:               30 * time.Minute,
		OrderStatusesOrphanVend:  []string{"paid", "vending"},
		OutboxMinAge:             2 * time.Minute,
		OutboxMaxPublishAttempts: 24,
		OutboxPublishBackoffBase: time.Second,
		OutboxPublishBackoffMax:  5 * time.Minute,
	}
}

// NormalizeRecoveryPolicy fills zero-valued slices and durations from DefaultRecoveryPolicy.
func NormalizeRecoveryPolicy(p RecoveryPolicy) RecoveryPolicy {
	def := DefaultRecoveryPolicy()
	if len(p.PaymentStatesNonTerminal) == 0 {
		p.PaymentStatesNonTerminal = def.PaymentStatesNonTerminal
	}
	if p.PaymentMinAge == 0 {
		p.PaymentMinAge = def.PaymentMinAge
	}
	if p.CommandMinAge == 0 {
		p.CommandMinAge = def.CommandMinAge
	}
	if len(p.VendStatesOrphan) == 0 {
		p.VendStatesOrphan = def.VendStatesOrphan
	}
	if p.VendMinAge == 0 {
		p.VendMinAge = def.VendMinAge
	}
	if len(p.OrderStatusesOrphanVend) == 0 {
		p.OrderStatusesOrphanVend = def.OrderStatusesOrphanVend
	}
	if p.OutboxMinAge == 0 {
		p.OutboxMinAge = def.OutboxMinAge
	}
	if p.OutboxMaxPublishAttempts <= 0 {
		p.OutboxMaxPublishAttempts = def.OutboxMaxPublishAttempts
	}
	if p.OutboxPublishBackoffBase <= 0 {
		p.OutboxPublishBackoffBase = def.OutboxPublishBackoffBase
	}
	if p.OutboxPublishBackoffMax <= 0 {
		p.OutboxPublishBackoffMax = def.OutboxPublishBackoffMax
	}
	return p
}

// ValidateRecoveryPolicy rejects policies that would fan out unbounded or zero-threshold scans.
func ValidateRecoveryPolicy(p RecoveryPolicy) error {
	if len(p.PaymentStatesNonTerminal) == 0 {
		return errors.Join(ErrInvalidArgument, errors.New("payment_states_non_terminal is required"))
	}
	if p.PaymentMinAge <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("payment_min_age must be positive"))
	}
	if p.CommandMinAge <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("command_min_age must be positive"))
	}
	if len(p.VendStatesOrphan) == 0 {
		return errors.Join(ErrInvalidArgument, errors.New("vend_states_orphan is required"))
	}
	if p.VendMinAge <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("vend_min_age must be positive"))
	}
	if len(p.OrderStatusesOrphanVend) == 0 {
		return errors.Join(ErrInvalidArgument, errors.New("order_statuses_orphan_vend is required"))
	}
	if p.OutboxMinAge <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("outbox_min_age must be positive"))
	}
	if p.OutboxMaxPublishAttempts < 2 {
		return errors.Join(ErrInvalidArgument, errors.New("outbox_max_publish_attempts must be at least 2"))
	}
	if p.OutboxPublishBackoffBase <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("outbox_publish_backoff_base must be positive"))
	}
	if p.OutboxPublishBackoffMax < p.OutboxPublishBackoffBase {
		return errors.Join(ErrInvalidArgument, errors.New("outbox_publish_backoff_max must be >= outbox_publish_backoff_base"))
	}
	return nil
}

func validateScanLimits(limits ScanLimits) error {
	if limits.MaxItems <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("limits.max_items must be positive"))
	}
	return nil
}

func validateRunContext(run ScanRunContext) error {
	if run.Now.IsZero() {
		return errors.Join(ErrInvalidArgument, errors.New("run.now must be set"))
	}
	return nil
}

// ScanStuckPayments lists stuck payment candidates and attaches explicit recovery decisions.
func (s *Service) ScanStuckPayments(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) (StuckPaymentScanResult, error) {
	if err := validateRunContext(run); err != nil {
		return StuckPaymentScanResult{}, err
	}
	if err := ValidateRecoveryPolicy(policy); err != nil {
		return StuckPaymentScanResult{}, err
	}
	if err := validateScanLimits(limits); err != nil {
		return StuckPaymentScanResult{}, err
	}
	if s.payments == nil {
		return StuckPaymentScanResult{}, ErrNotConfigured
	}
	candidates, err := s.payments.FindStuckPayments(ctx, run, policy, limits)
	if err != nil {
		return StuckPaymentScanResult{}, err
	}
	out := make([]PaymentRecoveryDecision, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, s.DecidePaymentRecovery(run, policy, c))
	}
	return StuckPaymentScanResult{Run: run, Decisions: out}, nil
}

// ScanStuckCommands lists stale command ledger rows and recommends edge republish review.
func (s *Service) ScanStuckCommands(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) (StuckCommandScanResult, error) {
	if err := validateRunContext(run); err != nil {
		return StuckCommandScanResult{}, err
	}
	if err := ValidateRecoveryPolicy(policy); err != nil {
		return StuckCommandScanResult{}, err
	}
	if err := validateScanLimits(limits); err != nil {
		return StuckCommandScanResult{}, err
	}
	if s.commands == nil {
		return StuckCommandScanResult{}, ErrNotConfigured
	}
	candidates, err := s.commands.FindStaleCommands(ctx, run, policy, limits)
	if err != nil {
		return StuckCommandScanResult{}, err
	}
	out := make([]CommandRecoveryDecision, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, s.DecideCommandRecovery(run, policy, c))
	}
	return StuckCommandScanResult{Run: run, Decisions: out}, nil
}

// ScanOrphanVendSessions lists vend sessions stalled relative to order lifecycle.
func (s *Service) ScanOrphanVendSessions(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) (OrphanVendScanResult, error) {
	if err := validateRunContext(run); err != nil {
		return OrphanVendScanResult{}, err
	}
	if err := ValidateRecoveryPolicy(policy); err != nil {
		return OrphanVendScanResult{}, err
	}
	if err := validateScanLimits(limits); err != nil {
		return OrphanVendScanResult{}, err
	}
	if s.vends == nil {
		return OrphanVendScanResult{}, ErrNotConfigured
	}
	candidates, err := s.vends.FindOrphanVendSessions(ctx, run, policy, limits)
	if err != nil {
		return OrphanVendScanResult{}, err
	}
	out := make([]VendRecoveryDecision, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, s.DecideVendRecovery(run, policy, c))
	}
	return OrphanVendScanResult{Run: run, Decisions: out}, nil
}

// PlanOutboxRepublishBatch lists unpublished outbox work and marks which rows are safe to (re)publish now.
func (s *Service) PlanOutboxRepublishBatch(ctx context.Context, run ScanRunContext, policy RecoveryPolicy, limits ScanLimits) (OutboxReplayPlan, error) {
	if err := validateRunContext(run); err != nil {
		return OutboxReplayPlan{}, err
	}
	if err := ValidateRecoveryPolicy(policy); err != nil {
		return OutboxReplayPlan{}, err
	}
	if err := validateScanLimits(limits); err != nil {
		return OutboxReplayPlan{}, err
	}
	if s.outbox == nil {
		return OutboxReplayPlan{}, ErrNotConfigured
	}
	pipeline, err := s.outbox.GetOutboxPipelineStats(ctx)
	if err != nil {
		return OutboxReplayPlan{}, err
	}
	events, err := s.outbox.ListUnpublished(ctx, limits.MaxItems)
	if err != nil {
		return OutboxReplayPlan{}, err
	}
	out := make([]OutboxReplayDecision, 0, len(events))
	for _, ev := range events {
		out = append(out, s.DecideOutboxReplay(run, policy, ev))
	}
	return OutboxReplayPlan{Run: run, Decisions: out, Pipeline: pipeline}, nil
}

// DecidePaymentRecovery is a pure policy decision for one payment candidate (traceable, idempotent).
func (s *Service) DecidePaymentRecovery(run ScanRunContext, policy RecoveryPolicy, c StuckPaymentCandidate) PaymentRecoveryDecision {
	age := run.Now.Sub(c.CreatedAt)
	if !containsString(policy.PaymentStatesNonTerminal, c.State) {
		return PaymentRecoveryDecision{
			Candidate:  c,
			Action:     ActionNoop,
			ReasonCode: ReasonNoopAlreadyTerminal,
			TraceNote:  fmt.Sprintf("payment=%s state=%s not in non-terminal policy set", c.PaymentID, c.State),
		}
	}
	if age < policy.PaymentMinAge {
		return PaymentRecoveryDecision{
			Candidate:  c,
			Action:     ActionNoop,
			ReasonCode: ReasonNoopFresh,
			TraceNote:  fmt.Sprintf("payment=%s age=%s below payment_min_age", c.PaymentID, age.String()),
		}
	}
	return PaymentRecoveryDecision{
		Candidate:  c,
		Action:     ActionEscalatePaymentReview,
		ReasonCode: ReasonPaymentAgedNonTerminal,
		TraceNote: fmt.Sprintf("payment=%s order=%s state=%s age=%s action=escalate_payment_review",
			c.PaymentID, c.OrderID, c.State, age.String()),
	}
}

// DecideCommandRecovery recommends republish review for stale command ledger rows (publish ≠ device action).
func (s *Service) DecideCommandRecovery(run ScanRunContext, policy RecoveryPolicy, c StuckCommandCandidate) CommandRecoveryDecision {
	age := run.Now.Sub(c.CreatedAt)
	if age < policy.CommandMinAge {
		return CommandRecoveryDecision{
			Candidate:  c,
			Action:     ActionNoop,
			ReasonCode: ReasonNoopFresh,
			TraceNote:  fmt.Sprintf("command=%s machine=%s seq=%d age=%s below command_min_age", c.CommandID, c.MachineID, c.Sequence, age.String()),
		}
	}
	return CommandRecoveryDecision{
		Candidate:  c,
		Action:     ActionSuggestCommandRepublish,
		ReasonCode: ReasonCommandLedgerStale,
		TraceNote: fmt.Sprintf("command=%s machine=%s seq=%d type=%s age=%s action=suggest_command_republish",
			c.CommandID, c.MachineID, c.Sequence, c.CommandType, age.String()),
	}
}

// DecideVendRecovery escalates orphan/stalled vend work for fulfillment reconciliation.
func (s *Service) DecideVendRecovery(run ScanRunContext, policy RecoveryPolicy, c OrphanVendCandidate) VendRecoveryDecision {
	age := run.Now.Sub(c.VendCreatedAt)
	if !containsString(policy.VendStatesOrphan, c.VendState) {
		return VendRecoveryDecision{
			Candidate:  c,
			Action:     ActionNoop,
			ReasonCode: ReasonNoopAlreadyTerminal,
			TraceNote:  fmt.Sprintf("order=%s slot=%d vend_state=%s not tracked as orphan", c.OrderID, c.SlotIndex, c.VendState),
		}
	}
	if !containsString(policy.OrderStatusesOrphanVend, c.OrderStatus) {
		return VendRecoveryDecision{
			Candidate:  c,
			Action:     ActionNoop,
			ReasonCode: ReasonNoopPolicy,
			TraceNote:  fmt.Sprintf("order=%s status=%s not in orphan order status set", c.OrderID, c.OrderStatus),
		}
	}
	if age < policy.VendMinAge {
		return VendRecoveryDecision{
			Candidate:  c,
			Action:     ActionNoop,
			ReasonCode: ReasonNoopFresh,
			TraceNote:  fmt.Sprintf("order=%s slot=%d age=%s below vend_min_age", c.OrderID, c.SlotIndex, age.String()),
		}
	}
	return VendRecoveryDecision{
		Candidate:  c,
		Action:     ActionEscalateVendRecovery,
		ReasonCode: ReasonVendStalledVsOrder,
		TraceNote: fmt.Sprintf("order=%s machine=%s slot=%d vend=%s order_status=%s age=%s action=escalate_vend_recovery",
			c.OrderID, c.MachineID, c.SlotIndex, c.VendState, c.OrderStatus, age.String()),
	}
}

// DecideOutboxReplay determines whether an unpublished outbox row should be published (or deferred).
func (s *Service) DecideOutboxReplay(run ScanRunContext, policy RecoveryPolicy, ev domainreliability.OutboxEvent) OutboxReplayDecision {
	if ev.PublishedAt != nil {
		return OutboxReplayDecision{
			Event:           ev,
			ShouldRepublish: false,
			ReasonCode:      ReasonNoopAlreadyTerminal,
			TraceNote:       fmt.Sprintf("outbox_id=%d already published", ev.ID),
		}
	}
	if ev.DeadLetteredAt != nil {
		return OutboxReplayDecision{
			Event:           ev,
			ShouldRepublish: false,
			ReasonCode:      ReasonOutboxDeadLettered,
			TraceNote:       fmt.Sprintf("outbox_id=%d dead_lettered_at=%s", ev.ID, ev.DeadLetteredAt.Format(time.RFC3339Nano)),
		}
	}
	if ev.NextPublishAfter != nil && run.Now.Before(*ev.NextPublishAfter) {
		return OutboxReplayDecision{
			Event:           ev,
			ShouldRepublish: false,
			ReasonCode:      ReasonOutboxPublishBackoff,
			TraceNote: fmt.Sprintf("outbox_id=%d next_publish_after=%s",
				ev.ID, ev.NextPublishAfter.Format(time.RFC3339Nano)),
		}
	}
	age := run.Now.Sub(ev.CreatedAt)
	if age < policy.OutboxMinAge {
		return OutboxReplayDecision{
			Event:           ev,
			ShouldRepublish: false,
			ReasonCode:      ReasonNoopFresh,
			TraceNote:       fmt.Sprintf("outbox_id=%d age=%s below outbox_min_age", ev.ID, age.String()),
		}
	}
	return OutboxReplayDecision{
		Event:           ev,
		ShouldRepublish: true,
		ReasonCode:      ReasonOutboxAgedUnpublished,
		TraceNote: fmt.Sprintf("outbox_id=%d topic=%s age=%s attempts=%d action=republish_outbox",
			ev.ID, ev.Topic, age.String(), ev.PublishAttemptCount),
	}
}

func containsString(set []string, v string) bool {
	for _, s := range set {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}
