package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/alerts"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const (
	auditOccurrenceA = "incident_runtime_error:audit-1"
	auditFingerprint = "fp-telegram-audit-shared"
)

func insertIncidentTestMachine(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	code := "is-" + siteID.String()
	_, err := pool.Exec(ctx, `INSERT INTO sites (id, name, code, status) VALUES ($1, $2, $3, 'active')`,
		siteID, "incident-site-"+machineID.String()[:8], code)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, 'online', 1)`,
		machineID, siteID, "sn-"+machineID.String())
	require.NoError(t, err)
	return machineID
}

func countOccurrences(t *testing.T, pool *pgxpool.Pool, machineID uuid.UUID, occurrenceID string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM machine_incident_occurrences WHERE machine_id = $1 AND occurrence_id = $2`,
		machineID, occurrenceID).Scan(&n)
	require.NoError(t, err)
	return n
}

func groupOccurrenceCount(t *testing.T, pool *pgxpool.Pool, machineID uuid.UUID, fingerprint string) int64 {
	t.Helper()
	var n int64
	err := pool.QueryRow(context.Background(), `
SELECT COALESCE(occurrence_count, 0) FROM machine_incidents WHERE machine_id = $1 AND dedupe_key = $2`,
		machineID, fingerprint).Scan(&n)
	require.NoError(t, err)
	return n
}

func countOutboxIntents(t *testing.T, pool *pgxpool.Pool, idempotencyKey string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM outbox_events WHERE topic = 'notification.telegram' AND idempotency_key = $1`,
		idempotencyKey).Scan(&n)
	require.NoError(t, err)
	return n
}

func outboxPayload(t *testing.T, pool *pgxpool.Pool, idempotencyKey string) map[string]any {
	t.Helper()
	var raw []byte
	err := pool.QueryRow(context.Background(), `
SELECT payload FROM outbox_events WHERE topic = 'notification.telegram' AND idempotency_key = $1 LIMIT 1`,
		idempotencyKey).Scan(&raw)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

func projectInput(machineID uuid.UUID, occurrenceID, transport string) alerts.ProjectInput {
	detail, _ := json.Marshal(map[string]any{
		"event_id":      occurrenceID,
		"password":      "my-secret",
		"authorization": "Bearer abc.secret",
		"access_token":  "fake-access",
	})
	return alerts.ProjectInput{
		MachineID:    machineID.String(),
		OccurrenceID: occurrenceID,
		Fingerprint:  auditFingerprint,
		Severity:     "high",
		Code:         "incident_runtime_error",
		Title:        "runtime error",
		EventType:    "incident_runtime_error",
		Transport:    transport,
		Detail:       detail,
		OccurredAt:   time.Now().UTC(),
	}
}

func TestProjectMachineIncident_CrossTransportDedupeMQTTThenGRPC(t *testing.T) {
	pool := testPool(t)
	store := postgres.NewStore(pool)
	machineID := insertIncidentTestMachine(t, pool)
	policy := alerts.DefaultPolicy()
	ctx := context.Background()

	r1, err := store.ProjectMachineIncident(ctx, projectInput(machineID, auditOccurrenceA, "mqtt"), policy)
	require.NoError(t, err)
	require.True(t, r1.NewOccurrence)
	require.True(t, r1.AlertQueued)

	r2, err := store.ProjectMachineIncident(ctx, projectInput(machineID, auditOccurrenceA, "grpc"), policy)
	require.NoError(t, err)
	require.True(t, r2.TransportDup)
	require.False(t, r2.NewOccurrence)
	require.False(t, r2.AlertQueued)

	require.Equal(t, 1, countOccurrences(t, pool, machineID, auditOccurrenceA))
	require.Equal(t, int64(1), groupOccurrenceCount(t, pool, machineID, auditFingerprint))
	key := alerts.TelegramAppIdempotencyKey(machineID.String(), auditOccurrenceA)
	require.Equal(t, 1, countOutboxIntents(t, pool, key))
	payload := outboxPayload(t, pool, key)
	require.Equal(t, "app", payload["source"])
	require.Equal(t, auditOccurrenceA, payload["occurrence_id"])
	require.Equal(t, machineID.String(), payload["machine_id"])
	raw, _ := json.Marshal(payload)
	require.NotContains(t, string(raw), "my-secret")
	require.NotContains(t, string(raw), "abc.secret")
	require.NotContains(t, string(raw), "fake-access")
}

func TestProjectMachineIncident_CrossTransportDedupeGRPCThenMQTT(t *testing.T) {
	pool := testPool(t)
	store := postgres.NewStore(pool)
	machineID := insertIncidentTestMachine(t, pool)
	policy := alerts.DefaultPolicy()
	ctx := context.Background()
	occ := "incident_anr:cross-grpc-first"

	_, err := store.ProjectMachineIncident(ctx, projectInput(machineID, occ, "grpc"), policy)
	require.NoError(t, err)
	r2, err := store.ProjectMachineIncident(ctx, projectInput(machineID, occ, "mqtt"), policy)
	require.NoError(t, err)
	require.True(t, r2.TransportDup)
	require.Equal(t, 1, countOccurrences(t, pool, machineID, occ))
	require.Equal(t, 1, countOutboxIntents(t, pool, alerts.TelegramAppIdempotencyKey(machineID.String(), occ)))
}

func TestProjectMachineIncident_SameFingerprintNewOccurrence(t *testing.T) {
	pool := testPool(t)
	store := postgres.NewStore(pool)
	machineID := insertIncidentTestMachine(t, pool)
	policy := alerts.DefaultPolicy()
	ctx := context.Background()

	occA := "incident_runtime_error:fp-a"
	occB := "incident_runtime_error:fp-b"
	_, err := store.ProjectMachineIncident(ctx, projectInput(machineID, occA, "grpc"), policy)
	require.NoError(t, err)
	r2, err := store.ProjectMachineIncident(ctx, projectInput(machineID, occB, "grpc"), policy)
	require.NoError(t, err)
	require.True(t, r2.NewOccurrence)
	require.True(t, r2.AlertQueued)
	require.Equal(t, int64(2), r2.OccurrenceCount)

	require.Equal(t, 1, countOccurrences(t, pool, machineID, occA))
	require.Equal(t, 1, countOccurrences(t, pool, machineID, occB))
	require.Equal(t, int64(2), groupOccurrenceCount(t, pool, machineID, auditFingerprint))
	require.Equal(t, 1, countOutboxIntents(t, pool, alerts.TelegramAppIdempotencyKey(machineID.String(), occA)))
	require.Equal(t, 1, countOutboxIntents(t, pool, alerts.TelegramAppIdempotencyKey(machineID.String(), occB)))
}

func TestProjectMachineIncident_MachineScopedOccurrenceIDs(t *testing.T) {
	pool := testPool(t)
	store := postgres.NewStore(pool)
	m1 := insertIncidentTestMachine(t, pool)
	m2 := insertIncidentTestMachine(t, pool)
	policy := alerts.DefaultPolicy()
	ctx := context.Background()
	occ := "incident_anr:123"

	_, err := store.ProjectMachineIncident(ctx, projectInput(m1, occ, "grpc"), policy)
	require.NoError(t, err)
	_, err = store.ProjectMachineIncident(ctx, projectInput(m2, occ, "grpc"), policy)
	require.NoError(t, err)
	require.Equal(t, 1, countOccurrences(t, pool, m1, occ))
	require.Equal(t, 1, countOccurrences(t, pool, m2, occ))
}

func TestProjectMachineIncident_ExactReplayNoInflation(t *testing.T) {
	pool := testPool(t)
	store := postgres.NewStore(pool)
	machineID := insertIncidentTestMachine(t, pool)
	policy := alerts.DefaultPolicy()
	ctx := context.Background()
	occ := "incident_runtime_error:replay-once"

	for i := 0; i < 5; i++ {
		_, err := store.ProjectMachineIncident(ctx, projectInput(machineID, occ, "grpc"), policy)
		require.NoError(t, err)
	}
	require.Equal(t, 1, countOccurrences(t, pool, machineID, occ))
	require.Equal(t, int64(1), groupOccurrenceCount(t, pool, machineID, auditFingerprint))
	require.Equal(t, 1, countOutboxIntents(t, pool, alerts.TelegramAppIdempotencyKey(machineID.String(), occ)))
}

func TestProjectMachineIncident_NonUUIDOccurrenceTextPreserved(t *testing.T) {
	pool := testPool(t)
	store := postgres.NewStore(pool)
	machineID := insertIncidentTestMachine(t, pool)
	policy := alerts.DefaultPolicy()
	ctx := context.Background()
	occ := "process_crash:boot-42:not-a-uuid"

	_, err := store.ProjectMachineIncident(ctx, projectInput(machineID, occ, "grpc"), policy)
	require.NoError(t, err)
	var got string
	err = pool.QueryRow(ctx, `
SELECT occurrence_id FROM machine_incident_occurrences WHERE machine_id = $1 AND occurrence_id = $2`,
		machineID, occ).Scan(&got)
	require.NoError(t, err)
	require.Equal(t, occ, got)
}

func TestProjectMachineIncident_AggregateUsesLastAlertedAt(t *testing.T) {
	pool := testPool(t)
	store := postgres.NewStore(pool)
	machineID := insertIncidentTestMachine(t, pool)
	policy := alerts.Policy{Cooldown: time.Hour, RepeatMode: alerts.RepeatAggregate}
	ctx := context.Background()

	r1, err := store.ProjectMachineIncident(ctx, projectInput(machineID, "incident_runtime_error:agg-1", "grpc"), policy)
	require.NoError(t, err)
	require.True(t, r1.AlertQueued)

	r2, err := store.ProjectMachineIncident(ctx, projectInput(machineID, "incident_runtime_error:agg-2", "grpc"), policy)
	require.NoError(t, err)
	require.True(t, r2.NewOccurrence)
	require.False(t, r2.AlertQueued)
	require.Equal(t, int64(2), groupOccurrenceCount(t, pool, machineID, auditFingerprint))
	require.Equal(t, 1, countOutboxIntents(t, pool, alerts.TelegramAppIdempotencyKey(machineID.String(), "incident_runtime_error:agg-1")))
	require.Equal(t, 0, countOutboxIntents(t, pool, alerts.TelegramAppIdempotencyKey(machineID.String(), "incident_runtime_error:agg-2")))
}

func TestProjectMachineIncident_SchemaIndexesPresent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	for _, name := range []string{
		"ux_machine_incident_occurrences_machine_occurrence",
		"ix_machine_incident_occurrences_machine_received",
	} {
		var cnt int
		err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1`, name).Scan(&cnt)
		require.NoError(t, err, name)
		require.Equal(t, 1, cnt, name)
	}
}
