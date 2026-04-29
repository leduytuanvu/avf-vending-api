package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/observability/telemetryretentionprom"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxRetentionBatchesPerStage = 500

// TelemetryRetentionRunResult captures per-stage row counts — deletes when DryRun=false, matching candidates when DryRun=true.
type TelemetryRetentionRunResult struct {
	Stages map[string]int64
	DryRun bool
}

// RunTelemetryRetention deletes aged telemetry projection rows (not financial OLTP payments/orders/audit tables).
// cfg drives horizons and batch sizing; metrics are recorded via telemetryretentionprom when deletes run (dry-run skips deletes).
func RunTelemetryRetention(ctx context.Context, pool *pgxpool.Pool, cfg config.TelemetryDataRetentionConfig, now time.Time) (TelemetryRetentionRunResult, error) {
	out := TelemetryRetentionRunResult{Stages: map[string]int64{}, DryRun: cfg.CleanupDryRun}
	if pool == nil {
		return out, errors.New("postgres: nil pool")
	}

	start := time.Now()
	deleted := make(map[string]int64)
	var runErr error
	defer func() {
		telemetryretentionprom.ObserveRun(start, deleted, cfg.CleanupDryRun, runErr)
	}()

	normalCutoff, criticalCutoff := cfg.RetentionCutoffs(now)
	q := db.New(pool)
	batch := cfg.CleanupBatchSize
	if batch <= 0 {
		runErr = errors.New("postgres: telemetry retention batch size must be > 0")
		return out, runErr
	}

	critTs := pgtype.Timestamptz{Time: criticalCutoff.UTC(), Valid: true}

	if cfg.CleanupDryRun {
		stages, err := telemetryRetentionCandidateCounts(ctx, q, normalCutoff, criticalCutoff, critTs)
		if err != nil {
			runErr = err
			return out, err
		}
		out.Stages = stages
		deleted = stages
		return out, nil
	}

	type batchFn func(context.Context) (int64, error)

	runStage := func(stage string, fn batchFn) error {
		var stageTotal int64
		for i := 0; i < maxRetentionBatchesPerStage; i++ {
			n, err := fn(ctx)
			if err != nil {
				return err
			}
			stageTotal += n
			if n == 0 || n < int64(batch) {
				break
			}
		}
		deleted[stage] = stageTotal
		out.Stages[stage] = stageTotal
		return nil
	}

	if err := runStage("device_telemetry_events_non_critical", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteNonCriticalDeviceEventsBatch(c, db.TelemetryRetentionDeleteNonCriticalDeviceEventsBatchParams{
			ReceivedAt: normalCutoff,
			Limit:      batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	if err := runStage("device_telemetry_events_critical", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteCriticalLinkedDeviceEventsBatch(c, db.TelemetryRetentionDeleteCriticalLinkedDeviceEventsBatchParams{
			ReceivedAt: criticalCutoff,
			Limit:      batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	if err := runStage("critical_telemetry_event_status", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteCriticalTelemetryStatusBatch(c, db.TelemetryRetentionDeleteCriticalTelemetryStatusBatchParams{
			ProcessedAt: critTs,
			Limit:       batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	if err := runStage("machine_check_ins", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteMachineCheckInsBatch(c, db.TelemetryRetentionDeleteMachineCheckInsBatchParams{
			OccurredAt: normalCutoff,
			Limit:      batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	if err := runStage("machine_state_transitions", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteStateTransitionsBatch(c, db.TelemetryRetentionDeleteStateTransitionsBatchParams{
			OccurredAt: normalCutoff,
			Limit:      batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	if err := runStage("machine_incidents_normal", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteIncidentsNormalSeverityBatch(c, db.TelemetryRetentionDeleteIncidentsNormalSeverityBatchParams{
			OpenedAt: normalCutoff,
			Limit:    batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	if err := runStage("machine_incidents_high_critical", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteIncidentsCriticalSeverityBatch(c, db.TelemetryRetentionDeleteIncidentsCriticalSeverityBatchParams{
			OpenedAt: criticalCutoff,
			Limit:    batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	if err := runStage("telemetry_rollups_1m", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteRollupsOneMinuteBatch(c, db.TelemetryRetentionDeleteRollupsOneMinuteBatchParams{
			BucketStart: normalCutoff,
			Limit:       batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	if err := runStage("telemetry_rollups_1h", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteRollupsOneHourBatch(c, db.TelemetryRetentionDeleteRollupsOneHourBatchParams{
			BucketStart: criticalCutoff,
			Limit:       batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	if err := runStage("diagnostic_bundle_manifests", func(c context.Context) (int64, error) {
		return q.TelemetryRetentionDeleteDiagnosticManifestsBatch(c, db.TelemetryRetentionDeleteDiagnosticManifestsBatchParams{
			CreatedAt: criticalCutoff,
			Limit:     batch,
		})
	}); err != nil {
		runErr = err
		return out, err
	}

	return out, nil
}

func telemetryRetentionCandidateCounts(ctx context.Context, q *db.Queries, normalCutoff, criticalCutoff time.Time, critTs pgtype.Timestamptz) (map[string]int64, error) {
	n1, err := q.TelemetryRetentionCountNonCriticalDeviceEvents(ctx, normalCutoff)
	if err != nil {
		return nil, err
	}
	n2, err := q.TelemetryRetentionCountCriticalLinkedDeviceEvents(ctx, criticalCutoff)
	if err != nil {
		return nil, err
	}
	n3, err := q.TelemetryRetentionCountCriticalTelemetryStatusRows(ctx, critTs)
	if err != nil {
		return nil, err
	}
	n4, err := q.TelemetryRetentionCountMachineCheckIns(ctx, normalCutoff)
	if err != nil {
		return nil, err
	}
	n5, err := q.TelemetryRetentionCountStateTransitions(ctx, normalCutoff)
	if err != nil {
		return nil, err
	}
	n6, err := q.TelemetryRetentionCountIncidentsNormalSeverity(ctx, normalCutoff)
	if err != nil {
		return nil, err
	}
	n7, err := q.TelemetryRetentionCountIncidentsCriticalSeverity(ctx, criticalCutoff)
	if err != nil {
		return nil, err
	}
	n8, err := q.TelemetryRetentionCountRollupsOneMinute(ctx, normalCutoff)
	if err != nil {
		return nil, err
	}
	n9, err := q.TelemetryRetentionCountRollupsOneHour(ctx, criticalCutoff)
	if err != nil {
		return nil, err
	}
	n10, err := q.TelemetryRetentionCountDiagnosticManifests(ctx, criticalCutoff)
	if err != nil {
		return nil, err
	}
	return map[string]int64{
		"device_telemetry_events_non_critical": n1,
		"device_telemetry_events_critical":     n2,
		"critical_telemetry_event_status":      n3,
		"machine_check_ins":                    n4,
		"machine_state_transitions":            n5,
		"machine_incidents_normal":             n6,
		"machine_incidents_high_critical":      n7,
		"telemetry_rollups_1m":                 n8,
		"telemetry_rollups_1h":                 n9,
		"diagnostic_bundle_manifests":          n10,
	}, nil
}
