package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

const uncastInsertMachineOfflineEventSQL = `
INSERT INTO machine_offline_events (
    machine_id, offline_sequence, event_type, event_id, client_event_id,
    occurred_at, payload, processing_status, processing_error, idempotency_key
) VALUES (
    $1, $2, 'commerce.offline_sale', 'evt', 'client-uncast-json',
    $3, $4, 'processing', '', 'idem-uncast-json'
)
RETURNING id
`

func TestInsertMachineOfflineEvent_UncastByteSliceJSON_Returns22P02(t *testing.T) {
	pool := commerceJSONPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	machineID := insertIncidentTestMachine(t, pool)
	seq := time.Now().UnixNano() % 1_000_000_000
	if seq < 1 {
		seq = 1
	}
	var rowID uuid.UUID
	err := pool.QueryRow(ctx, uncastInsertMachineOfflineEventSQL,
		machineID, seq, time.Now().UTC(), []byte(`{"orderId":"test"}`),
	).Scan(&rowID)
	require.Error(t, err)
	require.Equal(t, "22P02", pgErrCode(err))
}

func TestInsertMachineOfflineEvent_CastTextJSON_QueryExecModeExec(t *testing.T) {
	pool := commerceJSONPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	machineID := insertIncidentTestMachine(t, pool)
	seq := time.Now().UnixNano()%1_000_000_000 + 1_000_000_000
	payload := []byte(`{"orderId":"` + id.NewUUIDV7String() + `","canonical_operation":"commerce.offline_sale"}`)

	q := db.New(pool)
	row, err := q.InsertMachineOfflineEvent(ctx, db.InsertMachineOfflineEventParams{
		MachineID:        machineID,
		OfflineSequence:  seq,
		EventType:        "commerce.offline_sale",
		EventID:          "evt-bind-test",
		ClientEventID:    "client-bind-" + id.NewUUIDV7String(),
		OccurredAt:       time.Now().UTC(),
		Payload:          pgjson.RequiredString(payload),
		ProcessingStatus: "processing",
		ProcessingError:  "",
		IdempotencyKey:   "idem-bind-" + id.NewUUIDV7String(),
	})
	require.NoError(t, err)
	require.True(t, row.Inserted)
	require.NotEmpty(t, row.Payload)

	var payloadText string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT payload::text FROM machine_offline_events WHERE machine_id = $1 AND offline_sequence = $2`,
		machineID, seq,
	).Scan(&payloadText))
	require.Contains(t, payloadText, "commerce.offline_sale")
}
