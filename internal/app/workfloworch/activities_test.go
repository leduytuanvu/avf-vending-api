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

type stubRefundProvider struct {
	calls []ProviderRefundRequest
}

func (s *stubRefundProvider) RefundPayment(_ context.Context, in ProviderRefundRequest) error {
	s.calls = append(s.calls, in)
	return nil
}

type stubCommandAckReader struct {
	status string
}

func (s stubCommandAckReader) LatestCommandAttempt(context.Context, uuid.UUID) (CommandAttemptSnapshot, error) {
	return CommandAttemptSnapshot{Status: s.status}, nil
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

func TestActivities_EvaluatePaymentToVendTimeoutEscalatesReview(t *testing.T) {
	t.Parallel()
	orderID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	paymentID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	orgID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	acts, err := NewActivities(ActivityDeps{
		Lifecycle: stubLifecycleStore{
			order:   domaincommerce.Order{ID: orderID, OrganizationID: orgID, Status: "paid"},
			payment: domaincommerce.Payment{ID: paymentID, OrderID: orderID, State: "captured"},
			vend:    domaincommerce.VendSession{OrderID: orderID, State: "in_progress"},
		},
		RefundSink: &stubRefundSink{},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := acts.EvaluatePaymentToVend(context.Background(), PaymentToVendInput{
		OrganizationID: orgID,
		OrderID:        orderID,
		PaymentID:      paymentID,
		SlotIndex:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.EscalateManualReview || got.Reason != "manual_review:vend_result_timeout" {
		t.Fatalf("unexpected decision: %+v", got)
	}
}

func TestActivities_RequestProviderRefundUsesIdempotencyKey(t *testing.T) {
	t.Parallel()
	refunds := &stubRefundProvider{}
	acts, err := NewActivities(ActivityDeps{
		Lifecycle:  stubLifecycleStore{},
		RefundSink: &stubRefundSink{},
		Refunds:    refunds,
	})
	if err != nil {
		t.Fatal(err)
	}
	in := RefundWorkflowInput{
		OrganizationID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		OrderID:        uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		PaymentID:      uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		RefundID:       uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
		AmountMinor:    1200,
		Currency:       "USD",
		IdempotencyKey: "refund-idem-1",
	}
	for i := 0; i < 2; i++ {
		got, err := acts.RequestProviderRefund(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		if got.Action != "provider_refund_requested" || got.QueueRefundReview {
			t.Fatalf("unexpected decision: %+v", got)
		}
	}
	if len(refunds.calls) != 2 {
		t.Fatalf("provider calls=%d", len(refunds.calls))
	}
	for _, call := range refunds.calls {
		if call.IdempotencyKey != "refund-idem-1" {
			t.Fatalf("idempotency key not preserved: %+v", call)
		}
	}
}

func TestActivities_AssessCommandAckTimeoutEscalatesReview(t *testing.T) {
	t.Parallel()
	acts, err := NewActivities(ActivityDeps{
		Lifecycle:   stubLifecycleStore{},
		RefundSink:  &stubRefundSink{},
		CommandAcks: stubCommandAckReader{status: "ack_timeout"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := acts.AssessCommandAck(context.Background(), CommandAckWorkflowInput{
		OrganizationID: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"),
		MachineID:      uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"),
		CommandID:      uuid.MustParse("12121212-1212-1212-1212-121212121212"),
		Sequence:       12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.EscalateManualReview || got.Reason != "manual_review:command_ack_timeout" {
		t.Fatalf("unexpected decision: %+v", got)
	}
}
