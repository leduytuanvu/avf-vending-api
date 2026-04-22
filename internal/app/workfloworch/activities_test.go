package workfloworch

import (
	"context"
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

type stubLifecycleStore struct {
	order   domaincommerce.Order
	payment domaincommerce.Payment
	vend    domaincommerce.VendSession
}

func (s stubLifecycleStore) GetOrderByID(context.Context, uuid.UUID) (domaincommerce.Order, error) {
	return s.order, nil
}

func (s stubLifecycleStore) GetVendSessionByOrderAndSlot(context.Context, uuid.UUID, int32) (domaincommerce.VendSession, error) {
	return s.vend, nil
}

func (s stubLifecycleStore) GetLatestPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

func (s stubLifecycleStore) GetPaymentByID(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

type stubRefundSink struct {
	tickets []domaincommerce.RefundReviewTicket
}

func (s *stubRefundSink) EnqueueRefundReview(_ context.Context, ticket domaincommerce.RefundReviewTicket) error {
	s.tickets = append(s.tickets, ticket)
	return nil
}

func TestActivities_ResolvePaymentPendingTimeout(t *testing.T) {
	t.Parallel()
	orderID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	paymentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	orgID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	acts, err := NewActivities(ActivityDeps{
		Lifecycle: stubLifecycleStore{
			order: domaincommerce.Order{ID: orderID, OrganizationID: orgID},
			payment: domaincommerce.Payment{
				ID:      paymentID,
				OrderID: orderID,
				State:   "authorized",
			},
		},
		RefundSink: &stubRefundSink{},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := acts.ResolvePaymentPendingTimeout(context.Background(), PaymentPendingTimeoutInput{
		PaymentID:  paymentID,
		OrderID:    orderID,
		ObservedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.ShouldEscalate || got.OrganizationID != orgID || got.CurrentState != "authorized" {
		t.Fatalf("unexpected decision: %+v", got)
	}
}

func TestActivities_AssessVendFailureAfterPaymentSuccess(t *testing.T) {
	t.Parallel()
	orderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	paymentID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	orgID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	acts, err := NewActivities(ActivityDeps{
		Lifecycle: stubLifecycleStore{
			order:   domaincommerce.Order{ID: orderID, OrganizationID: orgID, Status: "failed"},
			payment: domaincommerce.Payment{ID: paymentID, OrderID: orderID, State: "captured"},
			vend:    domaincommerce.VendSession{OrderID: orderID, State: "failed"},
		},
		RefundSink: &stubRefundSink{},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := acts.AssessVendFailureAfterPaymentSuccess(context.Background(), VendFailureAfterPaymentSuccessInput{
		OrganizationID: orgID,
		OrderID:        orderID,
		PaymentID:      paymentID,
		SlotIndex:      1,
		ObservedAt:     time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.QueueRefundReview || got.OrganizationID != orgID {
		t.Fatalf("unexpected decision: %+v", got)
	}
}
