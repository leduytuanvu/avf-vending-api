package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/telemetry"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAppendDeviceTelemetryEdgeEvent_duplicateSafe(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID
	dedupe := "edge-dedupe-" + uuid.NewString()
	payload := []byte(`{"slot":1}`)

	dup1, err := store.AppendDeviceTelemetryEdgeEvent(ctx, mid, "events.vend", payload, dedupe)
	require.NoError(t, err)
	require.False(t, dup1)

	dup2, err := store.AppendDeviceTelemetryEdgeEvent(ctx, mid, "events.vend", payload, dedupe)
	require.NoError(t, err)
	require.True(t, dup2)

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM device_telemetry_events WHERE dedupe_key = $1`, dedupe,
	).Scan(&cnt))
	require.Equal(t, 1, cnt)
}

func TestAppendInventoryEventFromDeviceTelemetry_duplicateSafe(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID
	org := testfixtures.DevOrganizationID
	idem := "inv-idem-" + uuid.NewString()
	payload, err := json.Marshal(map[string]any{
		"event_type":      "adjustment",
		"slot_code":       "legacy-0",
		"quantity_delta":  0,
		"quantity_after":  10,
		"product_id":      testfixtures.DevProductCola.String(),
	})
	require.NoError(t, err)

	env := telemetry.Envelope{
		MachineID:   mid,
		TenantID:    &org,
		Idempotency: idem,
		ReceivedAt:  time.Now().UTC(),
	}

	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_events WHERE machine_id = $1 AND metadata->>'idempotency_key' = $2`, mid, idem)
	}()

	dup1, err := store.AppendInventoryEventFromDeviceTelemetry(ctx, env, payload)
	require.NoError(t, err)
	require.False(t, dup1)

	dup2, err := store.AppendInventoryEventFromDeviceTelemetry(ctx, env, payload)
	require.NoError(t, err)
	require.True(t, dup2)

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM inventory_events WHERE machine_id = $1 AND metadata->>'idempotency_key' = $2`,
		mid, idem,
	).Scan(&cnt))
	require.Equal(t, 1, cnt)
}

func TestCriticalMetricsRollup_anchorPattern_singleMergeOnReplay(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	mid := testfixtures.DevMachineID
	idem := "rollup-anchor-" + uuid.NewString()
	eventType := "payment.captured"
	data := []byte(`{"samples":{"captured_cents":100}}`)
	ts := time.Date(2030, 3, 15, 12, 0, 0, 0, time.UTC)

	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM device_telemetry_events WHERE dedupe_key = $1`, idem)
		_, _ = pool.Exec(ctx, `DELETE FROM telemetry_rollups WHERE machine_id = $1 AND granularity = '1m' AND metric_key = 'captured_cents' AND bucket_start = date_trunc('minute', $2::timestamptz)`, mid, ts)
	}()

	dup1, err := store.AppendDeviceTelemetryEdgeEvent(ctx, mid, eventType, data, idem)
	require.NoError(t, err)
	require.False(t, dup1)

	v := 100.0
	require.NoError(t, store.MergeTelemetryRollupMinute(ctx, mid, ts, "captured_cents", 1, &v, &v, &v, &v, nil))

	dup2, err := store.AppendDeviceTelemetryEdgeEvent(ctx, mid, eventType, data, idem)
	require.NoError(t, err)
	require.True(t, dup2)

	// JetStream worker returns after duplicate anchor without a second MergeTelemetryRollupMinute.
	var sampleCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT sample_count FROM telemetry_rollups WHERE machine_id = $1 AND granularity = '1m' AND metric_key = 'captured_cents' AND bucket_start = date_trunc('minute', $2::timestamptz)`,
		mid, ts,
	).Scan(&sampleCount))
	require.Equal(t, int64(1), sampleCount)
}
