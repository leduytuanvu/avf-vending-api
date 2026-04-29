package correctness

import (
	"context"
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestP06_E2E_PaymentWebhook_duplicateDeliveryIsReplayWithoutDoublePosting(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "p06-wh-" + uuid.NewString()
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

	webhookOutboxIDem := orderIDem + ":webhook:captured"

	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:         testfixtures.DevOrganizationID,
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      "prov-ref-p06-wh",
		WebhookEventID:         "evt-p06-wh-dup",
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{}`),
		OutboxTopic:            "commerce.payments",
		OutboxEventType:        domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:          []byte(`{}`),
		OutboxAggregateType:    "payment",
		OutboxAggregateID:      payRes.Payment.ID,
		OutboxIdempotencyKey:   webhookOutboxIDem,
	}

	r1, err := store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)
	require.False(t, r1.Replay)

	r2, err := store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)
	require.True(t, r2.Replay)
}
