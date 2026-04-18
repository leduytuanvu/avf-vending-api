// Package reconcilerprom implements app/background.ReconcilerTelemetry for Prometheus.
// It is imported only from cmd/reconciler so worker does not register these series.
package reconcilerprom

import (
	"time"

	appbackground "github.com/avf/avf-vending-api/internal/app/background"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cycleDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "avf",
		Subsystem: "reconciler",
		Name:      "cycle_duration_seconds",
		Help:      "Wall time for one reconciler job pass (single tick cycle).",
		Buckets:   prometheus.ExponentialBuckets(0.01, 2, 20),
	}, []string{"reconciler_job"})

	cycleCompletions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "reconciler",
		Name:      "cycle_completions_total",
		Help:      "Reconciler periodic cycle outcomes by reconciler_job and result (ok, canceled, cycle_deadline_exceeded, error).",
	}, []string{"reconciler_job", "result"})

	rowsSelected = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "reconciler",
		Name:      "rows_selected_total",
		Help:      "Rows or payments listed in a reconciler job summary (added per tick).",
	}, []string{"reconciler_job"})

	jobFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "reconciler",
		Name:      "job_failures_total",
		Help:      "Failures reported in reconciler_job_summary.failed (per tick).",
	}, []string{"reconciler_job"})

	batchLimitHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "reconciler",
		Name:      "job_batch_limit_hit_total",
		Help:      "Times a job hit its batch limit (at_batch_limit=true).",
	}, []string{"reconciler_job"})
)

type telemetry struct{}

// New returns a ReconcilerTelemetry backed by Prometheus default registry.
func New() appbackground.ReconcilerTelemetry {
	return telemetry{}
}

func (telemetry) CycleEnd(job string, duration time.Duration, _ error, result string) {
	cycleDuration.WithLabelValues(job).Observe(duration.Seconds())
	cycleCompletions.WithLabelValues(job, result).Inc()
}

func (telemetry) JobSummary(job string, selected, failed int, atBatchLimit bool) {
	if selected > 0 {
		rowsSelected.WithLabelValues(job).Add(float64(selected))
	}
	if failed > 0 {
		jobFailures.WithLabelValues(job).Add(float64(failed))
	}
	if atBatchLimit {
		batchLimitHits.WithLabelValues(job).Inc()
	}
}
