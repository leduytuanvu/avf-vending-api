package mqtt

import "strings"

// CommandLedgerLifecycleState is the enterprise command lifecycle vocabulary used
// by runbooks and transport adapters. Persistence-specific attempt states map into
// these labels so operators do not have to reason about broker implementation detail.
type CommandLedgerLifecycleState string

const (
	CommandLifecycleQueued    CommandLedgerLifecycleState = "queued"
	CommandLifecyclePublished CommandLedgerLifecycleState = "published"
	CommandLifecycleDelivered CommandLedgerLifecycleState = "delivered"
	CommandLifecycleAcked     CommandLedgerLifecycleState = "acked"
	CommandLifecycleExecuted  CommandLedgerLifecycleState = "executed"
	CommandLifecycleFailed    CommandLedgerLifecycleState = "failed"
	CommandLifecycleExpired   CommandLedgerLifecycleState = "expired"
	CommandLifecycleCanceled  CommandLedgerLifecycleState = "canceled"
)

// MapCommandAttemptStatusToLifecycle maps current command ledger / attempt states
// into the enterprise lifecycle labels. delivered, executed, and canceled are
// reserved for firmware/broker signals that are not distinct persistence states yet.
func MapCommandAttemptStatusToLifecycle(status string) (CommandLedgerLifecycleState, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "pending":
		return CommandLifecycleQueued, true
	case "published", "sent":
		return CommandLifecyclePublished, true
	case "delivered":
		return CommandLifecycleDelivered, true
	case "acked", "acknowledged", "completed", "duplicate":
		return CommandLifecycleAcked, true
	case "executed":
		return CommandLifecycleExecuted, true
	case "failed", "nack", "nacked", "ack_timeout":
		return CommandLifecycleFailed, true
	case "expired", "late":
		return CommandLifecycleExpired, true
	case "canceled", "cancelled", "superseded":
		return CommandLifecycleCanceled, true
	default:
		return "", false
	}
}
