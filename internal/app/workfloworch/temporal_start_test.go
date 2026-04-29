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

func TestStartPaymentToVend_BuildsDeterministicWorkflowID(t *testing.T) {
	t.Parallel()
	paymentID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	got := StartPaymentToVend(PaymentToVendInput{
		OrganizationID: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"),
		OrderID:        uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"),
		PaymentID:      paymentID,
		SlotIndex:      2,
	})
	if got.Kind != KindPaymentToVend {
		t.Fatalf("kind=%q", got.Kind)
	}
	if got.WorkflowID != "payment-to-vend:"+paymentID.String() {
		t.Fatalf("workflow_id=%q", got.WorkflowID)
	}
	if len(got.Args) != 1 {
		t.Fatalf("args=%d", len(got.Args))
	}
}

func TestStartCommandAckWorkflow_BuildsDeterministicWorkflowID(t *testing.T) {
	t.Parallel()
	commandID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	got := StartCommandAckWorkflow(CommandAckWorkflowInput{
		OrganizationID: uuid.MustParse("88888888-8888-8888-8888-888888888888"),
		MachineID:      uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		CommandID:      commandID,
		Sequence:       42,
	})
	if got.Kind != KindCommandAck {
		t.Fatalf("kind=%q", got.Kind)
	}
	if got.WorkflowID != "command-ack:"+commandID.String() {
		t.Fatalf("workflow_id=%q", got.WorkflowID)
	}
}
