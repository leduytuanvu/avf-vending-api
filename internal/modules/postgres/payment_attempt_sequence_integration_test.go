package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func paymentOutboxInput(orderID uuid.UUID, provider, idem string) commerce.PaymentOutboxInput {
	return commerce.PaymentOutboxInput{
		OrderID:              orderID,
		Provider:             provider,
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       idem,
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{"source":"attempt_seq_test"}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderID,
		OutboxIdempotencyKey: idem + ":outbox",
	}
}

func createAttemptSeqTestOrder(t *testing.T, store *postgres.Store) commerce.Order {
	t.Helper()
	ctx := context.Background()
	orderIDem := "attempt-seq-order-" + uuid.NewString()
	res, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
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
	return res.Order
}

func TestPaymentAttemptSequence_firstPaymentAllocatesOne(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	order := createAttemptSeqTestOrder(t, store)

	res, err := store.CreatePaymentWithOutbox(ctx, paymentOutboxInput(order.ID, "cash", "cash-"+uuid.NewString()))
	require.NoError(t, err)
	require.Equal(t, int32(1), res.Payment.AttemptSeq)
}

func TestPaymentAttemptSequence_cashAfterMomoGetsTwo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	order := createAttemptSeqTestOrder(t, store)

	momo, err := store.CreatePaymentWithOutbox(ctx, paymentOutboxInput(order.ID, "momo", "momo-"+uuid.NewString()))
	require.NoError(t, err)
	require.Equal(t, int32(1), momo.Payment.AttemptSeq)

	cashIn := paymentOutboxInput(order.ID, "cash", "cash-"+uuid.NewString())
	cashIn.PaymentState = "captured"
	cash, err := store.CreatePaymentWithOutbox(ctx, cashIn)
	require.NoError(t, err)
	require.Equal(t, int32(2), cash.Payment.AttemptSeq)
}

func TestPaymentAttemptSequence_monotonicAcrossProviders(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	order := createAttemptSeqTestOrder(t, store)

	providers := []string{"momo", "zalopay", "vietqr", "cash"}
	for i, provider := range providers {
		in := paymentOutboxInput(order.ID, provider, provider+"-"+uuid.NewString())
		if provider == "cash" {
			in.PaymentState = "captured"
		}
		res, err := store.CreatePaymentWithOutbox(ctx, in)
		require.NoError(t, err)
		require.Equal(t, int32(i+1), res.Payment.AttemptSeq)
	}
}

func TestPaymentAttemptSequence_idempotentRetryPreservesSequence(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	order := createAttemptSeqTestOrder(t, store)

	idem := "idem-" + uuid.NewString()
	in := paymentOutboxInput(order.ID, "momo", idem)

	first, err := store.CreatePaymentWithOutbox(ctx, in)
	require.NoError(t, err)
	require.False(t, first.Replay)
	require.Equal(t, int32(1), first.Payment.AttemptSeq)

	second, err := store.CreatePaymentWithOutbox(ctx, in)
	require.NoError(t, err)
	require.True(t, second.Replay)
	require.Equal(t, first.Payment.ID, second.Payment.ID)
	require.Equal(t, int32(1), second.Payment.AttemptSeq)
}

func TestPaymentAttemptSequence_concurrentCreatesDistinctSequences(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	order := createAttemptSeqTestOrder(t, store)

	const workers = 4
	var wg sync.WaitGroup
	seqs := make([]int32, workers)
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			in := paymentOutboxInput(order.ID, "momo", "concurrent-"+uuid.NewString())
			res, err := store.CreatePaymentWithOutbox(ctx, in)
			errs[idx] = err
			if err == nil {
				seqs[idx] = res.Payment.AttemptSeq
			}
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	seen := map[int32]bool{}
	for _, seq := range seqs {
		require.GreaterOrEqual(t, seq, int32(1))
		require.False(t, seen[seq], "duplicate attempt_seq %d", seq)
		seen[seq] = true
	}
	require.Len(t, seen, workers)
}

func TestPaymentAttemptSequence_omittedAttemptSeqAllocates(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	order := createAttemptSeqTestOrder(t, store)

	in := paymentOutboxInput(order.ID, "cash", "cash-omit-"+uuid.NewString())
	in.AttemptSeq = 0
	in.PaymentState = "captured"
	res, err := store.CreatePaymentWithOutbox(ctx, in)
	require.NoError(t, err)
	require.Equal(t, int32(1), res.Payment.AttemptSeq)
}
