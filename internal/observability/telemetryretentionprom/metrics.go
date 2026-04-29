// Package telemetryretentionprom registers Prometheus metrics for cmd/worker telemetry retention cleanup.
package telemetryretentionprom

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cleanupDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "avf",
		Subsystem: "telemetry_retention",
		Name:      "cleanup_duration_seconds",
		Help:      "Wall time for one telemetry retention job pass (single worker tick cycle).",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 22),
	})

	rowsDeleted = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_retention",
		Name:      "rows_deleted_total",
		Help:      "Rows deleted by telemetry retention batch deletes (labeled by stage).",
	}, []string{"stage"})

	lastSuccessUnix = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "telemetry_retention",
		Name:      "last_success_unix_seconds",
		Help:      "Unix timestamp (seconds) of the last successful telemetry retention cycle that performed deletes (dry-run excluded).",
	})

	failures = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_retention",
		Name:      "failures_total",
		Help:      "Telemetry retention cycles that returned an error.",
	})

	dryRunCycles = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_retention",
		Name:      "dry_run_cycles_total",
		Help:      "Telemetry retention ticks skipped deletes because TELEMETRY_CLEANUP_DRY_RUN=true.",
	})
)

// ObserveRun records duration, per-stage row counts, success gauge, and failures (Prometheus default registry).
func ObserveRun(start time.Time, deleted map[string]int64, dryRun bool, err error) {
	d := time.Since(start).Seconds()
	cleanupDuration.Observe(d)

	if dryRun {
		dryRunCycles.Inc()
		return
	}

	for stage, n := range deleted {
		if n <= 0 {
			continue
		}
		rowsDeleted.WithLabelValues(stage).Add(float64(n))
	}

	if err != nil {
		failures.Inc()
		return
	}

	lastSuccessUnix.Set(float64(time.Now().UTC().Unix()))
}
