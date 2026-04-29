package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
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
	webhookOutboxIDem := orderIDem + ":webhook:captured"
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
		OutboxTopic:            "commerce.payments",
		OutboxEventType:        domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:          []byte(`{"source":"webhook_test"}`),
		OutboxAggregateType:    "payment",
		OutboxAggregateID:      payRes.Payment.ID,
		OutboxIdempotencyKey:   webhookOutboxIDem,
	}

	r1, err := store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)
	require.False(t, r1.Replay)
	require.Equal(t, "captured", r1.Payment.State)

	var attemptCountAfterFirst int64
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM payment_attempts WHERE payment_id = $1`, payRes.Payment.ID).Scan(&attemptCountAfterFirst)
	require.NoError(t, err)
	require.EqualValues(t, 1, attemptCountAfterFirst)

	var webhookOutboxCountAfterFirst int64
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_events WHERE topic = $1 AND idempotency_key = $2 AND event_type = $3`,
		"commerce.payments", webhookOutboxIDem, domainreliability.OutboxEventPaymentConfirmed).Scan(&webhookOutboxCountAfterFirst)
	require.NoError(t, err)
	require.EqualValues(t, 1, webhookOutboxCountAfterFirst, "webhook state change must enqueue payment.confirmed in the same transaction")

	r2, err := store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)
	require.True(t, r2.Replay)
	require.Equal(t, r1.ProviderRowID, r2.ProviderRowID)

	var attemptCountAfterReplay int64
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM payment_attempts WHERE payment_id = $1`, payRes.Payment.ID).Scan(&attemptCountAfterReplay)
	require.NoError(t, err)
	require.Equal(t, attemptCountAfterFirst, attemptCountAfterReplay, "idempotent replay must not insert another payment_attempt row")

	var webhookOutboxCountAfterReplay int64
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_events WHERE topic = $1 AND idempotency_key = $2 AND event_type = $3`,
		"commerce.payments", webhookOutboxIDem, domainreliability.OutboxEventPaymentConfirmed).Scan(&webhookOutboxCountAfterReplay)
	require.NoError(t, err)
	require.Equal(t, webhookOutboxCountAfterFirst, webhookOutboxCountAfterReplay, "idempotent replay must not insert another outbox row")
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

func TestApplyReconciledPaymentTransition_insertsOutboxInSameTransaction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "reconcile-order-" + uuid.NewString()
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
	sessionOutboxIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
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
		OutboxIdempotencyKey: sessionOutboxIDem,
	})
	require.NoError(t, err)

	reconcileOutboxIDem := "payment_reconcile:" + payRes.Payment.ID.String() + ":" + domainreliability.OutboxEventPaymentConfirmed
	updated, err := store.ApplyReconciledPaymentTransition(ctx, commerce.ReconciledPaymentTransitionInput{
		PaymentID:            payRes.Payment.ID,
		ToState:              "captured",
		Reason:               "provider_probe:paid",
		ProviderHint:         []byte(`{"status":"paid"}`),
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:        []byte(`{"source":"reconcile_test"}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    payRes.Payment.ID,
		OutboxIdempotencyKey: reconcileOutboxIDem,
	})
	require.NoError(t, err)
	require.Equal(t, "captured", updated.State)

	var count int64
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_events WHERE topic = $1 AND idempotency_key = $2 AND event_type = $3`,
		"commerce.payments", reconcileOutboxIDem, domainreliability.OutboxEventPaymentConfirmed).Scan(&count)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
}
