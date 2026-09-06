package postgres_test

import (
	"context"
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestApplyPaymentProviderWebhook_replayPromotesCapturedUnpaidOrder(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "wh-promote-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
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

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	provRef := "prov-ref-" + uuid.NewString()
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      provRef,
		WebhookEventID:         "evt-" + uuid.NewString(),
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{"ok":true}`),
	}
	r1, err := store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)
	require.False(t, r1.Replay)
	require.Equal(t, "paid", r1.Order.Status)

	// Simulate settlement deadlock: payment captured but order stuck at created.
	_, err = pool.Exec(ctx, `UPDATE orders SET status = 'created', winning_payment_id = NULL, winning_claimed_at = NULL WHERE id = $1`, orderRes.Order.ID)
	require.NoError(t, err)

	r2, err := store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)
	require.True(t, r2.Replay)
	require.Equal(t, "paid", r2.Order.Status)
}
