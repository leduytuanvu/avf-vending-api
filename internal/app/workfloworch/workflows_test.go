package workfloworch

import (
	"testing"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"go.temporal.io/sdk/testsuite"
)

func TestPaymentToVendWorkflow_TimeoutQueuesManualReview(t *testing.T) {
	t.Parallel()
	orderID := uuid.MustParse("12121212-1212-1212-1212-121212121212")
	paymentID := uuid.MustParse("34343434-3434-3434-3434-343434343434")
	orgID := uuid.MustParse("56565656-5656-5656-5656-565656565656")
	sink := &stubRefundSink{}
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	if err := RegisterAll(env, ActivityDeps{
		Lifecycle: stubLifecycleStore{
			order:   domaincommerce.Order{ID: orderID, OrganizationID: orgID, Status: "paid"},
			payment: domaincommerce.Payment{ID: paymentID, OrderID: orderID, State: "captured"},
			vend:    domaincommerce.VendSession{OrderID: orderID, State: "in_progress"},
		},
		RefundSink: sink,
	}); err != nil {
		t.Fatal(err)
	}

	env.ExecuteWorkflow(WorkflowNamePaymentToVend, PaymentToVendInput{
		OrganizationID: orgID,
		OrderID:        orderID,
		PaymentID:      paymentID,
		SlotIndex:      1,
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if len(sink.tickets) != 1 {
		t.Fatalf("tickets=%d", len(sink.tickets))
	}
	if sink.tickets[0].Reason != "manual_review:vend_result_timeout" {
		t.Fatalf("ticket reason=%q", sink.tickets[0].Reason)
	}
}
