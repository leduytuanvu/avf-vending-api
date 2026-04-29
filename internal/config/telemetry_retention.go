package config

import (
	"errors"
	"fmt"
	"time"
)

// TelemetryDataRetentionConfig controls Postgres telemetry/evidence pruning in cmd/worker (not OLTP finance rows).
type TelemetryDataRetentionConfig struct {
	// RetentionDays is TELEMETRY_RETENTION_DAYS — horizon for standard telemetry projections and non-critical device rows.
	RetentionDays int
	// CriticalRetentionDays is TELEMETRY_CRITICAL_RETENTION_DAYS — horizon for critical telemetry evidence (linked device rows,
	// high-severity incidents, coarse rollups, diagnostics metadata).
	CriticalRetentionDays int
	// CleanupEnabled is TELEMETRY_CLEANUP_ENABLED — when false, worker does not schedule telemetry_retention ticks.
	CleanupEnabled bool
	// CleanupBatchSize is TELEMETRY_CLEANUP_BATCH_SIZE — max rows deleted per table per batch DELETE.
	CleanupBatchSize int32
	// CleanupDryRun is TELEMETRY_CLEANUP_DRY_RUN — when true, retention scans run without deleting rows (metrics reflect zero deletes).
	CleanupDryRun bool
}

func loadTelemetryDataRetentionConfig(appEnv AppEnvironment) TelemetryDataRetentionConfig {
	cleanupDefault := appEnv != AppEnvDevelopment && appEnv != AppEnvTest
	return TelemetryDataRetentionConfig{
		RetentionDays:         getenvInt("TELEMETRY_RETENTION_DAYS", 30),
		CriticalRetentionDays: getenvInt("TELEMETRY_CRITICAL_RETENTION_DAYS", 365),
		CleanupEnabled:        getenvBool("TELEMETRY_CLEANUP_ENABLED", cleanupDefault),
		CleanupBatchSize:      int32(getenvInt("TELEMETRY_CLEANUP_BATCH_SIZE", 500)),
		CleanupDryRun:         getenvBool("TELEMETRY_CLEANUP_DRY_RUN", false),
	}
}

func (c TelemetryDataRetentionConfig) validate() error {
	if c.RetentionDays <= 0 {
		return errors.New("config: TELEMETRY_RETENTION_DAYS must be > 0")
	}
	if c.CriticalRetentionDays <= 0 {
		return errors.New("config: TELEMETRY_CRITICAL_RETENTION_DAYS must be > 0")
	}
	if c.CriticalRetentionDays < c.RetentionDays {
		return fmt.Errorf("config: TELEMETRY_CRITICAL_RETENTION_DAYS (%d) must be >= TELEMETRY_RETENTION_DAYS (%d)",
			c.CriticalRetentionDays, c.RetentionDays)
	}
	if c.CleanupBatchSize <= 0 {
		return errors.New("config: TELEMETRY_CLEANUP_BATCH_SIZE must be > 0")
	}
	if c.CleanupBatchSize > 50000 {
		return errors.New("config: TELEMETRY_CLEANUP_BATCH_SIZE must be <= 50000")
	}
	return nil
}

// RetentionCutoffs returns UTC cutoff times from now for normal vs critical horizons.
func (c TelemetryDataRetentionConfig) RetentionCutoffs(now time.Time) (normalCutoff, criticalCutoff time.Time) {
	utc := now.UTC()
	return utc.Add(-time.Duration(c.RetentionDays) * 24 * time.Hour),
		utc.Add(-time.Duration(c.CriticalRetentionDays) * 24 * time.Hour)
}
