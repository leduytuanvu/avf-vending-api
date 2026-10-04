package grpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/stretchr/testify/require"
)

func TestMachineOfflineEvents_processingStatusAllowsDispatchFailures(t *testing.T) {
	t.Parallel()

	pool := machineGRPCTestPool(t)
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	require.NoError(t, insertMachineReplayLedgerFixture(ctx, pool, siteID, machineID))

	seq := int64(9001)
	_, err := pool.Exec(ctx, `
INSERT INTO machine_offline_events (
    machine_id, offline_sequence, event_type, event_id, client_event_id,
    occurred_at, payload, processing_status, processing_error, idempotency_key
) VALUES (
    $1, $2, 'telemetry.batch', 'evt', 'client-dispatch-fail-test',
    $3, '{}'::jsonb, 'processing', '', 'idem-dispatch-fail-test'
)`,
		machineID, seq, time.Now().UTC())
	require.NoError(t, err)

	q := db.New(pool)
	for _, st := range []string{"failed_retryable", "failed_terminal"} {
		err := q.UpdateMachineOfflineEventStatus(ctx, db.UpdateMachineOfflineEventStatusParams{
			MachineID:        machineID,
			OfflineSequence:  seq,
			ProcessingStatus: st,
			ProcessingError:  "synthetic dispatch failure",
		})
		require.NoError(t, err, "status %q must satisfy CHECK after migration 00038", st)
	}

	var stored string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT processing_status FROM machine_offline_events WHERE machine_id = $1 AND offline_sequence = $2`,
		machineID, seq).Scan(&stored))
	require.Equal(t, "failed_terminal", stored)
}
