package postgres_test

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestCashCheckout_storeFlow mirrors POST /v1/commerce/cash-checkout persistence: order, captured cash payment, then paid order.
func TestCashCheckout_storeFlow_orderPaid_cashProviderCaptured(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "cash-order-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":cash:payment"
	outIDem := orderIDem + ":cash:payment:outbox:" + orderRes.Order.ID.String()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "cash",
		PaymentState:         "captured",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.captured",
		OutboxPayload:        []byte(`{"source":"cash_checkout"}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	ord, err := store.UpdateOrderStatus(ctx, orderRes.Order.ID, testfixtures.DevOrganizationID, "paid")
	require.NoError(t, err)
	require.Equal(t, "paid", ord.Status)

	pay, err := store.GetLatestPaymentForOrder(ctx, orderRes.Order.ID)
	require.NoError(t, err)
	require.Equal(t, "cash", pay.Provider)
	require.Equal(t, "captured", pay.State)
}
