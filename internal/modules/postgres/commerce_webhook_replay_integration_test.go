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

func TestApplyPaymentProviderWebhook_replayByProviderRef(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "wh-order-" + uuid.NewString()
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

	payload := []byte(`{"ok":true}`)
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:         testfixtures.DevOrganizationID,
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      "prov-ref-1",
		WebhookEventID:         "evt-1",
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                payload,
	}

	r1, err := store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)
	require.False(t, r1.Replay)
	require.Equal(t, "captured", r1.Payment.State)

	r2, err := store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)
	require.True(t, r2.Replay)
	require.Equal(t, r1.ProviderRowID, r2.ProviderRowID)
}

func TestApplyPaymentProviderWebhook_webhookEventIdConflict(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "wh-order2-" + uuid.NewString()
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

	payload, _ := json.Marshal(map[string]any{"seq": 1})
	in1 := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:         testfixtures.DevOrganizationID,
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      "prov-ref-a",
		WebhookEventID:         "shared-evt",
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                payload,
	}
	_, err = store.ApplyPaymentProviderWebhook(ctx, in1)
	require.NoError(t, err)

	in2 := in1
	in2.ProviderReference = "prov-ref-b"
	_, err = store.ApplyPaymentProviderWebhook(ctx, in2)
	require.Error(t, err)
	require.ErrorIs(t, err, appcommerce.ErrWebhookIdempotencyConflict)
}
