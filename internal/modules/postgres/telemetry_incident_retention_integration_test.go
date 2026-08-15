package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/alerts"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/stretchr/testify/require"
)

func TestRunTelemetryRetention_prunesOldIncidentOccurrences(t *testing.T) {
	pool := testPool(t)
	store := postgres.NewStore(pool)
	machineID := insertIncidentTestMachine(t, pool)
	policy := alerts.DefaultPolicy()
	ctx := context.Background()

	oldOcc := "incident_runtime_error:ret-old"
	recentOcc := "incident_runtime_error:ret-recent"
	_, err := store.ProjectMachineIncident(ctx, projectInput(machineID, oldOcc, "grpc"), policy)
	require.NoError(t, err)
	_, err = store.ProjectMachineIncident(ctx, projectInput(machineID, recentOcc, "grpc"), policy)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
UPDATE machine_incident_occurrences SET received_at = now() - interval '120 days'
WHERE machine_id = $1 AND occurrence_id = $2`, machineID, oldOcc)
	require.NoError(t, err)

	cfg := config.TelemetryDataRetentionConfig{
		RetentionDays:         7,
		CriticalRetentionDays: 90,
		CleanupEnabled:        true,
		CleanupBatchSize:      500,
		CleanupDryRun:         false,
	}
	_, err = postgres.RunTelemetryRetention(ctx, pool, cfg, time.Now().UTC())
	require.NoError(t, err)

	require.Equal(t, 0, countOccurrences(t, pool, machineID, oldOcc))
	require.Equal(t, 1, countOccurrences(t, pool, machineID, recentOcc))
}
