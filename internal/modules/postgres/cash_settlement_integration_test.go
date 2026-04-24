package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/operator"
	cashdomain "github.com/avf/avf-vending-api/internal/domain/cash"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCashSettlement_summaryExpectedFromCommerce(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "cash-settle-order-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  150,
		TaxMinor:       0,
		TotalMinor:     150,
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
		AmountMinor:          150,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.captured",
		OutboxPayload:        []byte(`{"source":"cash_settlement_test"}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	sum, err := store.GetMachineCashboxSummary(ctx, testfixtures.DevOrganizationID, testfixtures.DevMachineID, "USD", 500)
	require.NoError(t, err)
	require.GreaterOrEqual(t, sum.ExpectedAmountMinor, int64(150))
	require.Equal(t, "USD", sum.Currency)
}

func TestCashSettlement_startCloseIdempotencyAndVariance(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "cash-settle-flow-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  400,
		TaxMinor:       0,
		TotalMinor:     400,
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
		AmountMinor:          400,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.captured",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)

	tid := testfixtures.DevTechnicianID
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)
	sid := sess.ID

	idemKey := "cash-start-" + uuid.NewString()
	open, err := store.StartMachineCashCollection(ctx, postgres.StartMachineCashCollectionInput{
		OrganizationID:      testfixtures.DevOrganizationID,
		MachineID:           testfixtures.DevMachineID,
		OperatorSessionID:   &sid,
		Currency:            "USD",
		Notes:               "test open",
		StartIdempotencyKey: idemKey,
		OpenedAt:            time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, "open", open.LifecycleStatus)

	replayOpen, err := store.StartMachineCashCollection(ctx, postgres.StartMachineCashCollectionInput{
		OrganizationID:      testfixtures.DevOrganizationID,
		MachineID:           testfixtures.DevMachineID,
		OperatorSessionID:   &sid,
		Currency:            "USD",
		StartIdempotencyKey: idemKey,
		OpenedAt:            time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, open.ID, replayOpen.ID)

	closed, err := store.CloseMachineCashCollection(ctx, postgres.CloseMachineCashCollectionInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		MachineID:               testfixtures.DevMachineID,
		CollectionID:            open.ID,
		OperatorSessionID:       &sid,
		CountedAmountMinor:      400,
		Currency:                "USD",
		Notes:                   "match",
		VarianceReviewThreshold: 500,
	})
	require.NoError(t, err)
	require.Equal(t, "closed", closed.LifecycleStatus)
	require.Equal(t, int64(400), closed.AmountMinor)
	require.Equal(t, int64(400), closed.ExpectedAmountMinor)
	require.Equal(t, int64(0), closed.VarianceAmountMinor)
	require.False(t, closed.RequiresReview)

	sameAgain, err := store.CloseMachineCashCollection(ctx, postgres.CloseMachineCashCollectionInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		MachineID:               testfixtures.DevMachineID,
		CollectionID:            open.ID,
		OperatorSessionID:       &sid,
		CountedAmountMinor:      400,
		Currency:                "USD",
		Notes:                   "match",
		VarianceReviewThreshold: 500,
	})
	require.NoError(t, err)
	require.Equal(t, closed.ID, sameAgain.ID)

	_, err = store.CloseMachineCashCollection(ctx, postgres.CloseMachineCashCollectionInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		MachineID:               testfixtures.DevMachineID,
		CollectionID:            open.ID,
		OperatorSessionID:       &sid,
		CountedAmountMinor:      401,
		Currency:                "USD",
		VarianceReviewThreshold: 500,
	})
	require.ErrorIs(t, err, cashdomain.ErrClosePayloadConflict)

	idem2 := "cash-start-2-" + uuid.NewString()
	open2, err := store.StartMachineCashCollection(ctx, postgres.StartMachineCashCollectionInput{
		OrganizationID:      testfixtures.DevOrganizationID,
		MachineID:           testfixtures.DevMachineID,
		OperatorSessionID:   &sid,
		Currency:            "USD",
		StartIdempotencyKey: idem2,
		OpenedAt:            time.Now().UTC(),
	})
	require.NoError(t, err)

	closed2, err := store.CloseMachineCashCollection(ctx, postgres.CloseMachineCashCollectionInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		MachineID:               testfixtures.DevMachineID,
		CollectionID:            open2.ID,
		OperatorSessionID:       &sid,
		CountedAmountMinor:      950,
		Currency:                "USD",
		VarianceReviewThreshold: 500,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), closed2.ExpectedAmountMinor, "no cash commerce since previous close")
	require.Equal(t, int64(950), closed2.VarianceAmountMinor)
	require.True(t, closed2.RequiresReview)

	_, err = store.GetMachineCashCollection(ctx, uuid.New(), testfixtures.DevMachineID, open2.ID)
	require.Error(t, err)

	_, err = svc.EndOperatorSession(ctx, operator.EndOperatorSessionInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		SessionID:      sid,
		FinalStatus:    domainoperator.SessionStatusEnded,
		EndedReason:    "test_cleanup",
	})
	require.NoError(t, err)
}

func TestCashSettlement_extendedCloseIdempotentAndConflict(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "cash-settle-ext-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  300,
		TaxMinor:       0,
		TotalMinor:     300,
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
		AmountMinor:          300,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.captured",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	repo := postgres.NewOperatorRepository(pool)
	machines := postgres.NewMachineRepository(pool)
	tech := postgres.NewTechnicianRepository(pool)
	assign := postgres.NewTechnicianAssignmentRepository(pool)
	svc := operator.NewService(repo, machines, tech, assign)
	tid := testfixtures.DevTechnicianID
	sess, err := svc.StartOperatorSession(ctx, operator.StartOperatorSessionInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)
	sid := sess.ID

	open, err := store.StartMachineCashCollection(ctx, postgres.StartMachineCashCollectionInput{
		OrganizationID:      testfixtures.DevOrganizationID,
		MachineID:           testfixtures.DevMachineID,
		OperatorSessionID:   &sid,
		Currency:            "USD",
		StartIdempotencyKey: "cash-ext-start-" + uuid.NewString(),
		OpenedAt:            time.Now().UTC(),
	})
	require.NoError(t, err)

	photo := uuid.NewString()
	in := postgres.CloseMachineCashCollectionInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		MachineID:               testfixtures.DevMachineID,
		CollectionID:            open.ID,
		OperatorSessionID:       &sid,
		CountedAmountMinor:      300,
		CountedCashboxMinor:     250,
		CountedRecyclerMinor:    50,
		Currency:                "USD",
		Notes:                   "ext",
		EvidencePhotoArtifactID: photo,
		Denominations: []postgres.CloseDenominationCount{
			{DenominationMinor: 50, Count: 6},
		},
		ClosedAtRFC3339:         "2026-04-24T12:00:00Z",
		VarianceReviewThreshold: 500,
		UsesExtendedCloseHash:   true,
	}
	closed, err := store.CloseMachineCashCollection(ctx, in)
	require.NoError(t, err)
	require.Equal(t, "closed", closed.LifecycleStatus)

	again, err := store.CloseMachineCashCollection(ctx, in)
	require.NoError(t, err)
	require.Equal(t, closed.ID, again.ID)

	bad := in
	bad.CountedCashboxMinor = 249
	bad.CountedAmountMinor = 299
	_, err = store.CloseMachineCashCollection(ctx, bad)
	require.ErrorIs(t, err, cashdomain.ErrClosePayloadConflict)

	_, err = svc.EndOperatorSession(ctx, operator.EndOperatorSessionInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		SessionID:      sid,
		FinalStatus:    domainoperator.SessionStatusEnded,
		EndedReason:    "test_cleanup",
	})
	require.NoError(t, err)
}
