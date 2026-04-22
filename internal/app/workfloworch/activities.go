package workfloworch

import (
	"context"
	"fmt"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// LifecycleStore reads authoritative commerce state for workflow activities.
type LifecycleStore interface {
	GetOrderByID(ctx context.Context, orderID uuid.UUID) (domaincommerce.Order, error)
	GetVendSessionByOrderAndSlot(ctx context.Context, orderID uuid.UUID, slotIndex int32) (domaincommerce.VendSession, error)
	GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error)
	GetPaymentByID(ctx context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error)
}

// ActivityDeps wires workflow activities to existing stores/sinks.
type ActivityDeps struct {
	Lifecycle  LifecycleStore
	RefundSink domaincommerce.RefundReviewSink
}

// PaymentPendingTimeoutDecision is the activity output for an aged payment follow-up.
type PaymentPendingTimeoutDecision struct {
	OrganizationID uuid.UUID
	CurrentState   string
	ShouldEscalate bool
}

// VendFailureFollowUpDecision is the activity output for a failed vend after payment success.
type VendFailureFollowUpDecision struct {
	OrganizationID       uuid.UUID
	CurrentPaymentState  string
	CurrentVendState     string
	CurrentOrderStatus   string
	QueueRefundReview    bool
	EscalateManualReview bool
}

// TicketDispatchResult records the external ticket action taken by a workflow.
type TicketDispatchResult struct {
	Action string
	Reason string
}

// Activities hosts Temporal activity methods.
type Activities struct {
	lifecycle  LifecycleStore
	refundSink domaincommerce.RefundReviewSink
}

// NewActivities validates and returns the workflow activity set.
func NewActivities(deps ActivityDeps) (*Activities, error) {
	if deps.Lifecycle == nil {
		return nil, fmt.Errorf("workfloworch: lifecycle activity dependency is required")
	}
	if deps.RefundSink == nil {
		return nil, fmt.Errorf("workfloworch: refund sink activity dependency is required")
	}
	return &Activities{
		lifecycle:  deps.Lifecycle,
		refundSink: deps.RefundSink,
	}, nil
}

func (a *Activities) ResolvePaymentPendingTimeout(ctx context.Context, in PaymentPendingTimeoutInput) (PaymentPendingTimeoutDecision, error) {
	pay, err := a.lifecycle.GetPaymentByID(ctx, in.PaymentID)
	if err != nil {
		return PaymentPendingTimeoutDecision{}, err
	}
	order, err := a.lifecycle.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return PaymentPendingTimeoutDecision{}, err
	}
	if pay.OrderID != order.ID {
		return PaymentPendingTimeoutDecision{}, fmt.Errorf("workfloworch: payment %s does not belong to order %s", pay.ID, order.ID)
	}
	return PaymentPendingTimeoutDecision{
		OrganizationID: order.OrganizationID,
		CurrentState:   pay.State,
		ShouldEscalate: pay.State == "created" || pay.State == "authorized",
	}, nil
}

func (a *Activities) AssessVendFailureAfterPaymentSuccess(ctx context.Context, in VendFailureAfterPaymentSuccessInput) (VendFailureFollowUpDecision, error) {
	order, err := a.lifecycle.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return VendFailureFollowUpDecision{}, err
	}
	pay, err := a.lifecycle.GetPaymentByID(ctx, in.PaymentID)
	if err != nil {
		return VendFailureFollowUpDecision{}, err
	}
	vend, err := a.lifecycle.GetVendSessionByOrderAndSlot(ctx, in.OrderID, in.SlotIndex)
	if err != nil {
		return VendFailureFollowUpDecision{}, err
	}
	out := VendFailureFollowUpDecision{
		OrganizationID:      order.OrganizationID,
		CurrentPaymentState: pay.State,
		CurrentVendState:    vend.State,
		CurrentOrderStatus:  order.Status,
	}
	switch {
	case pay.State == "captured" && vend.State == "failed" && order.Status == "failed":
		out.QueueRefundReview = true
	case pay.State == "refunded":
		// No-op: compensation already completed.
	case vend.State == "failed" && order.Status == "failed":
		out.EscalateManualReview = true
	}
	return out, nil
}

func (a *Activities) EnqueueRefundReview(ctx context.Context, in RefundOrchestrationInput) (TicketDispatchResult, error) {
	if err := a.refundSink.EnqueueRefundReview(ctx, domaincommerce.RefundReviewTicket{
		OrganizationID: in.OrganizationID,
		OrderID:        in.OrderID,
		PaymentID:      in.PaymentID,
		Reason:         strings.TrimSpace(in.Reason),
	}); err != nil {
		return TicketDispatchResult{}, err
	}
	return TicketDispatchResult{
		Action: "refund_review_enqueued",
		Reason: strings.TrimSpace(in.Reason),
	}, nil
}

func (a *Activities) EnqueueManualReview(ctx context.Context, in ManualReviewEscalationInput) (TicketDispatchResult, error) {
	if err := a.refundSink.EnqueueRefundReview(ctx, domaincommerce.RefundReviewTicket{
		OrganizationID: in.OrganizationID,
		OrderID:        in.OrderID,
		PaymentID:      in.PaymentID,
		Reason:         strings.TrimSpace(in.Reason),
	}); err != nil {
		return TicketDispatchResult{}, err
	}
	return TicketDispatchResult{
		Action: "manual_review_enqueued",
		Reason: strings.TrimSpace(in.Reason),
	}, nil
}
