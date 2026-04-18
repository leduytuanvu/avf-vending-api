package workfloworch

import (
	"context"
)

// Kind identifies a class of long-running workflow (extension point; not yet bound to SDK workflows).
type Kind string

const (
	KindReconciliationFollowUp Kind = "reconciliation_follow_up"
	KindDelayedCompensation    Kind = "delayed_compensation"
	KindHumanReviewEscalation  Kind = "human_review_escalation"
)

// StartInput carries workflow identity and optional arguments for Temporal ExecuteWorkflow (future).
type StartInput struct {
	Kind       Kind
	WorkflowID string
	Args       []any
}

// Boundary schedules durable workflows. Implementations must be safe to call from async paths;
// Start must not block on business I/O beyond Temporal client RPC.
type Boundary interface {
	Enabled() bool
	Start(ctx context.Context, in StartInput) error
	Close() error
}
