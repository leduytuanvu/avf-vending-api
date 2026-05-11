package correctness

import (
	"context"
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	apppayments "github.com/avf/avf-vending-api/internal/app/payments"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestP06_PaymentReconciliation_providerCapturedLocalPendingVisible(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	q := db.New(pool)
	admin, err := apppayments.NewAdminService(pool, q, nil)
	require.NoError(t, err)

	orderIDem := "p06-rec-pend-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
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

	provRef := "prov-ref-rec-a-" + uuid.NewString()
	webhookEv := "evt-rec-a-" + uuid.NewString()
	whIDem := orderIDem + ":webhook:captured"
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		OrderID:                 orderRes.Order.ID,
		PaymentID:               payRes.Payment.ID,
		Provider:                "psp_fixture",
		ProviderReference:       provRef,
		WebhookEventID:          webhookEv,
		EventType:               "payment.captured",
		NormalizedPaymentState:  "captured",
		Payload:                 []byte(`{"normalized_payment_state":"captured"}`),
		OutboxTopic:             "commerce.payments",
		OutboxEventType:         domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:           []byte(`{}`),
		OutboxAggregateType:     "payment",
		OutboxAggregateID:       payRes.Payment.ID,
		OutboxIdempotencyKey:    whIDem,
		WebhookValidationStatus: "hmac_verified",
	}
	_, err = store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE payments SET state = 'authorized', updated_at = now() WHERE id = $1`, payRes.Payment.ID)
	require.NoError(t, err)

	rep, err := admin.ListPaymentReconciliationDrift(ctx, testfixtures.DevOrganizationID, 3600, 100)
	require.NoError(t, err)
	found := false
	for _, r := range rep.ProviderCapturedVsLocalPending {
		if r.PaymentID != nil && *r.PaymentID == payRes.Payment.ID.String() {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestP06_PaymentReconciliation_localCapturedMissingProviderEvidenceVisible(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	q := db.New(pool)
	admin, err := apppayments.NewAdminService(pool, q, nil)
	require.NoError(t, err)

	orderIDem := "p06-rec-evid-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
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

	provRef := "prov-ref-rec-b-" + uuid.NewString()
	webhookEv := "evt-rec-b-" + uuid.NewString()
	whIDem := orderIDem + ":webhook:captured"
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		OrderID:                 orderRes.Order.ID,
		PaymentID:               payRes.Payment.ID,
		Provider:                "psp_fixture",
		ProviderReference:       provRef,
		WebhookEventID:          webhookEv,
		EventType:               "payment.captured",
		NormalizedPaymentState:  "captured",
		Payload:                 []byte(`{"normalized_payment_state":"captured"}`),
		OutboxTopic:             "commerce.payments",
		OutboxEventType:         domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:           []byte(`{}`),
		OutboxAggregateType:     "payment",
		OutboxAggregateID:       payRes.Payment.ID,
		OutboxIdempotencyKey:    whIDem,
		WebhookValidationStatus: "hmac_verified",
	}
	_, err = store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM payment_provider_events WHERE payment_id = $1`, payRes.Payment.ID)
	require.NoError(t, err)

	rep, err := admin.ListPaymentReconciliationDrift(ctx, testfixtures.DevOrganizationID, 3600, 100)
	require.NoError(t, err)
	found := false
	for _, r := range rep.LocalCapturedMissingProviderAudit {
		if r.PaymentID == payRes.Payment.ID.String() {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestP06_PaymentReconciliation_appliedWebhookAmountMismatchVisible(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	q := db.New(pool)
	admin, err := apppayments.NewAdminService(pool, q, nil)
	require.NoError(t, err)

	orderIDem := "p06-rec-mm-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
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

	provRef := "prov-ref-rec-mm-" + uuid.NewString()
	webhookEv := "evt-rec-mm-" + uuid.NewString()
	whIDem := orderIDem + ":webhook:captured"
	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		OrderID:                 orderRes.Order.ID,
		PaymentID:               payRes.Payment.ID,
		Provider:                "psp_fixture",
		ProviderReference:       provRef,
		WebhookEventID:          webhookEv,
		EventType:               "payment.captured",
		NormalizedPaymentState:  "captured",
		Payload:                 []byte(`{"normalized_payment_state":"captured"}`),
		ProviderAmountMinor:     int64Ptr(200),
		Currency:                stringPtr("USD"),
		OutboxTopic:             "commerce.payments",
		OutboxEventType:         domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:           []byte(`{}`),
		OutboxAggregateType:     "payment",
		OutboxAggregateID:       payRes.Payment.ID,
		OutboxIdempotencyKey:    whIDem,
		WebhookValidationStatus: "hmac_verified",
	}
	_, err = store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)

	var evID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM payment_provider_events WHERE payment_id = $1 ORDER BY id DESC LIMIT 1`,
		payRes.Payment.ID).Scan(&evID))

	_, err = pool.Exec(ctx, `UPDATE payment_provider_events SET provider_amount_minor = 999 WHERE id = $1`, evID)
	require.NoError(t, err)

	rep, err := admin.ListPaymentReconciliationDrift(ctx, testfixtures.DevOrganizationID, 3600, 100)
	require.NoError(t, err)
	found := false
	for _, r := range rep.AppliedWebhookVsPaymentAmountMismatch {
		if r.ProviderEventID == evID && r.PaymentID == payRes.Payment.ID.String() {
			found = true
			require.NotNil(t, r.WebhookAmountMinor)
			require.Equal(t, int64(999), *r.WebhookAmountMinor)
			require.Equal(t, int64(200), r.PaymentAmountMinor)
			break
		}
	}
	require.True(t, found)
}

func int64Ptr(v int64) *int64 { return &v }

func stringPtr(v string) *string { return &v }
