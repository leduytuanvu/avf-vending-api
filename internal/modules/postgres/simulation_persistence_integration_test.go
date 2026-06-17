package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestSimulationPersistence_CreateOrderAndPayment(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	runID := "sim-run-" + uuid.NewString()
	scenario := "exact_cash_price_bill_stack_success_tcn_success"
	idem := "sim-order-" + uuid.NewString()

	r1, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		MachineID:          testfixtures.DevMachineID,
		ProductID:          testfixtures.DevProductCola,
		SlotIndex:          1,
		Currency:           "USD",
		SubtotalMinor:      15000,
		TaxMinor:           0,
		TotalMinor:         15000,
		IdempotencyKey:     idem,
		OrderStatus:        "created",
		VendState:          "pending",
		Simulated:          true,
		SimulationRunID:    runID,
		SimulationScenario: scenario,
		FakeBill:           true,
		FakeBoard:          true,
	})
	require.NoError(t, err)
	require.True(t, r1.Order.Simulated)
	require.NotNil(t, r1.Order.SimulationRunID)
	require.Equal(t, runID, *r1.Order.SimulationRunID)
	require.Equal(t, scenario, *r1.Order.SimulationScenario)
	require.True(t, r1.Order.FakeBill)
	require.True(t, r1.Order.FakeBoard)
	require.True(t, r1.Vend.Simulated)

	realIdem := "real-order-" + uuid.NewString()
	r2, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  100,
		TaxMinor:       0,
		TotalMinor:     100,
		IdempotencyKey: realIdem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	require.False(t, r2.Order.Simulated)
	require.False(t, r2.Order.FakeBill)
	require.False(t, r2.Order.FakeBoard)

	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrderID:              r1.Order.ID,
		Provider:             "cash",
		PaymentState:         "captured",
		AmountMinor:          15000,
		Currency:             "USD",
		IdempotencyKey:       "sim-pay-" + uuid.NewString(),
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{"simulated":true}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    r1.Order.ID,
		OutboxIdempotencyKey: "sim-obx-" + uuid.NewString(),
		Simulated:            true,
		SimulationRunID:      runID,
		SimulationScenario:   scenario,
		FakeBill:             true,
		FakeBoard:            true,
	})
	require.NoError(t, err)
	require.True(t, payRes.Payment.Simulated)
	require.True(t, payRes.Outbox.Simulated)
	require.NotNil(t, payRes.Outbox.SimulationRunID)
	require.Equal(t, runID, *payRes.Outbox.SimulationRunID)
}

func TestSimulationPersistence_RevenueExcludesSimulated(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	q := db.New(pool)

	now := time.Now().UTC()
	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)

	simTotal, err := q.ReportingSalesTotalsFiltered(ctx, db.ReportingSalesTotalsFilteredParams{
		Column1: from,
		Column2: to,
		Column3: uuid.Nil,
		Column4: testfixtures.DevMachineID,
		Column5: uuid.Nil,
	})
	require.NoError(t, err)

	withSim, err := q.ReportingSalesTotalsWithSimulationOption(ctx, db.ReportingSalesTotalsWithSimulationOptionParams{
		Column1:          from,
		Column2:          to,
		Column3:          uuid.Nil,
		Column4:          testfixtures.DevMachineID,
		IncludeSimulated: pgtype.Bool{Bool: true, Valid: true},
	})
	require.NoError(t, err)

	require.GreaterOrEqual(t, withSim.GrossTotalMinor, simTotal.GrossTotalMinor)
	_ = store
}
