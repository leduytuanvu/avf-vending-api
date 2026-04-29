package postgres_test

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appoutbox "github.com/avf/avf-vending-api/internal/app/outbox"
	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOutbox_CreatePaymentWithOutbox_TransactionallyLinked(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	idemOrder := "obx-txn-order-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      5,
		Currency:       "USD",
		SubtotalMinor:  10,
		TaxMinor:       0,
		TotalMinor:     10,
		IdempotencyKey: idemOrder,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	outTopic := "payments.txn." + uuid.NewString()
	outIDem := "obx-txn-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-txn-" + uuid.NewString(),
		OutboxTopic:          outTopic,
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	var payCount, obCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM payments WHERE order_id = $1`, orderRes.Order.ID).Scan(&payCount))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`, outTopic, outIDem).Scan(&obCount))
	require.Equal(t, 1, payCount)
	require.Equal(t, 1, obCount)
}

func TestOutbox_PublishFailuresDeadLetterThenAdminReplay(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	repo := postgres.NewOutboxRepository(pool)
	queries := db.New(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      3,
		Currency:       "USD",
		SubtotalMinor:  10,
		TaxMinor:       0,
		TotalMinor:     10,
		IdempotencyKey: "obx-dlq-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	outTopic := "payments.dlq." + uuid.NewString()
	outIDem := "obx-dlq-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-dlq-" + uuid.NewString(),
		OutboxTopic:          outTopic,
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	var obID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`,
		outTopic, outIDem,
	).Scan(&obID))

	const maxAttempts = 3
	_, err = pool.Exec(ctx, `UPDATE outbox_events SET max_publish_attempts = $2 WHERE id = $1`, obID, maxAttempts)
	require.NoError(t, err)

	for {
		var cnt int32
		require.NoError(t, pool.QueryRow(ctx, `SELECT publish_attempt_count FROM outbox_events WHERE id = $1`, obID).Scan(&cnt))
		dead := appreliability.OutboxWillDeadLetterThisFailure(cnt, maxAttempts)
		var next *time.Time
		if !dead {
			bo := appreliability.OutboxPublishBackoffAfterFailure(cnt+1, time.Second, time.Hour)
			t := time.Now().UTC().Add(bo)
			next = &t
		}
		require.NoError(t, repo.RecordOutboxPublishFailure(ctx, domainreliability.OutboxPublishFailureRecord{
			EventID:          obID,
			ErrorMessage:     "simulated broker failure",
			NextPublishAfter: next,
			DeadLettered:     dead,
		}))
		if dead {
			break
		}
	}

	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM outbox_events WHERE id = $1`, obID).Scan(&status))
	require.Equal(t, "dead_letter", status)

	n, err := queries.AdminRetryOutboxDeadLetter(ctx, obID)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM outbox_events WHERE id = $1`, obID).Scan(&status))
	require.Equal(t, "pending", status)
}

func TestOutbox_AdminReplayDeadLetter_AuditedInSameTransaction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	repo := postgres.NewOutboxRepository(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      3,
		Currency:       "USD",
		SubtotalMinor:  10,
		TaxMinor:       0,
		TotalMinor:     10,
		IdempotencyKey: "obx-audit-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	outTopic := "payments.audit." + uuid.NewString()
	outIDem := "obx-audit-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-audit-" + uuid.NewString(),
		OutboxTopic:          outTopic,
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	var obID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`,
		outTopic, outIDem,
	).Scan(&obID))

	const maxAttempts = 2
	_, err = pool.Exec(ctx, `UPDATE outbox_events SET max_publish_attempts = $2 WHERE id = $1`, obID, maxAttempts)
	require.NoError(t, err)

	for {
		var cnt int32
		require.NoError(t, pool.QueryRow(ctx, `SELECT publish_attempt_count FROM outbox_events WHERE id = $1`, obID).Scan(&cnt))
		dead := appreliability.OutboxWillDeadLetterThisFailure(cnt, maxAttempts)
		var next *time.Time
		if !dead {
			bo := appreliability.OutboxPublishBackoffAfterFailure(cnt+1, time.Second, time.Hour)
			t := time.Now().UTC().Add(bo)
			next = &t
		}
		require.NoError(t, repo.RecordOutboxPublishFailure(ctx, domainreliability.OutboxPublishFailureRecord{
			EventID:          obID,
			ErrorMessage:     "simulated broker failure",
			NextPublishAfter: next,
			DeadLettered:     dead,
		}))
		if dead {
			break
		}
	}

	var auditCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE organization_id = $1 AND action = $2 AND resource_type = 'outbox_events'`,
		testfixtures.DevOrganizationID, compliance.ActionAdminPlatformOutboxReplay,
	).Scan(&auditCount))
	require.Equal(t, 0, auditCount)

	rid := uuid.NewString()
	resID := strconv.FormatInt(obID, 10)
	rec := compliance.EnterpriseAuditRecord{
		OrganizationID: testfixtures.DevOrganizationID,
		ActorType:      compliance.ActorUser,
		ActorID:        &rid,
		Action:         compliance.ActionAdminPlatformOutboxReplay,
		ResourceType:   "outbox_events",
		ResourceID:     &resID,
		Metadata:       []byte(`{}`),
	}

	adminSvc := appoutbox.NewAdminService(pool)
	auditSvc := appaudit.NewService(pool, appaudit.ServiceOpts{CriticalFailOpen: false})
	n, err := adminSvc.ReplayDeadLetterTx(ctx, obID, auditSvc, rec)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE organization_id = $1 AND action = $2 AND resource_id = $3`,
		testfixtures.DevOrganizationID, compliance.ActionAdminPlatformOutboxReplay, strconv.FormatInt(obID, 10),
	).Scan(&auditCount))
	require.Equal(t, 1, auditCount)

	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM outbox_events WHERE id = $1`, obID).Scan(&status))
	require.Equal(t, "pending", status)

	n, err = adminSvc.ReplayDeadLetterTx(ctx, obID, auditSvc, rec)
	require.NoError(t, err)
	require.EqualValues(t, 0, n)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE organization_id = $1 AND action = $2 AND resource_id = $3`,
		testfixtures.DevOrganizationID, compliance.ActionAdminPlatformOutboxReplay, strconv.FormatInt(obID, 10),
	).Scan(&auditCount))
	require.Equal(t, 1, auditCount)
}

func TestOutbox_ConcurrentLeaseOutbox_NoDoubleClaim(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	repo := postgres.NewOutboxRepository(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      2,
		Currency:       "USD",
		SubtotalMinor:  10,
		TaxMinor:       0,
		TotalMinor:     10,
		IdempotencyKey: "obx-cc-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	outTopic := "payments.cc." + uuid.NewString()
	outIDem := "obx-cc-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-cc-" + uuid.NewString(),
		OutboxTopic:          outTopic,
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	var obID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`,
		outTopic, outIDem,
	).Scan(&obID))

	_, err = pool.Exec(ctx, `UPDATE outbox_events SET created_at = now() - interval '2 hours' WHERE id = $1`, obID)
	require.NoError(t, err)

	const workers = 24
	var winners int32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			wid := "test-worker-cc-" + uuid.NewString()
			leased, lerr := repo.LeaseOutboxForPublish(ctx, wid, time.Minute, 0, 50)
			if lerr != nil {
				return
			}
			for _, e := range leased {
				if e.ID == obID {
					atomic.AddInt32(&winners, 1)
				}
			}
		}(i)
	}
	wg.Wait()
	require.EqualValues(t, 1, winners)
}

func TestOutbox_ManualMarkDeadLetter(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	queries := db.New(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  10,
		TaxMinor:       0,
		TotalMinor:     10,
		IdempotencyKey: "obx-md-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	outTopic := "payments.md." + uuid.NewString()
	outIDem := "obx-md-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-md-" + uuid.NewString(),
		OutboxTopic:          outTopic,
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	var obID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`,
		outTopic, outIDem,
	).Scan(&obID))

	n, err := queries.AdminMarkOutboxManualDeadLetter(ctx, db.AdminMarkOutboxManualDeadLetterParams{
		ID:   obID,
		Note: "operator quarantine integration",
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM outbox_events WHERE id = $1`, obID).Scan(&status))
	require.Equal(t, "dead_letter", status)
}

func TestOutbox_MarkPublished_IdempotentNoop(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	repo := postgres.NewOutboxRepository(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      7,
		Currency:       "USD",
		SubtotalMinor:  10,
		TaxMinor:       0,
		TotalMinor:     10,
		IdempotencyKey: "obx-mp-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)
	outTopic := "payments.pub." + uuid.NewString()
	outIDem := "obx-mp-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-mp-" + uuid.NewString(),
		OutboxTopic:          outTopic,
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	var obID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`,
		outTopic, outIDem,
	).Scan(&obID))

	ok, err := repo.MarkOutboxPublished(ctx, obID)
	require.NoError(t, err)
	require.True(t, ok)

	ok2, err := repo.MarkOutboxPublished(ctx, obID)
	require.NoError(t, err)
	require.False(t, ok2)

	var pubAt interface{}
	require.NoError(t, pool.QueryRow(ctx, `SELECT published_at FROM outbox_events WHERE id = $1`, obID).Scan(&pubAt))
	require.NotNil(t, pubAt)
}
