package commerce

import (
	"context"
	"testing"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type orderStatusLifecycleStub struct {
	order    domaincommerce.Order
	vend     domaincommerce.VendSession
	payment  domaincommerce.Payment
	slotMiss bool
}

func (s *orderStatusLifecycleStub) GetOrderByID(context.Context, uuid.UUID) (domaincommerce.Order, error) {
	return s.order, nil
}

func (s *orderStatusLifecycleStub) UpdateOrderStatus(context.Context, uuid.UUID, uuid.UUID, string) (domaincommerce.Order, error) {
	return domaincommerce.Order{}, ErrNotConfigured
}

func (s *orderStatusLifecycleStub) GetVendSessionByOrderAndSlot(context.Context, uuid.UUID, int32) (domaincommerce.VendSession, error) {
	if s.slotMiss {
		return domaincommerce.VendSession{}, ErrNotFound
	}
	return s.vend, nil
}

func (s *orderStatusLifecycleStub) GetVendSessionByOrderAndLineSequence(context.Context, uuid.UUID, int32) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, ErrNotFound
}

func (s *orderStatusLifecycleStub) ListVendSessionsForOrder(context.Context, uuid.UUID) ([]domaincommerce.VendSession, error) {
	return []domaincommerce.VendSession{s.vend}, nil
}

func (s *orderStatusLifecycleStub) UpdateVendSessionState(context.Context, UpdateVendSessionParams) (domaincommerce.VendSession, error) {
	return domaincommerce.VendSession{}, ErrNotConfigured
}

func (s *orderStatusLifecycleStub) GetLatestPaymentForOrder(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

func (s *orderStatusLifecycleStub) GetPaymentByID(context.Context, uuid.UUID) (domaincommerce.Payment, error) {
	return s.payment, nil
}

func (s *orderStatusLifecycleStub) GetLatestPaymentAttemptProviderReference(context.Context, uuid.UUID) (string, error) {
	return "", nil
}

func (s *orderStatusLifecycleStub) GetLatestPaymentAttemptPayload(context.Context, uuid.UUID) ([]byte, error) {
	return nil, nil
}

func (s *orderStatusLifecycleStub) InsertPaymentAttempt(context.Context, InsertPaymentAttemptParams) (PaymentAttemptView, error) {
	return PaymentAttemptView{}, ErrNotConfigured
}

func (s *orderStatusLifecycleStub) InsertRefundRow(context.Context, InsertRefundRowInput) (RefundRowView, error) {
	return RefundRowView{}, ErrNotConfigured
}

func (s *orderStatusLifecycleStub) ListRefundsForOrder(context.Context, uuid.UUID) ([]RefundRowView, error) {
	return nil, nil
}

func (s *orderStatusLifecycleStub) GetRefundByIDForOrder(context.Context, uuid.UUID, uuid.UUID) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (s *orderStatusLifecycleStub) GetRefundByOrderIdempotency(context.Context, uuid.UUID, string) (RefundRowView, error) {
	return RefundRowView{}, ErrNotFound
}

func (s *orderStatusLifecycleStub) SumNonFailedRefundAmountForPayment(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func (s *orderStatusLifecycleStub) FulfillSuccessfulVendAtomically(context.Context, FulfillSuccessfulVendInput) (FulfillSuccessfulVendResult, error) {
	return FulfillSuccessfulVendResult{}, ErrNotConfigured
}

func (s *orderStatusLifecycleStub) FulfillFailedVendAtomically(context.Context, FulfillFailedVendInput) (FulfillFailedVendResult, error) {
	return FulfillFailedVendResult{}, ErrNotConfigured
}

func TestGetOrderStatusView_FallsBackWhenSlotIndexMisses(t *testing.T) {
	orderID := uuid.New()
	vendID := uuid.New()
	life := &orderStatusLifecycleStub{
		order: domaincommerce.Order{ID: orderID, Status: "created"},
		vend: domaincommerce.VendSession{
			ID:        vendID,
			OrderID:   orderID,
			SlotIndex: 11,
			State:     "pending",
		},
		payment: domaincommerce.Payment{
			ID:      uuid.New(),
			OrderID: orderID,
			State:   "created",
		},
		slotMiss: true,
	}
	svc := NewService(Deps{
		OrderVend:     stubOrderVend{},
		PaymentOutbox: stubPaymentOutbox{},
		Lifecycle:     life,
		SaleLines:     stubSaleLineResolver{},
	})

	view, err := svc.GetOrderStatusView(context.Background(), uuid.Nil, orderID, 0, 0)
	require.NoError(t, err)
	require.Equal(t, "created", view.Order.Status)
	require.Equal(t, int32(11), view.Vend.SlotIndex)
	require.Equal(t, "pending", view.Vend.State)
	require.True(t, view.PaymentPresent)
	require.Equal(t, "created", view.Payment.State)
}

func TestGetCheckoutStatus_StillStrictForSlotMismatch(t *testing.T) {
	orderID := uuid.New()
	life := &orderStatusLifecycleStub{
		order: domaincommerce.Order{ID: orderID, Status: "created"},
		vend: domaincommerce.VendSession{
			ID:        uuid.New(),
			OrderID:   orderID,
			SlotIndex: 11,
			State:     "pending",
		},
		slotMiss: true,
	}
	svc := NewService(Deps{
		OrderVend:     stubOrderVend{},
		PaymentOutbox: stubPaymentOutbox{},
		Lifecycle:     life,
		SaleLines:     stubSaleLineResolver{},
	})

	_, err := svc.GetCheckoutStatus(context.Background(), uuid.Nil, orderID, 0)
	require.ErrorIs(t, err, ErrNotFound)
}
