package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestApplyPaymentProviderWebhook_amountMismatchRejected(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "wh-order-amt-" + uuid.NewString()
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

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
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

	bad := int64(199)
	cur := "USD"
	payload, _ := json.Marshal(map[string]any{"ok": true})
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:         testfixtures.DevOrganizationID,
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      "prov-ref-amt",
		WebhookEventID:         "evt-amt",
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                payload,
		ProviderAmountMinor:    &bad,
		Currency:               &cur,
	}
	_, err = store.ApplyPaymentProviderWebhook(ctx, in)
	require.Error(t, err)
	require.ErrorIs(t, err, appcommerce.ErrWebhookAmountCurrencyMismatch)
}

func TestApplyPaymentProviderWebhook_lateWebhookOnRefundedPaymentRejected(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "wh-order-term-" + uuid.NewString()
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

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "captured",
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

	_, err = pool.Exec(ctx, `UPDATE payments SET state = 'refunded' WHERE id = $1`, payRes.Payment.ID)
	require.NoError(t, err)

	payload, _ := json.Marshal(map[string]any{"ok": true})
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:         testfixtures.DevOrganizationID,
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      "prov-ref-late",
		WebhookEventID:         "evt-late",
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                payload,
	}
	_, err = store.ApplyPaymentProviderWebhook(ctx, in)
	require.Error(t, err)
	require.ErrorIs(t, err, appcommerce.ErrWebhookAfterTerminalOrder)
}
