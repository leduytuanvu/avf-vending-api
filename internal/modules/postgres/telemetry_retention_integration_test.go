package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRunTelemetryRetention_nilPool(t *testing.T) {
	_, err := postgres.RunTelemetryRetention(context.Background(), nil, config.TelemetryDataRetentionConfig{
		RetentionDays: 7, CriticalRetentionDays: 90, CleanupBatchSize: 10,
	}, time.Now().UTC())
	require.Error(t, err)
}

func TestRunTelemetryRetention_nonCriticalOldDeleted_recentKept(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	mid := testfixtures.DevMachineID

	oldKey := uuid.New().String()
	recentKey := uuid.New().String()
	_, err := pool.Exec(ctx, `
INSERT INTO device_telemetry_events (machine_id, event_type, payload, dedupe_key, received_at)
VALUES
  ($1, 'heartbeat', '{}'::jsonb, $2, now() - interval '30 days'),
  ($1, 'heartbeat', '{}'::jsonb, $3, now() - interval '1 hour')
`, mid, oldKey, recentKey)
	require.NoError(t, err)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM device_telemetry_events WHERE dedupe_key = ANY($1::text[])`, []string{oldKey, recentKey})
	}()

	cfg := config.TelemetryDataRetentionConfig{
		RetentionDays:         7,
		CriticalRetentionDays: 90,
		CleanupEnabled:        true,
		CleanupBatchSize:      500,
		CleanupDryRun:         false,
	}
	_, err = postgres.RunTelemetryRetention(ctx, pool, cfg, time.Now().UTC())
	require.NoError(t, err)

	var cntOld int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM device_telemetry_events WHERE dedupe_key = $1`, oldKey,
	).Scan(&cntOld))
	require.Equal(t, 0, cntOld)

	var cntRecent int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM device_telemetry_events WHERE dedupe_key = $1`, recentKey,
	).Scan(&cntRecent))
	require.Equal(t, 1, cntRecent)
}

func TestRunTelemetryRetention_criticalLinked_keptPastNormalHorizon(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	mid := testfixtures.DevMachineID
	idem := "crit-ret-" + uuid.New().String()

	_, err := pool.Exec(ctx, `
INSERT INTO critical_telemetry_event_status (machine_id, idempotency_key, status, accepted_at, processed_at)
VALUES ($1, $2, 'processed', now() - interval '40 days', now() - interval '40 days')
`, mid, idem)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO device_telemetry_events (machine_id, event_type, payload, dedupe_key, received_at)
VALUES ($1, 'payment.captured', '{"samples":{"x":1}}'::jsonb, $2, now() - interval '40 days')
`, mid, idem)
	require.NoError(t, err)

	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM device_telemetry_events WHERE dedupe_key = $1`, idem)
		_, _ = pool.Exec(ctx, `DELETE FROM critical_telemetry_event_status WHERE machine_id = $1 AND idempotency_key = $2`, mid, idem)
	}()

	cfg := config.TelemetryDataRetentionConfig{
		RetentionDays:         7,
		CriticalRetentionDays: 90,
		CleanupBatchSize:      500,
		CleanupDryRun:         false,
	}
	_, err = postgres.RunTelemetryRetention(ctx, pool, cfg, time.Now().UTC())
	require.NoError(t, err)

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM device_telemetry_events WHERE dedupe_key = $1`, idem,
	).Scan(&cnt))
	require.Equal(t, 1, cnt)
}

func TestRunTelemetryRetention_criticalLinked_deletedPastCriticalHorizon(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	mid := testfixtures.DevMachineID
	idem := "crit-ret-old-" + uuid.New().String()

	_, err := pool.Exec(ctx, `
INSERT INTO critical_telemetry_event_status (machine_id, idempotency_key, status, accepted_at, processed_at)
VALUES ($1, $2, 'processed', now() - interval '400 days', now() - interval '400 days')
`, mid, idem)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO device_telemetry_events (machine_id, event_type, payload, dedupe_key, received_at)
VALUES ($1, 'payment.captured', '{}'::jsonb, $2, now() - interval '400 days')
`, mid, idem)
	require.NoError(t, err)

	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM device_telemetry_events WHERE dedupe_key = $1`, idem)
		_, _ = pool.Exec(ctx, `DELETE FROM critical_telemetry_event_status WHERE machine_id = $1 AND idempotency_key = $2`, mid, idem)
	}()

	cfg := config.TelemetryDataRetentionConfig{
		RetentionDays:         7,
		CriticalRetentionDays: 90,
		CleanupBatchSize:      500,
		CleanupDryRun:         false,
	}
	_, err = postgres.RunTelemetryRetention(ctx, pool, cfg, time.Now().UTC())
	require.NoError(t, err)

	var cntEv int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM device_telemetry_events WHERE dedupe_key = $1`, idem,
	).Scan(&cntEv))
	require.Equal(t, 0, cntEv)

	var cntSt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM critical_telemetry_event_status WHERE machine_id = $1 AND idempotency_key = $2`, mid, idem,
	).Scan(&cntSt))
	require.Equal(t, 0, cntSt)
}

func TestRunTelemetryRetention_batchSizeHonoredAcrossLoops(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	mid := testfixtures.DevMachineID

	var keys []string
	for i := 0; i < 12; i++ {
		keys = append(keys, "batch-ret-"+uuid.New().String())
	}
	for _, k := range keys {
		_, err := pool.Exec(ctx, `
INSERT INTO device_telemetry_events (machine_id, event_type, payload, dedupe_key, received_at)
VALUES ($1, 't', '{}'::jsonb, $2, now() - interval '30 days')
`, mid, k)
		require.NoError(t, err)
	}
	defer func() {
		for _, k := range keys {
			_, _ = pool.Exec(ctx, `DELETE FROM device_telemetry_events WHERE dedupe_key = $1`, k)
		}
	}()

	cfg := config.TelemetryDataRetentionConfig{
		RetentionDays:         7,
		CriticalRetentionDays: 90,
		CleanupBatchSize:      5,
		CleanupDryRun:         false,
	}
	_, err := postgres.RunTelemetryRetention(ctx, pool, cfg, time.Now().UTC())
	require.NoError(t, err)

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM device_telemetry_events WHERE dedupe_key = ANY($1::text[])`, keys,
	).Scan(&cnt))
	require.Equal(t, 0, cnt)
}

func TestRunTelemetryRetention_dryRunDoesNotDelete(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	mid := testfixtures.DevMachineID
	k := "dry-run-" + uuid.New().String()

	_, err := pool.Exec(ctx, `
INSERT INTO device_telemetry_events (machine_id, event_type, payload, dedupe_key, received_at)
VALUES ($1, 't', '{}'::jsonb, $2, now() - interval '30 days')
`, mid, k)
	require.NoError(t, err)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM device_telemetry_events WHERE dedupe_key = $1`, k)
	}()

	cfg := config.TelemetryDataRetentionConfig{
		RetentionDays:         7,
		CriticalRetentionDays: 90,
		CleanupBatchSize:      500,
		CleanupDryRun:         true,
	}
	_, err = postgres.RunTelemetryRetention(ctx, pool, cfg, time.Now().UTC())
	require.NoError(t, err)

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM device_telemetry_events WHERE dedupe_key = $1`, k,
	).Scan(&cnt))
	require.Equal(t, 1, cnt)
}

func TestRunTelemetryRetention_machineCheckIns_orgScopedMachineOnly(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	mid := testfixtures.DevMachineID
	org := testfixtures.DevOrganizationID

	_, err := pool.Exec(ctx, `
INSERT INTO machine_check_ins (organization_id, machine_id, occurred_at)
VALUES ($1, $2, now() - interval '400 days')
`, org, mid)
	require.NoError(t, err)

	var checkID int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM machine_check_ins WHERE machine_id = $1 ORDER BY occurred_at DESC LIMIT 1`, mid,
	).Scan(&checkID))

	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM machine_check_ins WHERE id = $1`, checkID)
	}()

	cfg := config.TelemetryDataRetentionConfig{
		RetentionDays:         7,
		CriticalRetentionDays: 90,
		CleanupBatchSize:      500,
		CleanupDryRun:         false,
	}
	_, err = postgres.RunTelemetryRetention(ctx, pool, cfg, time.Now().UTC())
	require.NoError(t, err)

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM machine_check_ins WHERE id = $1`, checkID,
	).Scan(&cnt))
	require.Equal(t, 0, cnt)
}
