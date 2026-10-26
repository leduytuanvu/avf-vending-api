package postgres_test

import (
	"context"
	"errors"
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFulfillSuccessfulVendAtomically_usesWinningPaymentWhenLaterPrewarmCanceled(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: "vend-win-pay-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	orderID := orderRes.Order.ID

	momo, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrderID:              orderID,
		Provider:             "momo",
		PaymentState:         "captured",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       "momo-" + uuid.NewString(),
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.captured",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderID,
		OutboxIdempotencyKey: "momo-out-" + uuid.NewString(),
	})
	require.NoError(t, err)

	shopee, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrderID:              orderID,
		Provider:             "shopeepay",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       "shopee-" + uuid.NewString(),
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderID,
		OutboxIdempotencyKey: "shopee-out-" + uuid.NewString(),
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE payments SET outcome = 'winner' WHERE id = $1`, momo.Payment.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE payments SET state = 'canceled', outcome = 'superseded' WHERE id = $1`, shopee.Payment.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE orders
		SET status = 'paid', winning_payment_id = $1, winning_claimed_at = now()
		WHERE id = $2`, momo.Payment.ID, orderID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE vend_sessions SET state = 'in_progress' WHERE order_id = $1`, orderID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE machine_slot_state
		SET current_quantity = GREATEST(current_quantity, 3)
		WHERE machine_id = $1 AND slot_index = 1`, testfixtures.DevMachineID)
	require.NoError(t, err)

	out, err := store.FulfillSuccessfulVendAtomically(ctx, appcommerce.FulfillSuccessfulVendInput{
		OrderID:              orderID,
		SlotIndex:            1,
		InventoryDedupeKey:   "vend-win-pay-inv-" + uuid.NewString(),
		OutboxTopic:          "commerce.vends",
		OutboxEventType:      "vend.succeeded",
		OutboxAggregateType:  "order",
		OutboxIdempotencyKey: "vend-win-pay-ob-" + uuid.NewString(),
	})
	require.NoError(t, err)
	require.Equal(t, "completed", out.Order.Status)
	require.Equal(t, "success", out.Vend.State)
}

func TestFulfillSuccessfulVendAtomically_rejectsWhenNoWinnerAndLatestNotCaptured(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: "vend-unsettle-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	orderID := orderRes.Order.ID

	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrderID:              orderID,
		Provider:             "momo",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       "momo-open-" + uuid.NewString(),
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderID,
		OutboxIdempotencyKey: "momo-open-out-" + uuid.NewString(),
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE vend_sessions SET state = 'in_progress' WHERE order_id = $1`, orderID)
	require.NoError(t, err)

	_, err = store.FulfillSuccessfulVendAtomically(ctx, appcommerce.FulfillSuccessfulVendInput{
		OrderID:              orderID,
		SlotIndex:            1,
		InventoryDedupeKey:   "vend-unsettle-inv-" + uuid.NewString(),
		OutboxTopic:          "commerce.vends",
		OutboxEventType:      "vend.succeeded",
		OutboxAggregateType:  "order",
		OutboxIdempotencyKey: "vend-unsettle-ob-" + uuid.NewString(),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, appcommerce.ErrPaymentNotSettled), "got: %v", err)
}
