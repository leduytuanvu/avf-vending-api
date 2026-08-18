package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/alerts"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const uncastInsertDeviceTelemetrySQL = `
INSERT INTO device_telemetry_events (machine_id, event_type, payload, dedupe_key)
VALUES ($1, $2, $3, $4)
RETURNING id
`

const uncastInsertIncidentDetailSQL = `
INSERT INTO machine_incidents (machine_id, severity, code, title, detail, dedupe_key, opened_at, updated_at, occurrence_count, last_alerted_at)
VALUES ($1,'high','incident_runtime_error','t',$2::jsonb,$3,$4,$4,1,NULL)
RETURNING id
`

func execModePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testDSN(t)
	migrateUp(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pcfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	pcfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func pgSQLState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestPgxExecMode_UncastTelemetryPayloadByteSlice_Returns22P02(t *testing.T) {
	pool := execModePool(t)
	ctx := context.Background()
	machineID := insertIncidentTestMachine(t, pool)
	err := pool.QueryRow(ctx, uncastInsertDeviceTelemetrySQL,
		machineID, "checkin", []byte("{}"), "dedupe-h3-"+id.NewUUIDV7String(),
	).Scan(new(int64))
	require.Error(t, err)
	require.Equal(t, "22P02", pgSQLState(err), "got %v", err)
}

func TestPgxExecMode_UncastIncidentDetailByteSlice_Returns22P02(t *testing.T) {
	pool := execModePool(t)
	ctx := context.Background()
	machineID := insertIncidentTestMachine(t, pool)
	now := time.Now().UTC()
	err := pool.QueryRow(ctx, uncastInsertIncidentDetailSQL,
		machineID, []byte("{}"), "fp-h3-"+id.NewUUIDV7String(), now,
	).Scan(new(uuid.UUID))
	require.Error(t, err)
	require.Equal(t, "22P02", pgSQLState(err), "got %v", err)
}

func TestAppendDeviceTelemetryEdgeEvent_QueryExecModeExec(t *testing.T) {
	pool := execModePool(t)
	store := postgres.NewStore(pool)
	ctx := context.Background()
	machineID := insertIncidentTestMachine(t, pool)

	dup, err := store.AppendDeviceTelemetryEdgeEvent(ctx, machineID, "checkin", []byte("{}"), "ok-empty-"+id.NewUUIDV7String())
	require.NoError(t, err)
	require.False(t, dup)

	nested := []byte(`{"event_id":"e1","note":"phiên bán hàng","attrs":{"n":1}}`)
	dup, err = store.AppendDeviceTelemetryEdgeEvent(ctx, machineID, "checkin", nested, "ok-nested-"+id.NewUUIDV7String())
	require.NoError(t, err)
	require.False(t, dup)
}

func TestProjectMachineIncident_QueryExecModeExec(t *testing.T) {
	pool := execModePool(t)
	store := postgres.NewStore(pool)
	machineID := insertIncidentTestMachine(t, pool)
	policy := alerts.DefaultPolicy()
	ctx := context.Background()
	occ := "incident_runtime_session:h3-" + id.NewUUIDV7String()
	in := projectInput(machineID, occ, "grpc")
	in.Fingerprint = "fp-h3-proj-" + id.NewUUIDV7String()
	res, err := store.ProjectMachineIncident(ctx, in, policy)
	require.NoError(t, err)
	require.True(t, res.NewOccurrence)
	require.Equal(t, 1, countOccurrences(t, pool, machineID, occ))
}
