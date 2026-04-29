package commerce

import (
	"context"
	"errors"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

type stubLifecycleForWorkflow struct {
	order   domaincommerce.Order
	vend    domaincommerce.VendSession
	payment domaincommerce.Payment
}

func (s *stubLifecycleForWorkflow) GetOrderByID(context.Context, uuid.UUID) (domaincommerce.Order, error) {
	return s.order, nil
}

func (s *stubLifecycleForWorkflow) UpdateOrderStatus(context.Context, uuid.UUID, uuid.UUID, string) (domaincommerce.Order, error) {
	s.order.Status = "failed"
	return s.order, nil
}

func (s *stubLifecycleForWorkflow) GetVendSessionByOrderAndSlot(context.Context, uuid.UUID, int32) (domaincommerce.VendSession, error) {
	return s.vend, nil
}

func (s *stubLifecycleForWorkflow) UpdateVendSessionState(context.Context, UpdateVendSessionParams) (domaincommerce.VendSession, error) {
	s.vend.State = "failed"
	return s.vend, nil
}

func (s *stubLifecycleForWorkflow) GetLatestPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

func (s *stubLifecycleForWorkflow) GetPaymentByID(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

func (s *stubLifecycleForWorkflow) InsertPaymentAttempt(context.Context, InsertPaymentAttemptParams) (PaymentAttemptView, error) {
	return PaymentAttemptView{}, nil
}

func (s *stubLifecycleForWorkflow) InsertRefundRow(context.Context, InsertRefundRowInput) (RefundRowView, error) {
	return RefundRowView{}, errors.New("not implemented")
}

func (s *stubLifecycleForWorkflow) ListRefundsForOrder(context.Context, uuid.UUID) ([]RefundRowView, error) {
	return nil, nil
}

func (s *stubLifecycleForWorkflow) GetRefundByIDForOrder(context.Context, uuid.UUID, uuid.UUID) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (s *stubLifecycleForWorkflow) GetRefundByOrderIdempotency(context.Context, uuid.UUID, string) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (s *stubLifecycleForWorkflow) SumNonFailedRefundAmountForPayment(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func (s *stubLifecycleForWorkflow) FulfillSuccessfulVendAtomically(context.Context, FulfillSuccessfulVendInput) (FulfillSuccessfulVendResult, error) {
	return FulfillSuccessfulVendResult{}, ErrNotConfigured
}

func (s *stubLifecycleForWorkflow) FulfillFailedVendAtomically(_ context.Context, _ FulfillFailedVendInput) (FulfillFailedVendResult, error) {
	s.vend.State = "failed"
	s.order.Status = "failed"
	return FulfillFailedVendResult{
		Order:  s.order,
		Vend:   s.vend,
		Replay: false,
	}, nil
}

type stubOrderVendWorkflow struct{}

func (stubOrderVendWorkflow) CreateOrderWithVendSession(context.Context, domaincommerce.CreateOrderVendInput) (domaincommerce.CreateOrderVendResult, error) {
	return domaincommerce.CreateOrderVendResult{}, nil
}

func (stubOrderVendWorkflow) TryReplayCreateOrderWithVend(context.Context, uuid.UUID, string) (domaincommerce.CreateOrderVendResult, bool, error) {
	return domaincommerce.CreateOrderVendResult{}, false, nil
}

type stubSaleLineResolver struct{}

func (stubSaleLineResolver) ResolveSaleLine(context.Context, ResolveSaleLineInput) (ResolvedSaleLine, error) {
	return ResolvedSaleLine{}, nil
}

func (stubSaleLineResolver) LookupSlotDisplay(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int32) (ResolvedSaleLine, error) {
	return ResolvedSaleLine{}, nil
}

func TestFinalizeOrderAfterVend_SchedulesVendFailureWorkflow(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orderID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	vendID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	paymentID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	life := &stubLifecycleForWorkflow{
		order: domaincommerce.Order{ID: orderID, OrganizationID: orgID, Status: "vending"},
		vend: domaincommerce.VendSession{
			ID:        vendID,
			OrderID:   orderID,
			SlotIndex: 1,
			State:     "in_progress",
		},
		payment: domaincommerce.Payment{
			ID:      paymentID,
			OrderID: orderID,
			State:   "captured",
		},
	}
	wf := &stubWorkflowBoundary{}
	svc := NewService(Deps{
		OrderVend:                   stubOrderVendWorkflow{},
		Lifecycle:                   life,
		SaleLines:                   stubSaleLineResolver{},
		WorkflowOrchestration:       wf,
		ScheduleVendFailureFollowUp: true,
	})
	_, err := svc.FinalizeOrderAfterVend(context.Background(), FinalizeAfterVendInput{
		OrganizationID:    orgID,
		OrderID:           orderID,
		SlotIndex:         1,
		TerminalVendState: "failed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wf.starts) != 1 || wf.starts[0].Kind != workfloworch.KindVendFailureAfterPaymentSuccess {
		t.Fatalf("starts=%+v", wf.starts)
	}
}

type stubWorkflowBoundary struct {
	starts []workfloworch.StartInput
}

func (s *stubWorkflowBoundary) Enabled() bool { return true }

func (s *stubWorkflowBoundary) Start(_ context.Context, in workfloworch.StartInput) error {
	s.starts = append(s.starts, in)
	return nil
}

func (s *stubWorkflowBoundary) Close() error { return nil }
