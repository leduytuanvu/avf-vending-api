package postgres_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/domain/device"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration tests in -short mode")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return dsn
}

func migrateUp(t *testing.T, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	goBin := os.Getenv("GO_BIN")
	if goBin == "" {
		goBin = "go"
	}
	repoRoot := testfixtures.RepoRoot(t)
	absRoot, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	migrationsDir := filepath.Join(absRoot, "migrations")
	cmd := exec.CommandContext(ctx, goBin, "run", "github.com/pressly/goose/v3/cmd/goose@v3.27.0",
		"-dir", migrationsDir,
		"postgres", dsn, "up",
	)
	cmd.Dir = absRoot
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", string(out))
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testDSN(t)
	migrateUp(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	testfixtures.EnsureDevCommerceIntegrationData(t, pool)
	return pool
}

func TestSchemaCriticalIndexes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	names := []string{
		"ux_orders_org_idempotency",
		"ux_outbox_topic_idempotency",
		"ux_command_ledger_machine_idempotency",
		"ix_outbox_unpublished",
		"ix_outbox_pending_due",
	}
	for _, name := range names {
		var cnt int
		err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1`, name).Scan(&cnt)
		require.NoError(t, err, name)
		require.Equal(t, 1, cnt, "missing index %s", name)
	}
}

func TestOrgSiteProductRepos_ReadSeed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	orgRepo := postgres.NewOrgRepository(pool)
	o, err := orgRepo.GetByID(ctx, testfixtures.DevOrganizationID)
	require.NoError(t, err)
	require.Equal(t, "Local Dev Org", o.Name)

	siteRepo := postgres.NewSiteRepository(pool)
	s, err := siteRepo.GetByID(ctx, testfixtures.DevSiteID)
	require.NoError(t, err)
	require.Equal(t, "Main DC", s.Name)

	prodRepo := postgres.NewProductRepository(pool)
	p, err := prodRepo.GetByID(ctx, testfixtures.DevProductCola)
	require.NoError(t, err)
	require.Equal(t, "SKU-COLA", p.Sku)
}

func TestCreateOrderWithVendSession_AndReplay(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	idem := "order-" + uuid.NewString()
	in := commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductCola,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  100,
		TaxMinor:       0,
		TotalMinor:     100,
		IdempotencyKey: idem,
		OrderStatus:    "created",
		VendState:      "pending",
	}

	r1, err := store.CreateOrderWithVendSession(ctx, in)
	require.NoError(t, err)
	require.False(t, r1.Replay)

	r2, err := store.CreateOrderWithVendSession(ctx, in)
	require.NoError(t, err)
	require.True(t, r2.Replay)
	require.Equal(t, r1.Order.ID, r2.Order.ID)
	require.Equal(t, r1.Vend.ID, r2.Vend.ID)
}

func TestCreateOrderWithVendSession_RollbackOnInvalidMachine(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	idem := "rollback-" + uuid.NewString()
	badMachine := uuid.New()
	_, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      badMachine,
		ProductID:      testfixtures.DevProductCola,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  1,
		TaxMinor:       0,
		TotalMinor:     1,
		IdempotencyKey: idem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.Error(t, err)

	var cnt int
	qerr := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE organization_id = $1 AND idempotency_key = $2`,
		testfixtures.DevOrganizationID, idem,
	).Scan(&cnt)
	require.NoError(t, qerr)
	require.Zero(t, cnt)
}

func TestCreatePaymentWithOutbox_UnpublishedOutbox(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "pay-order-" + uuid.NewString()
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

	payIDem := "pay-" + uuid.NewString()
	outIDem := "obx-" + uuid.NewString()
	in := commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "payments",
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{"ok":true}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	}

	res, err := store.CreatePaymentWithOutbox(ctx, in)
	require.NoError(t, err)
	require.False(t, res.Replay)
	require.Nil(t, res.Outbox.PublishedAt)

	res2, err := store.CreatePaymentWithOutbox(ctx, in)
	require.NoError(t, err)
	require.True(t, res2.Replay)
	require.Nil(t, res2.Outbox.PublishedAt)
}

func TestAppendCommandUpdateShadow_MonotonicAndIdempotent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID

	var seqStart int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT command_sequence FROM machines WHERE id = $1`, mid).Scan(&seqStart))

	idem1 := "cmd-a-" + uuid.NewString()
	r1, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      mid,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: idem1,
		DesiredState:   []byte(`{"x":1}`),
	})
	require.NoError(t, err)
	require.False(t, r1.Replay)
	require.NotEqual(t, uuid.Nil, r1.CommandID)
	require.Equal(t, seqStart+1, r1.Sequence)

	idem2 := "cmd-b-" + uuid.NewString()
	r2, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      mid,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: idem2,
		DesiredState:   []byte(`{"x":2}`),
	})
	require.NoError(t, err)
	require.False(t, r2.Replay)
	require.Equal(t, seqStart+2, r2.Sequence)

	r2b, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      mid,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: idem2,
		DesiredState:   []byte(`{"x":2}`),
	})
	require.NoError(t, err)
	require.True(t, r2b.Replay)
	require.Equal(t, r2.CommandID, r2b.CommandID)
	require.Equal(t, r2.Sequence, r2b.Sequence)

	var cnt int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM command_ledger WHERE machine_id = $1 AND idempotency_key = $2`,
		mid, idem2,
	).Scan(&cnt)
	require.NoError(t, err)
	require.Equal(t, 1, cnt)
}

func TestAppendCommandUpdateShadowAndOutbox_CommandAndOutboxAtomic(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID

	cmdIDem := "cmd-obo-" + uuid.NewString()
	outIDem := "obx-cmd-" + uuid.NewString()
	topic := "commands." + uuid.NewString()

	in := postgres.AppendCommandWithOutboxInput{
		Command: device.AppendCommandInput{
			MachineID:      mid,
			CommandType:    "noop",
			Payload:        []byte(`{}`),
			IdempotencyKey: cmdIDem,
			DesiredState:   []byte(`{"cmd":1}`),
		},
		OrganizationID:       testfixtures.DevOrganizationID,
		OutboxTopic:          topic,
		OutboxEventType:      "command.issued",
		OutboxPayload:        []byte(`{"issued":true}`),
		OutboxAggregateType:  "machine",
		OutboxAggregateID:    mid,
		OutboxIdempotencyKey: outIDem,
	}

	r1, err := store.AppendCommandUpdateShadowAndOutbox(ctx, in)
	require.NoError(t, err)
	require.False(t, r1.CommandReplay)
	require.False(t, r1.OutboxReplay)
	require.NotZero(t, r1.Sequence)
	require.Nil(t, r1.Outbox.PublishedAt)

	r2, err := store.AppendCommandUpdateShadowAndOutbox(ctx, in)
	require.NoError(t, err)
	require.True(t, r2.CommandReplay)
	require.True(t, r2.OutboxReplay)
	require.Equal(t, r1.Sequence, r2.Sequence)
	require.Equal(t, r1.Outbox.ID, r2.Outbox.ID)
}

func TestAppendCommandUpdateShadowAndOutbox_RepairsMissingOutboxInSameTransactionPath(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID

	cmdIDem := "cmd-obo-repair-" + uuid.NewString()
	outIDem := "obx-repair-" + uuid.NewString()
	topic := "commands.repair." + uuid.NewString()

	in := postgres.AppendCommandWithOutboxInput{
		Command: device.AppendCommandInput{
			MachineID:      mid,
			CommandType:    "noop",
			Payload:        []byte(`{}`),
			IdempotencyKey: cmdIDem,
			DesiredState:   []byte(`{"repair":1}`),
		},
		OrganizationID:       testfixtures.DevOrganizationID,
		OutboxTopic:          topic,
		OutboxEventType:      "command.issued",
		OutboxPayload:        []byte(`{"repair":true}`),
		OutboxAggregateType:  "machine",
		OutboxAggregateID:    mid,
		OutboxIdempotencyKey: outIDem,
	}

	r1, err := store.AppendCommandUpdateShadowAndOutbox(ctx, in)
	require.NoError(t, err)
	require.False(t, r1.CommandReplay)

	_, err = pool.Exec(ctx, `DELETE FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`, topic, outIDem)
	require.NoError(t, err)

	r2, err := store.AppendCommandUpdateShadowAndOutbox(ctx, in)
	require.NoError(t, err)
	require.True(t, r2.CommandReplay)
	require.False(t, r2.OutboxReplay, "missing outbox row must be re-inserted on command replay")
	require.Equal(t, r1.Sequence, r2.Sequence)
	require.Nil(t, r2.Outbox.PublishedAt)

	var cnt int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`,
		topic, outIDem,
	).Scan(&cnt)
	require.NoError(t, err)
	require.Equal(t, 1, cnt)
}

func TestCreatePaymentWithOutbox_RepairsMissingOutboxWhenPaymentExists(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductCola,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  50,
		TaxMinor:       0,
		TotalMinor:     50,
		IdempotencyKey: "pay-repair-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := "pay-repair-" + uuid.NewString()
	outTopic := "payments.repair." + uuid.NewString()
	outIDem := "obx-pay-repair-" + uuid.NewString()
	in := commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          50,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          outTopic,
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{"repair":1}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	}

	res1, err := store.CreatePaymentWithOutbox(ctx, in)
	require.NoError(t, err)
	require.False(t, res1.Replay)

	_, err = pool.Exec(ctx, `DELETE FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`, outTopic, outIDem)
	require.NoError(t, err)

	res2, err := store.CreatePaymentWithOutbox(ctx, in)
	require.NoError(t, err)
	require.True(t, res2.Replay)
	require.Equal(t, res1.Payment.ID, res2.Payment.ID)
	require.Nil(t, res2.Outbox.PublishedAt)

	var payCnt, obCnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE order_id = $1 AND idempotency_key = $2`,
		orderRes.Order.ID, payIDem,
	).Scan(&payCnt))
	require.Equal(t, 1, payCnt)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE topic = $1 AND idempotency_key = $2`,
		outTopic, outIDem,
	).Scan(&obCnt))
	require.Equal(t, 1, obCnt)
}

func TestOutboxRepository_ListUnpublished_IncludesNewCommerceOutbox(t *testing.T) {
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
		IdempotencyKey: "obx-list-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	outTopic := "payments.list." + uuid.NewString()
	outIDem := "obx-list-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-list-" + uuid.NewString(),
		OutboxTopic:          outTopic,
		OutboxEventType:      "payment.created",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "order",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	events, err := repo.ListUnpublished(ctx, 10000)
	require.NoError(t, err)
	var found bool
	for _, e := range events {
		if e.Topic == outTopic && e.IdempotencyKey != nil && *e.IdempotencyKey == outIDem {
			found = true
			break
		}
	}
	require.True(t, found, "fresh outbox row should appear in unpublished listing")
}

func TestOutboxRepository_BackoffHidesRowUntilDue(t *testing.T) {
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
		IdempotencyKey: "obx-bo-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	outTopic := "payments.bo." + uuid.NewString()
	outIDem := "obx-bo-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-bo-" + uuid.NewString(),
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

	future := time.Now().UTC().Add(90 * time.Minute)
	require.NoError(t, repo.RecordOutboxPublishFailure(ctx, domainreliability.OutboxPublishFailureRecord{
		EventID:          obID,
		ErrorMessage:     "forced transport failure for test",
		NextPublishAfter: &future,
		DeadLettered:     false,
	}))

	events, err := repo.ListUnpublished(ctx, 10000)
	require.NoError(t, err)
	for _, e := range events {
		if e.ID == obID {
			t.Fatalf("expected outbox id %d to be hidden until next_publish_after", obID)
		}
	}

	stats, err := repo.GetOutboxPipelineStats(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.PendingTotal, int64(1))
}

func TestOutboxRepository_RecordFailureIncrementsAttemptCount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	repo := postgres.NewOutboxRepository(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      4,
		Currency:       "USD",
		SubtotalMinor:  10,
		TaxMinor:       0,
		TotalMinor:     10,
		IdempotencyKey: "obx-attempt-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	outTopic := "payments.attempt." + uuid.NewString()
	outIDem := "obx-attempt-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-attempt-" + uuid.NewString(),
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

	next := time.Now().UTC().Add(time.Minute)
	require.NoError(t, repo.RecordOutboxPublishFailure(ctx, domainreliability.OutboxPublishFailureRecord{
		EventID:          obID,
		ErrorMessage:     "first failure",
		NextPublishAfter: &next,
		DeadLettered:     false,
	}))

	var cnt int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT publish_attempt_count FROM outbox_events WHERE id = $1`, obID,
	).Scan(&cnt))
	require.Equal(t, int32(1), cnt)

	require.NoError(t, repo.RecordOutboxPublishFailure(ctx, domainreliability.OutboxPublishFailureRecord{
		EventID:          obID,
		ErrorMessage:     "second failure",
		NextPublishAfter: &next,
		DeadLettered:     true,
	}))
	var dead bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT publish_attempt_count, dead_lettered_at IS NOT NULL FROM outbox_events WHERE id = $1`, obID,
	).Scan(&cnt, &dead))
	require.Equal(t, int32(2), cnt)
	require.True(t, dead)
}

func TestApplyCommandReceiptTransition_DedupeKeyPreventsDoubleApply(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      mid,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "rcp-cmd-" + uuid.NewString(),
		DesiredState:   []byte(`{"want":1}`),
	})
	require.NoError(t, err)

	dedupe := "rcp-dedupe-" + uuid.NewString()
	shadow := []byte(`{"applied":true}`)
	occAt := time.Now().UTC()
	p1 := postgres.CommandReceiptTransitionParams{
		MachineID:          mid,
		Sequence:           appendRes.Sequence,
		Status:             "acked",
		Payload:            []byte(`{}`),
		DedupeKey:          dedupe,
		ReportedShadowJSON: shadow,
		CommandID:          appendRes.CommandID,
		OccurredAt:         occAt,
	}
	r1, err := store.ApplyCommandReceiptTransition(ctx, p1)
	require.NoError(t, err)
	require.False(t, r1.ReceiptReplay)

	r2, err := store.ApplyCommandReceiptTransition(ctx, p1)
	require.NoError(t, err)
	require.True(t, r2.ReceiptReplay)

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM device_command_receipts WHERE dedupe_key = $1`, dedupe,
	).Scan(&cnt))
	require.Equal(t, 1, cnt)

	var reportedJSON []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT reported_state FROM machine_shadow WHERE machine_id = $1`, mid,
	).Scan(&reportedJSON))
	var reported map[string]any
	require.NoError(t, json.Unmarshal(reportedJSON, &reported))
	require.Equal(t, true, reported["applied"])
}

func TestApplyCommandReceiptTransition_RejectsLateSuccessAck(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      mid,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "rcp-late-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, mid, appendRes.Sequence)
	require.NoError(t, err)
	ledgerDeadline := time.Now().UTC().Add(time.Hour)
	att, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, mid, nil, []byte(`{}`), ledgerDeadline, "")
	require.NoError(t, err)
	past := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, store.MarkMQTTDispatchAttemptSent(ctx, att.ID, past))

	_, err = store.ApplyCommandReceiptTransition(ctx, postgres.CommandReceiptTransitionParams{
		MachineID:  mid,
		Sequence:   appendRes.Sequence,
		Status:     "acked",
		Payload:    []byte(`{}`),
		DedupeKey:  "late-ack-" + uuid.NewString(),
		CommandID:  appendRes.CommandID,
		OccurredAt: time.Now().UTC(),
	})
	require.Error(t, err)
}

func TestApplyCommandReceiptTransition_IdempotentTerminalOutcome(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      mid,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "rcp-term-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, mid, appendRes.Sequence)
	require.NoError(t, err)
	att, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, mid, nil, []byte(`{}`), time.Now().UTC().Add(time.Hour), "")
	require.NoError(t, err)
	require.NoError(t, store.MarkMQTTDispatchAttemptSent(ctx, att.ID, time.Now().UTC().Add(30*time.Second)))

	d1 := "term-d1-" + uuid.NewString()
	p1 := postgres.CommandReceiptTransitionParams{
		MachineID:  mid,
		Sequence:   appendRes.Sequence,
		Status:     "acked",
		Payload:    []byte(`{}`),
		DedupeKey:  d1,
		CommandID:  appendRes.CommandID,
		OccurredAt: time.Now().UTC(),
	}
	r1, err := store.ApplyCommandReceiptTransition(ctx, p1)
	require.NoError(t, err)
	require.False(t, r1.ReceiptReplay)

	d2 := "term-d2-" + uuid.NewString()
	r2, err := store.ApplyCommandReceiptTransition(ctx, postgres.CommandReceiptTransitionParams{
		MachineID:  mid,
		Sequence:   appendRes.Sequence,
		Status:     "acked",
		Payload:    []byte(`{}`),
		DedupeKey:  d2,
		CommandID:  appendRes.CommandID,
		OccurredAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	require.True(t, r2.ReceiptReplay)
}

func TestApplyCommandReceiptTransition_ConflictingAckIsAudited(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      mid,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "rcp-conf-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, mid, appendRes.Sequence)
	require.NoError(t, err)
	confAtt, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, mid, nil, []byte(`{}`), time.Now().UTC().Add(time.Hour), "")
	require.NoError(t, err)
	require.NoError(t, store.MarkMQTTDispatchAttemptSent(ctx, confAtt.ID, time.Now().UTC().Add(30*time.Second)))

	_, err = store.ApplyCommandReceiptTransition(ctx, postgres.CommandReceiptTransitionParams{
		MachineID:  mid,
		Sequence:   appendRes.Sequence,
		Status:     "acked",
		Payload:    []byte(`{}`),
		DedupeKey:  "conf-a-" + uuid.NewString(),
		CommandID:  appendRes.CommandID,
		OccurredAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	r, err := store.ApplyCommandReceiptTransition(ctx, postgres.CommandReceiptTransitionParams{
		MachineID:  mid,
		Sequence:   appendRes.Sequence,
		Status:     "failed",
		Payload:    []byte(`{}`),
		DedupeKey:  "conf-b-" + uuid.NewString(),
		CommandID:  appendRes.CommandID,
		OccurredAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	require.True(t, r.IgnoredConflict)

	var n int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE organization_id = $1 AND action = $2`,
		testfixtures.DevOrganizationID, "mqtt.command_ack_conflict",
	).Scan(&n))
	require.GreaterOrEqual(t, n, int64(1))
}

func TestFleetQueries_OrganizationAndSiteScope(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)

	byOrg, err := q.ListMachinesByOrganizationID(ctx, testfixtures.DevOrganizationID)
	require.NoError(t, err)
	require.Len(t, byOrg, 1)
	require.Equal(t, testfixtures.DevMachineID, byOrg[0].ID)

	emptyOrg, err := q.ListMachinesByOrganizationID(ctx, uuid.New())
	require.NoError(t, err)
	require.Empty(t, emptyOrg)

	bySite, err := q.ListMachinesBySiteAndOrganization(ctx, db.ListMachinesBySiteAndOrganizationParams{
		SiteID:         testfixtures.DevSiteID,
		OrganizationID: testfixtures.DevOrganizationID,
	})
	require.NoError(t, err)
	require.Len(t, bySite, 1)

	wrongSite, err := q.ListMachinesBySiteAndOrganization(ctx, db.ListMachinesBySiteAndOrganizationParams{
		SiteID:         uuid.New(),
		OrganizationID: testfixtures.DevOrganizationID,
	})
	require.NoError(t, err)
	require.Empty(t, wrongSite)
}

func TestFleetQueries_TechnicianAssignmentScope(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)

	technicianID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	rows, err := q.ListMachinesForTechnicianID(ctx, technicianID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, testfixtures.DevMachineID, rows[0].ID)

	otherTech := uuid.New()
	empty, err := q.ListMachinesForTechnicianID(ctx, otherTech)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestMessagingConsumerDedupe_DuplicateReturnsFalse(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	d := postgres.NewMessagingConsumerDeduper(pool)
	msgID := "dup-" + uuid.NewString()
	first, err := d.TryClaim(ctx, "integration_dup_consumer", "test.subject.dup", msgID)
	require.NoError(t, err)
	require.True(t, first)
	dup, err := d.TryClaim(ctx, "integration_dup_consumer", "test.subject.dup", msgID)
	require.NoError(t, err)
	require.False(t, dup)
}

func TestOutboxRepository_LeaseOutboxForPublish_SetsPublishing(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	repo := postgres.NewOutboxRepository(pool)

	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      5,
		Currency:       "USD",
		SubtotalMinor:  10,
		TaxMinor:       0,
		TotalMinor:     10,
		IdempotencyKey: "obx-lease-order-" + uuid.NewString(),
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	outTopic := "payments.lease." + uuid.NewString()
	outIDem := "obx-lease-" + uuid.NewString()
	_, err = store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "test",
		PaymentState:         "created",
		AmountMinor:          10,
		Currency:             "USD",
		IdempotencyKey:       "pay-lease-" + uuid.NewString(),
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

	leased, err := repo.LeaseOutboxForPublish(ctx, "test-worker-lease", 60*time.Second, 0, 50)
	require.NoError(t, err)
	var found bool
	for _, e := range leased {
		if e.ID == obID {
			found = true
			require.Equal(t, "publishing", e.Status)
			require.NotNil(t, e.LockedBy)
			require.Equal(t, "test-worker-lease", *e.LockedBy)
			break
		}
	}
	require.True(t, found, "leased batch should include the aged pending row")

	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM outbox_events WHERE id = $1`, obID).Scan(&status))
	require.Equal(t, "publishing", status)
}

func TestInsertMQTTDispatchAttemptWithLedgerMeta_RespectsMaxDispatchAttempts(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      mid,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "max-att-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE command_ledger SET max_dispatch_attempts = 1 WHERE id = $1`, appendRes.CommandID)
	require.NoError(t, err)

	deadline := time.Now().UTC().Add(time.Hour)
	_, err = store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, appendRes.CommandID, mid, nil, []byte(`{}`), deadline, "")
	require.NoError(t, err)

	_, err = store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, appendRes.CommandID, mid, nil, []byte(`{}`), deadline, "")
	require.ErrorIs(t, err, postgres.ErrMQTTMaxDispatchAttemptsExceeded)
}
