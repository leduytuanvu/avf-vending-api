package correctness

import (
	"context"
	"strings"
	"testing"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestP06_E2E_VendInventory_* validates commerce vend inventory guards against double-application on replay.

func TestP06_E2E_VendInventory_successDecrementThenReplayDoesNotDoubleApply(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "p06-vend-inv-" + uuid.NewString()
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

	_, err = store.ApplyPaymentProviderWebhook(ctx, appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:         testfixtures.DevOrganizationID,
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      "prov-ref-p06-vend",
		WebhookEventID:         "evt-p06-vend",
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{}`),
		OutboxTopic:            "commerce.payments",
		OutboxEventType:        domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:          []byte(`{}`),
		OutboxAggregateType:    "payment",
		OutboxAggregateID:      payRes.Payment.ID,
		OutboxIdempotencyKey:   orderIDem + ":webhook:captured",
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE vend_sessions SET state = 'success' WHERE order_id = $1 AND slot_index = $2`,
		orderRes.Order.ID, int32(2))
	require.NoError(t, err)

	idemInv := "inv-sale-" + uuid.NewString()

	var qtyBefore int32
	require.NoError(t, pool.QueryRow(ctx, `
SELECT current_quantity FROM machine_slot_state
WHERE machine_id = $1 AND slot_index = $2 AND product_id = $3`,
		testfixtures.DevMachineID, int32(2), testfixtures.DevProductWater).Scan(&qtyBefore))

	replay1, err := store.ApplyCommerceVendSuccessInventory(ctx,
		testfixtures.DevOrganizationID,
		testfixtures.DevMachineID,
		orderRes.Order.ID,
		2,
		testfixtures.DevProductWater,
		idemInv,
		nil,
	)
	require.NoError(t, err)
	require.False(t, replay1)

	replay2, err := store.ApplyCommerceVendSuccessInventory(ctx,
		testfixtures.DevOrganizationID,
		testfixtures.DevMachineID,
		orderRes.Order.ID,
		2,
		testfixtures.DevProductWater,
		idemInv,
		nil,
	)
	require.NoError(t, err)
	require.True(t, replay2)

	var qtyAfter int32
	require.NoError(t, pool.QueryRow(ctx, `
SELECT current_quantity FROM machine_slot_state
WHERE machine_id = $1 AND slot_index = $2 AND product_id = $3`,
		testfixtures.DevMachineID, int32(2), testfixtures.DevProductWater).Scan(&qtyAfter))

	require.Equal(t, qtyBefore-1, qtyAfter)
}

func TestP06_E2E_VendInventory_failVendBlocksInventoryApply(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "p06-vend-fail-" + uuid.NewString()
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

	_, err = pool.Exec(ctx, `UPDATE vend_sessions SET state = 'failed' WHERE order_id = $1 AND slot_index = $2`,
		orderRes.Order.ID, int32(2))
	require.NoError(t, err)

	_, err = store.ApplyCommerceVendSuccessInventory(ctx,
		testfixtures.DevOrganizationID,
		testfixtures.DevMachineID,
		orderRes.Order.ID,
		2,
		testfixtures.DevProductWater,
		"p06-no-inv-fail",
		nil,
	)
	require.Error(t, err)
}

func TestP06_E2E_FinalizeSuccessfulVendReplayTripleDedupesInventory(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:             store,
		PaymentOutbox:         store,
		Lifecycle:             store,
		SaleLines:             store,
		WorkflowOrchestration: workfloworch.NewDisabled(),
	})

	orderIDem := "p06-fulfill-inv-" + uuid.NewString()
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

	_, err = store.ApplyPaymentProviderWebhook(ctx, appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:         testfixtures.DevOrganizationID,
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      "prov-ref-p06-atom",
		WebhookEventID:         "evt-p06-atom-vend",
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{}`),
		OutboxTopic:            "commerce.payments",
		OutboxEventType:        domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:          []byte(`{}`),
		OutboxAggregateType:    "payment",
		OutboxAggregateID:      payRes.Payment.ID,
		OutboxIdempotencyKey:   orderIDem + ":webhook:captured",
	})
	require.NoError(t, err)

	_, err = commerceSvc.AdvanceVend(ctx, appcommerce.AdvanceVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		OrderID:        orderRes.Order.ID,
		SlotIndex:      2,
		ToState:        "in_progress",
	})
	require.NoError(t, err)

	var qtyBefore int32
	require.NoError(t, pool.QueryRow(ctx, `
SELECT current_quantity FROM machine_slot_state
WHERE machine_id = $1 AND slot_index = $2 AND product_id = $3`,
		testfixtures.DevMachineID, int32(2), testfixtures.DevProductWater).Scan(&qtyBefore))

	dedupe := "replay-3x|" + uuid.NewString()
	for attempt := range 3 {
		fout, err := commerceSvc.FinalizeOrderAfterVend(ctx, appcommerce.FinalizeAfterVendInput{
			OrganizationID:     testfixtures.DevOrganizationID,
			OrderID:            orderRes.Order.ID,
			SlotIndex:          2,
			TerminalVendState:  "success",
			InventoryDedupeKey: dedupe,
		})
		require.NoError(t, err)
		require.Equal(t, "completed", fout.Order.Status)
		require.Equal(t, "success", fout.Vend.State)
		if attempt == 0 {
			require.False(t, fout.OrderVendReplay)
			require.False(t, fout.InventoryReplay)
			continue
		}
		require.True(t, fout.OrderVendReplay)
		require.True(t, fout.InventoryReplay)
	}

	var qtyAfter int32
	require.NoError(t, pool.QueryRow(ctx, `
SELECT current_quantity FROM machine_slot_state
WHERE machine_id = $1 AND slot_index = $2 AND product_id = $3`,
		testfixtures.DevMachineID, int32(2), testfixtures.DevProductWater).Scan(&qtyAfter))

	require.Equal(t, qtyBefore-1, qtyAfter)
}

func TestP06_E2E_ZerostockFinalizeSuccessRollsBack(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:             store,
		PaymentOutbox:         store,
		Lifecycle:             store,
		SaleLines:             store,
		WorkflowOrchestration: workfloworch.NewDisabled(),
	})

	orderIDem := "p06-stock0-" + uuid.NewString()
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

	_, err = store.ApplyPaymentProviderWebhook(ctx, appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:         testfixtures.DevOrganizationID,
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      "prov-stock0",
		WebhookEventID:         "evt-stock0",
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{}`),
		OutboxTopic:            "commerce.payments",
		OutboxEventType:        domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:          []byte(`{}`),
		OutboxAggregateType:    "payment",
		OutboxAggregateID:      payRes.Payment.ID,
		OutboxIdempotencyKey:   orderIDem + ":webhook:captured",
	})
	require.NoError(t, err)

	_, err = commerceSvc.AdvanceVend(ctx, appcommerce.AdvanceVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		OrderID:        orderRes.Order.ID,
		SlotIndex:      2,
		ToState:        "in_progress",
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
UPDATE machine_slot_state
SET current_quantity = 0
WHERE machine_id = $1 AND slot_index = $2 AND product_id = $3`,
		testfixtures.DevMachineID, int32(2), testfixtures.DevProductWater)
	require.NoError(t, err)

	_, err = commerceSvc.FinalizeOrderAfterVend(ctx, appcommerce.FinalizeAfterVendInput{
		OrganizationID:     testfixtures.DevOrganizationID,
		OrderID:            orderRes.Order.ID,
		SlotIndex:          2,
		TerminalVendState:  "success",
		InventoryDedupeKey: "stock0-inv|" + uuid.NewString(),
	})
	require.Error(t, err)
	require.True(t, strings.Contains(strings.ToLower(err.Error()), "insufficient"))

	o, err := store.GetOrderByID(ctx, orderRes.Order.ID)
	require.NoError(t, err)
	require.NotEqual(t, "completed", o.Status)

	v, err := store.GetVendSessionByOrderAndSlot(ctx, orderRes.Order.ID, 2)
	require.NoError(t, err)
	require.Equal(t, "in_progress", v.State)
}

func TestP06_E2E_VendFailsAfterCapturedWritesTimelineRefundHint(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:             store,
		PaymentOutbox:         store,
		Lifecycle:             store,
		SaleLines:             store,
		WorkflowOrchestration: workfloworch.NewDisabled(),
	})

	orderIDem := "p06-fail-tl-" + uuid.NewString()
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

	_, err = store.ApplyPaymentProviderWebhook(ctx, appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:         testfixtures.DevOrganizationID,
		OrderID:                orderRes.Order.ID,
		PaymentID:              payRes.Payment.ID,
		Provider:               "psp_fixture",
		ProviderReference:      "prov-failtl",
		WebhookEventID:         "evt-failtl",
		EventType:              "payment.captured",
		NormalizedPaymentState: "captured",
		Payload:                []byte(`{}`),
		OutboxTopic:            "commerce.payments",
		OutboxEventType:        domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:          []byte(`{}`),
		OutboxAggregateType:    "payment",
		OutboxAggregateID:      payRes.Payment.ID,
		OutboxIdempotencyKey:   orderIDem + ":webhook:captured",
	})
	require.NoError(t, err)

	_, err = commerceSvc.AdvanceVend(ctx, appcommerce.AdvanceVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		OrderID:        orderRes.Order.ID,
		SlotIndex:      2,
		ToState:        "in_progress",
	})
	require.NoError(t, err)

	fr := "motor stalled"
	fout, err := commerceSvc.FinalizeOrderAfterVend(ctx, appcommerce.FinalizeAfterVendInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		OrderID:           orderRes.Order.ID,
		SlotIndex:         2,
		TerminalVendState: "failed",
		FailureReason:     &fr,
	})
	require.NoError(t, err)
	require.Equal(t, "failed", fout.Order.Status)
	require.Equal(t, "failed", fout.Vend.State)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT count(*) FROM order_timelines
WHERE organization_id = $1 AND order_id = $2 AND event_type = 'commerce_vend_dispense_failed'`,
		testfixtures.DevOrganizationID, orderRes.Order.ID).Scan(&n))
	require.Equal(t, 1, n)
}
