package workfloworch

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	WorkflowNamePaymentToVend = "workflow.payment_to_vend"
	WorkflowNameRefund        = "workflow.refund"
	WorkflowNameCommandAck    = "workflow.command_ack"

	WorkflowNamePaymentPendingTimeoutFollowUp  = "workflow.payment_pending_timeout_follow_up"
	WorkflowNameVendFailureAfterPaymentSuccess = "workflow.vend_failure_after_payment_success"
	WorkflowNameRefundOrchestration            = "workflow.refund_orchestration"
	WorkflowNameManualReviewEscalation         = "workflow.manual_review_escalation"

	ActivityNameEnsureVendStartedForPaidOrder = "activity.ensure_vend_started_for_paid_order"
	ActivityNameEvaluatePaymentToVend         = "activity.evaluate_payment_to_vend"
	ActivityNameRequestProviderRefund         = "activity.request_provider_refund"
	ActivityNameAssessCommandAck              = "activity.assess_command_ack"
	ActivityNameResolvePaymentPendingTimeout  = "activity.resolve_payment_pending_timeout"
	ActivityNameAssessVendFailureAfterPayment = "activity.assess_vend_failure_after_payment"
	ActivityNameEnqueueRefundReview           = "activity.enqueue_refund_review"
	ActivityNameEnqueueManualReview           = "activity.enqueue_manual_review"
)

// WorkflowOutcome is a stable result envelope for workflow tests and logs.
type WorkflowOutcome struct {
	Kind   Kind
	Action string
	Detail string
}

func PaymentToVendWorkflow(ctx workflow.Context, in PaymentToVendInput) (WorkflowOutcome, error) {
	in = normalizePaymentToVendInput(in)
	var started VendStartResult
	if err := workflow.ExecuteActivity(writeActivityContext(ctx), ActivityNameEnsureVendStartedForPaidOrder, in).Get(ctx, &started); err != nil {
		return WorkflowOutcome{Kind: KindPaymentToVend}, err
	}
	if in.VendResultTimeout > 0 {
		if err := workflow.Sleep(ctx, in.VendResultTimeout); err != nil {
			return WorkflowOutcome{Kind: KindPaymentToVend}, err
		}
	}
	var decision PaymentToVendDecision
	if err := workflow.ExecuteActivity(readActivityContext(ctx), ActivityNameEvaluatePaymentToVend, in).Get(ctx, &decision); err != nil {
		return WorkflowOutcome{Kind: KindPaymentToVend}, err
	}
	if decision.QueueRefundReview {
		var queued TicketDispatchResult
		err := workflow.ExecuteActivity(
			externalDispatchActivityContext(ctx),
			ActivityNameEnqueueRefundReview,
			RefundOrchestrationInput{
				OrganizationID: decision.OrganizationID,
				OrderID:        in.OrderID,
				PaymentID:      in.PaymentID,
				Reason:         decision.Reason,
				RequestedAt:    workflow.Now(ctx),
			},
		).Get(ctx, &queued)
		if err != nil {
			return WorkflowOutcome{Kind: KindPaymentToVend}, err
		}
		return WorkflowOutcome{Kind: KindPaymentToVend, Action: queued.Action, Detail: decision.Detail}, nil
	}
	if decision.EscalateManualReview {
		var queued TicketDispatchResult
		err := workflow.ExecuteActivity(
			externalDispatchActivityContext(ctx),
			ActivityNameEnqueueManualReview,
			ManualReviewEscalationInput{
				OrganizationID: decision.OrganizationID,
				OrderID:        in.OrderID,
				PaymentID:      in.PaymentID,
				Reason:         decision.Reason,
				RequestedAt:    workflow.Now(ctx),
			},
		).Get(ctx, &queued)
		if err != nil {
			return WorkflowOutcome{Kind: KindPaymentToVend}, err
		}
		return WorkflowOutcome{Kind: KindPaymentToVend, Action: queued.Action, Detail: decision.Detail}, nil
	}
	action := "noop"
	if started.Started {
		action = "vend_started"
	}
	if decision.Action != "" {
		action = decision.Action
	}
	return WorkflowOutcome{Kind: KindPaymentToVend, Action: action, Detail: decision.Detail}, nil
}

func RefundWorkflow(ctx workflow.Context, in RefundWorkflowInput) (WorkflowOutcome, error) {
	in = normalizeRefundWorkflowInput(in)
	var decision ProviderRefundDecision
	if err := workflow.ExecuteActivity(externalDispatchActivityContext(ctx), ActivityNameRequestProviderRefund, in).Get(ctx, &decision); err != nil {
		return WorkflowOutcome{Kind: KindRefund}, err
	}
	if decision.QueueRefundReview {
		var queued TicketDispatchResult
		err := workflow.ExecuteActivity(
			externalDispatchActivityContext(ctx),
			ActivityNameEnqueueRefundReview,
			RefundOrchestrationInput{
				OrganizationID: in.OrganizationID,
				OrderID:        in.OrderID,
				PaymentID:      in.PaymentID,
				Reason:         decision.Reason,
				RequestedAt:    in.RequestedAt,
			},
		).Get(ctx, &queued)
		if err != nil {
			return WorkflowOutcome{Kind: KindRefund}, err
		}
		return WorkflowOutcome{Kind: KindRefund, Action: queued.Action, Detail: decision.Detail}, nil
	}
	return WorkflowOutcome{Kind: KindRefund, Action: decision.Action, Detail: decision.Detail}, nil
}

func CommandAckWorkflow(ctx workflow.Context, in CommandAckWorkflowInput) (WorkflowOutcome, error) {
	in = normalizeCommandAckWorkflowInput(in)
	if in.AckTimeout > 0 {
		if err := workflow.Sleep(ctx, in.AckTimeout); err != nil {
			return WorkflowOutcome{Kind: KindCommandAck}, err
		}
	}
	var decision CommandAckDecision
	if err := workflow.ExecuteActivity(readActivityContext(ctx), ActivityNameAssessCommandAck, in).Get(ctx, &decision); err != nil {
		return WorkflowOutcome{Kind: KindCommandAck}, err
	}
	if decision.EscalateManualReview {
		var queued TicketDispatchResult
		err := workflow.ExecuteActivity(
			externalDispatchActivityContext(ctx),
			ActivityNameEnqueueManualReview,
			ManualReviewEscalationInput{
				OrganizationID: in.OrganizationID,
				Reason:         decision.Reason,
				RequestedAt:    workflow.Now(ctx),
			},
		).Get(ctx, &queued)
		if err != nil {
			return WorkflowOutcome{Kind: KindCommandAck}, err
		}
		return WorkflowOutcome{Kind: KindCommandAck, Action: queued.Action, Detail: decision.Detail}, nil
	}
	return WorkflowOutcome{Kind: KindCommandAck, Action: decision.Action, Detail: decision.Detail}, nil
}

func PaymentPendingTimeoutFollowUpWorkflow(ctx workflow.Context, in PaymentPendingTimeoutInput) (WorkflowOutcome, error) {
	var decision PaymentPendingTimeoutDecision
	if err := workflow.ExecuteActivity(readActivityContext(ctx), ActivityNameResolvePaymentPendingTimeout, normalizePaymentPendingTimeoutInput(in)).Get(ctx, &decision); err != nil {
		return WorkflowOutcome{Kind: KindPaymentPendingTimeoutFollowUp}, err
	}
	if !decision.ShouldEscalate {
		return WorkflowOutcome{
			Kind:   KindPaymentPendingTimeoutFollowUp,
			Action: "noop",
			Detail: decision.CurrentState,
		}, nil
	}
	var queued TicketDispatchResult
	err := workflow.ExecuteActivity(
		externalDispatchActivityContext(ctx),
		ActivityNameEnqueueManualReview,
		ManualReviewEscalationInput{
			OrganizationID: decision.OrganizationID,
			OrderID:        in.OrderID,
			PaymentID:      in.PaymentID,
			Reason:         "manual_review:payment_pending_timeout",
			RequestedAt:    in.ObservedAt,
		},
	).Get(ctx, &queued)
	if err != nil {
		return WorkflowOutcome{Kind: KindPaymentPendingTimeoutFollowUp}, err
	}
	return WorkflowOutcome{
		Kind:   KindPaymentPendingTimeoutFollowUp,
		Action: queued.Action,
		Detail: decision.CurrentState,
	}, nil
}

func VendFailureAfterPaymentSuccessWorkflow(ctx workflow.Context, in VendFailureAfterPaymentSuccessInput) (WorkflowOutcome, error) {
	var decision VendFailureFollowUpDecision
	if err := workflow.ExecuteActivity(readActivityContext(ctx), ActivityNameAssessVendFailureAfterPayment, normalizeVendFailureAfterPaymentSuccessInput(in)).Get(ctx, &decision); err != nil {
		return WorkflowOutcome{Kind: KindVendFailureAfterPaymentSuccess}, err
	}
	switch {
	case decision.QueueRefundReview:
		var queued TicketDispatchResult
		err := workflow.ExecuteActivity(
			externalDispatchActivityContext(ctx),
			ActivityNameEnqueueRefundReview,
			RefundOrchestrationInput{
				OrganizationID: decision.OrganizationID,
				OrderID:        in.OrderID,
				PaymentID:      in.PaymentID,
				Reason:         "captured_payment_failed_order",
				RequestedAt:    in.ObservedAt,
			},
		).Get(ctx, &queued)
		if err != nil {
			return WorkflowOutcome{Kind: KindVendFailureAfterPaymentSuccess}, err
		}
		return WorkflowOutcome{
			Kind:   KindVendFailureAfterPaymentSuccess,
			Action: queued.Action,
			Detail: decision.CurrentPaymentState,
		}, nil
	case decision.EscalateManualReview:
		var queued TicketDispatchResult
		err := workflow.ExecuteActivity(
			externalDispatchActivityContext(ctx),
			ActivityNameEnqueueManualReview,
			ManualReviewEscalationInput{
				OrganizationID: decision.OrganizationID,
				OrderID:        in.OrderID,
				PaymentID:      in.PaymentID,
				Reason:         "manual_review:vend_failed_after_payment_success",
				RequestedAt:    in.ObservedAt,
			},
		).Get(ctx, &queued)
		if err != nil {
			return WorkflowOutcome{Kind: KindVendFailureAfterPaymentSuccess}, err
		}
		return WorkflowOutcome{
			Kind:   KindVendFailureAfterPaymentSuccess,
			Action: queued.Action,
			Detail: decision.CurrentPaymentState,
		}, nil
	default:
		return WorkflowOutcome{
			Kind:   KindVendFailureAfterPaymentSuccess,
			Action: "noop",
			Detail: fmt.Sprintf("payment=%s vend=%s order=%s", decision.CurrentPaymentState, decision.CurrentVendState, decision.CurrentOrderStatus),
		}, nil
	}
}

func RefundOrchestrationWorkflow(ctx workflow.Context, in RefundOrchestrationInput) (WorkflowOutcome, error) {
	var queued TicketDispatchResult
	if err := workflow.ExecuteActivity(externalDispatchActivityContext(ctx), ActivityNameEnqueueRefundReview, normalizeRefundOrchestrationInput(in)).Get(ctx, &queued); err != nil {
		return WorkflowOutcome{Kind: KindRefundOrchestration}, err
	}
	return WorkflowOutcome{
		Kind:   KindRefundOrchestration,
		Action: queued.Action,
		Detail: queued.Reason,
	}, nil
}

func ManualReviewEscalationWorkflow(ctx workflow.Context, in ManualReviewEscalationInput) (WorkflowOutcome, error) {
	var queued TicketDispatchResult
	if err := workflow.ExecuteActivity(externalDispatchActivityContext(ctx), ActivityNameEnqueueManualReview, normalizeManualReviewEscalationInput(in)).Get(ctx, &queued); err != nil {
		return WorkflowOutcome{Kind: KindManualReviewEscalation}, err
	}
	return WorkflowOutcome{
		Kind:   KindManualReviewEscalation,
		Action: queued.Action,
		Detail: queued.Reason,
	}, nil
}

func readActivityContext(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    5 * time.Second,
			MaximumAttempts:    3,
		},
	})
}

func writeActivityContext(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    5 * time.Second,
			MaximumAttempts:    3,
		},
	})
}

func externalDispatchActivityContext(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})
}
