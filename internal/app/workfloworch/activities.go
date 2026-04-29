package workfloworch

import (
	"context"
	"fmt"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/google/uuid"
)

// LifecycleStore reads authoritative commerce state for workflow activities.
type LifecycleStore interface {
	GetOrderByID(ctx context.Context, orderID uuid.UUID) (domaincommerce.Order, error)
	GetVendSessionByOrderAndSlot(ctx context.Context, orderID uuid.UUID, slotIndex int32) (domaincommerce.VendSession, error)
	GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error)
	GetPaymentByID(ctx context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error)
}

type VendStarter interface {
	EnsureVendInProgressForPaidOrder(ctx context.Context, organizationID, orderID uuid.UUID, slotIndex int32) error
}

type ProviderRefundRequest struct {
	OrganizationID uuid.UUID
	PaymentID      uuid.UUID
	RefundID       uuid.UUID
	AmountMinor    int64
	Currency       string
	IdempotencyKey string
}

type RefundProvider interface {
	RefundPayment(ctx context.Context, in ProviderRefundRequest) error
}

type CommandAttemptSnapshot struct {
	CommandID uuid.UUID
	Status    string
}

type CommandAckReader interface {
	LatestCommandAttempt(ctx context.Context, commandID uuid.UUID) (CommandAttemptSnapshot, error)
}

// ActivityDeps wires workflow activities to existing stores/sinks.
type ActivityDeps struct {
	Lifecycle   LifecycleStore
	RefundSink  domaincommerce.RefundReviewSink
	VendStarter VendStarter
	Refunds     RefundProvider
	CommandAcks CommandAckReader
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

type VendStartResult struct {
	Started bool
	Action  string
	Detail  string
}

type PaymentToVendDecision struct {
	OrganizationID       uuid.UUID
	Action               string
	Detail               string
	Reason               string
	QueueRefundReview    bool
	EscalateManualReview bool
	CurrentPaymentState  string
	CurrentVendState     string
	CurrentOrderStatus   string
}

type ProviderRefundDecision struct {
	Action            string
	Detail            string
	Reason            string
	QueueRefundReview bool
}

type CommandAckDecision struct {
	Action               string
	Detail               string
	Reason               string
	EscalateManualReview bool
	CurrentAttemptStatus string
}

// Activities hosts Temporal activity methods.
type Activities struct {
	lifecycle   LifecycleStore
	refundSink  domaincommerce.RefundReviewSink
	vendStarter VendStarter
	refunds     RefundProvider
	commandAcks CommandAckReader
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
		lifecycle:   deps.Lifecycle,
		refundSink:  deps.RefundSink,
		vendStarter: deps.VendStarter,
		refunds:     deps.Refunds,
		commandAcks: deps.CommandAcks,
	}, nil
}

func (a *Activities) EnsureVendStartedForPaidOrder(ctx context.Context, in PaymentToVendInput) (VendStartResult, error) {
	if a.vendStarter == nil {
		return VendStartResult{
			Action: "vend_start_not_configured",
			Detail: "no vend starter is wired for Temporal worker activities",
		}, nil
	}
	if err := a.vendStarter.EnsureVendInProgressForPaidOrder(ctx, in.OrganizationID, in.OrderID, in.SlotIndex); err != nil {
		return VendStartResult{}, err
	}
	return VendStartResult{
		Started: true,
		Action:  "vend_start_ensured",
		Detail:  "vend state is in_progress when payment/order state allows it",
	}, nil
}

func (a *Activities) EvaluatePaymentToVend(ctx context.Context, in PaymentToVendInput) (PaymentToVendDecision, error) {
	order, err := a.lifecycle.GetOrderByID(ctx, in.OrderID)
	if err != nil {
		return PaymentToVendDecision{}, err
	}
	pay, err := a.lifecycle.GetPaymentByID(ctx, in.PaymentID)
	if err != nil {
		return PaymentToVendDecision{}, err
	}
	if pay.OrderID != order.ID {
		return PaymentToVendDecision{}, fmt.Errorf("workfloworch: payment %s does not belong to order %s", pay.ID, order.ID)
	}
	vend, err := a.lifecycle.GetVendSessionByOrderAndSlot(ctx, in.OrderID, in.SlotIndex)
	if err != nil {
		return PaymentToVendDecision{}, err
	}
	out := PaymentToVendDecision{
		OrganizationID:      order.OrganizationID,
		CurrentPaymentState: pay.State,
		CurrentVendState:    vend.State,
		CurrentOrderStatus:  order.Status,
		Detail:              fmt.Sprintf("payment=%s vend=%s order=%s", pay.State, vend.State, order.Status),
	}
	switch {
	case vend.State == "success" || order.Status == "completed":
		out.Action = "vend_completed"
	case (pay.State == "captured" || pay.State == "partially_refunded") && vend.State == "failed":
		out.Action = "refund_review_required"
		out.Reason = "captured_payment_failed_order"
		out.QueueRefundReview = true
	case vend.State == "pending" || vend.State == "in_progress":
		out.Action = "manual_review_required"
		out.Reason = "manual_review:vend_result_timeout"
		out.EscalateManualReview = true
	default:
		out.Action = "noop"
	}
	return out, nil
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

func (a *Activities) RequestProviderRefund(ctx context.Context, in RefundWorkflowInput) (ProviderRefundDecision, error) {
	if a.refunds == nil {
		return ProviderRefundDecision{
			Action:            "provider_refund_not_configured",
			Detail:            "refund provider is not wired; refund review was requested instead",
			Reason:            "refund_provider_not_configured",
			QueueRefundReview: true,
		}, nil
	}
	err := a.refunds.RefundPayment(ctx, ProviderRefundRequest{
		OrganizationID: in.OrganizationID,
		PaymentID:      in.PaymentID,
		RefundID:       in.RefundID,
		AmountMinor:    in.AmountMinor,
		Currency:       in.Currency,
		IdempotencyKey: in.IdempotencyKey,
	})
	if err != nil {
		productionmetrics.RecordRefundFailed("provider_refund_failed")
		return ProviderRefundDecision{
			Action:            "provider_refund_failed",
			Detail:            err.Error(),
			Reason:            "provider_refund_failed",
			QueueRefundReview: true,
		}, nil
	}
	return ProviderRefundDecision{
		Action: "provider_refund_requested",
		Detail: "provider accepted the idempotent refund request",
	}, nil
}

func (a *Activities) AssessCommandAck(ctx context.Context, in CommandAckWorkflowInput) (CommandAckDecision, error) {
	if a.commandAcks == nil {
		return CommandAckDecision{
			Action:               "manual_review_required",
			Detail:               fmt.Sprintf("command=%s machine=%s seq=%d has no ACK reader wired", in.CommandID, in.MachineID, in.Sequence),
			Reason:               "manual_review:command_ack_reader_not_configured",
			EscalateManualReview: true,
		}, nil
	}
	snap, err := a.commandAcks.LatestCommandAttempt(ctx, in.CommandID)
	if err != nil {
		return CommandAckDecision{}, err
	}
	status := strings.ToLower(strings.TrimSpace(snap.Status))
	out := CommandAckDecision{
		Action:               "noop",
		Detail:               fmt.Sprintf("command=%s attempt_status=%s", in.CommandID, status),
		CurrentAttemptStatus: status,
	}
	switch status {
	case "completed", "acked", "delivered", "executed":
		out.Action = "ack_observed"
	case "failed", "nack", "nacked", "ack_timeout", "expired", "late", "sent", "pending", "":
		out.Action = "manual_review_required"
		out.Reason = "manual_review:command_ack_timeout"
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
