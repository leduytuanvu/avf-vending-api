package workfloworch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Kind identifies a class of long-running workflow.
type Kind string

const (
	KindPaymentPendingTimeoutFollowUp  Kind = "payment_pending_timeout_follow_up"
	KindVendFailureAfterPaymentSuccess Kind = "vend_failed_after_payment_success"
	KindRefundOrchestration            Kind = "refund_orchestration"
	KindManualReviewEscalation         Kind = "manual_review_escalation"

	// Legacy aliases kept for compatibility with earlier call sites/tests.
	KindReconciliationFollowUp Kind = KindPaymentPendingTimeoutFollowUp
	KindDelayedCompensation    Kind = KindRefundOrchestration
	KindHumanReviewEscalation  Kind = KindManualReviewEscalation
)

// StartInput carries workflow identity and optional arguments for Temporal ExecuteWorkflow.
type StartInput struct {
	Kind       Kind
	WorkflowID string
	Args       []any
}

// PaymentPendingTimeoutInput schedules follow-up for an aged non-terminal payment.
type PaymentPendingTimeoutInput struct {
	PaymentID  uuid.UUID
	OrderID    uuid.UUID
	ObservedAt time.Time
	ReasonCode string
	TraceNote  string
}

// VendFailureAfterPaymentSuccessInput schedules compensation after a failed vend with captured payment.
type VendFailureAfterPaymentSuccessInput struct {
	OrganizationID uuid.UUID
	OrderID        uuid.UUID
	PaymentID      uuid.UUID
	VendID         uuid.UUID
	SlotIndex      int32
	FailureReason  string
	ObservedAt     time.Time
}

// RefundOrchestrationInput schedules refund follow-up on an existing review/manual pipeline.
type RefundOrchestrationInput struct {
	OrganizationID uuid.UUID
	OrderID        uuid.UUID
	PaymentID      uuid.UUID
	Reason         string
	RequestedAt    time.Time
}

// ManualReviewEscalationInput schedules a human-review escalation on the existing review/manual pipeline.
type ManualReviewEscalationInput struct {
	OrganizationID uuid.UUID
	OrderID        uuid.UUID
	PaymentID      uuid.UUID
	Reason         string
	RequestedAt    time.Time
}

// Boundary schedules durable workflows. Implementations must be safe to call from async paths;
// Start must not block on business I/O beyond Temporal client RPC.
type Boundary interface {
	Enabled() bool
	Start(ctx context.Context, in StartInput) error
	Close() error
}

func StartPaymentPendingTimeoutFollowUp(in PaymentPendingTimeoutInput) StartInput {
	return StartInput{
		Kind:       KindPaymentPendingTimeoutFollowUp,
		WorkflowID: workflowID("payment-pending-timeout", in.PaymentID),
		Args:       []any{normalizePaymentPendingTimeoutInput(in)},
	}
}

func StartVendFailureAfterPaymentSuccess(in VendFailureAfterPaymentSuccessInput) StartInput {
	return StartInput{
		Kind:       KindVendFailureAfterPaymentSuccess,
		WorkflowID: workflowID("vend-failure-after-payment", in.VendID),
		Args:       []any{normalizeVendFailureAfterPaymentSuccessInput(in)},
	}
}

func StartRefundOrchestration(in RefundOrchestrationInput) StartInput {
	return StartInput{
		Kind:       KindRefundOrchestration,
		WorkflowID: workflowID("refund-orchestration", in.PaymentID),
		Args:       []any{normalizeRefundOrchestrationInput(in)},
	}
}

func StartManualReviewEscalation(in ManualReviewEscalationInput) StartInput {
	return StartInput{
		Kind:       KindManualReviewEscalation,
		WorkflowID: workflowID("manual-review", in.PaymentID),
		Args:       []any{normalizeManualReviewEscalationInput(in)},
	}
}

func workflowID(prefix string, id uuid.UUID) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "workflow"
	}
	if id == uuid.Nil {
		return prefix + ":missing"
	}
	return fmt.Sprintf("%s:%s", prefix, id.String())
}

func normalizePaymentPendingTimeoutInput(in PaymentPendingTimeoutInput) PaymentPendingTimeoutInput {
	if in.ObservedAt.IsZero() {
		in.ObservedAt = time.Now().UTC()
	}
	in.ReasonCode = strings.TrimSpace(in.ReasonCode)
	in.TraceNote = strings.TrimSpace(in.TraceNote)
	return in
}

func normalizeVendFailureAfterPaymentSuccessInput(in VendFailureAfterPaymentSuccessInput) VendFailureAfterPaymentSuccessInput {
	if in.ObservedAt.IsZero() {
		in.ObservedAt = time.Now().UTC()
	}
	in.FailureReason = strings.TrimSpace(in.FailureReason)
	return in
}

func normalizeRefundOrchestrationInput(in RefundOrchestrationInput) RefundOrchestrationInput {
	if in.RequestedAt.IsZero() {
		in.RequestedAt = time.Now().UTC()
	}
	in.Reason = strings.TrimSpace(in.Reason)
	return in
}

func normalizeManualReviewEscalationInput(in ManualReviewEscalationInput) ManualReviewEscalationInput {
	if in.RequestedAt.IsZero() {
		in.RequestedAt = time.Now().UTC()
	}
	in.Reason = strings.TrimSpace(in.Reason)
	return in
}
