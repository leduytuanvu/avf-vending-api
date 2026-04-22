package workfloworch

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestWorkflowSpecForStart_UnknownKindNotImplemented(t *testing.T) {
	t.Parallel()
	_, err := workflowSpecForStart(StartInput{
		Kind:       Kind("unknown_kind"),
		WorkflowID: "wf-1",
	})
	if !errors.Is(err, ErrWorkflowNotImplemented) {
		t.Fatalf("got %v want ErrWorkflowNotImplemented", err)
	}
}

func TestStartRefundOrchestration_BuildsDeterministicWorkflowID(t *testing.T) {
	t.Parallel()
	paymentID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	got := StartRefundOrchestration(RefundOrchestrationInput{
		OrganizationID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		OrderID:        uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		PaymentID:      paymentID,
		Reason:         "captured_payment_failed_order",
	})
	if got.Kind != KindRefundOrchestration {
		t.Fatalf("kind=%q", got.Kind)
	}
	if got.WorkflowID != "refund-orchestration:"+paymentID.String() {
		t.Fatalf("workflow_id=%q", got.WorkflowID)
	}
	if len(got.Args) != 1 {
		t.Fatalf("args=%d", len(got.Args))
	}
}
