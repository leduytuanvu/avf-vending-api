package workfloworch

import (
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// Registrar is the subset of worker registration methods used by RegisterAll.
type Registrar interface {
	RegisterWorkflowWithOptions(w any, options workflow.RegisterOptions)
	RegisterActivityWithOptions(a any, options activity.RegisterOptions)
}

// RegisterAll registers all workflows and activities on the provided Temporal worker.
func RegisterAll(reg Registrar, deps ActivityDeps) error {
	if reg == nil {
		return fmt.Errorf("workfloworch: nil registrar")
	}
	acts, err := NewActivities(deps)
	if err != nil {
		return err
	}
	reg.RegisterWorkflowWithOptions(PaymentToVendWorkflow, workflow.RegisterOptions{Name: WorkflowNamePaymentToVend})
	reg.RegisterWorkflowWithOptions(RefundWorkflow, workflow.RegisterOptions{Name: WorkflowNameRefund})
	reg.RegisterWorkflowWithOptions(CommandAckWorkflow, workflow.RegisterOptions{Name: WorkflowNameCommandAck})
	reg.RegisterWorkflowWithOptions(PaymentPendingTimeoutFollowUpWorkflow, workflow.RegisterOptions{Name: WorkflowNamePaymentPendingTimeoutFollowUp})
	reg.RegisterWorkflowWithOptions(VendFailureAfterPaymentSuccessWorkflow, workflow.RegisterOptions{Name: WorkflowNameVendFailureAfterPaymentSuccess})
	reg.RegisterWorkflowWithOptions(RefundOrchestrationWorkflow, workflow.RegisterOptions{Name: WorkflowNameRefundOrchestration})
	reg.RegisterWorkflowWithOptions(ManualReviewEscalationWorkflow, workflow.RegisterOptions{Name: WorkflowNameManualReviewEscalation})

	reg.RegisterActivityWithOptions(acts.EnsureVendStartedForPaidOrder, activity.RegisterOptions{Name: ActivityNameEnsureVendStartedForPaidOrder})
	reg.RegisterActivityWithOptions(acts.EvaluatePaymentToVend, activity.RegisterOptions{Name: ActivityNameEvaluatePaymentToVend})
	reg.RegisterActivityWithOptions(acts.RequestProviderRefund, activity.RegisterOptions{Name: ActivityNameRequestProviderRefund})
	reg.RegisterActivityWithOptions(acts.AssessCommandAck, activity.RegisterOptions{Name: ActivityNameAssessCommandAck})
	reg.RegisterActivityWithOptions(acts.ResolvePaymentPendingTimeout, activity.RegisterOptions{Name: ActivityNameResolvePaymentPendingTimeout})
	reg.RegisterActivityWithOptions(acts.AssessVendFailureAfterPaymentSuccess, activity.RegisterOptions{Name: ActivityNameAssessVendFailureAfterPayment})
	reg.RegisterActivityWithOptions(acts.EnqueueRefundReview, activity.RegisterOptions{Name: ActivityNameEnqueueRefundReview})
	reg.RegisterActivityWithOptions(acts.EnqueueManualReview, activity.RegisterOptions{Name: ActivityNameEnqueueManualReview})
	return nil
}
