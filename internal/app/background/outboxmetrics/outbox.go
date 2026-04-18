// Package outboxmetrics registers Prometheus series for cmd/worker outbox dispatch only.
// It lives in a subpackage so cmd/reconciler does not link these collectors.
package outboxmetrics

import (
	"time"

	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	pendingTotalGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "pending_total",
		Help:      "Unpublished outbox rows not dead-lettered (matches pipeline snapshot).",
	})
	pendingDueNowGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "pending_due_now_total",
		Help:      "Subset of pending rows eligible for dispatch now (next_publish_after cleared or due).",
	})
	deadLetteredTotalGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "dead_lettered_total",
		Help:      "Rows with dead_lettered_at set (terminal quarantine).",
	})
	maxPendingAttemptsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "max_publish_attempts",
		Help:      "Max publish_attempt_count among pending rows (retry pressure signal).",
	})
	oldestPendingAgeSecondsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "oldest_pending_age_seconds",
		Help:      "Seconds since created_at of oldest pending row; 0 if none.",
	})
	publishSuccessLag = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "publish_success_lag_seconds",
		Help:      "Wall time from outbox created_at to successful publish+mark in this worker.",
		Buckets:   prometheus.ExponentialBuckets(0.05, 2, 18),
	})
	dispatchPublishFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "dispatch_publish_failed_total",
		Help:      "JetStream publish failures recorded in a dispatch tick.",
	})
	dispatchPublished = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "dispatch_published_total",
		Help:      "Successful publishes that updated published_at in a dispatch tick.",
	})
	dispatchDeadLettered = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "dispatch_dead_lettered_total",
		Help:      "Rows quarantined in Postgres after exhausting publish attempts in a dispatch tick.",
	})
	dispatchDLQPublishFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "worker_outbox",
		Name:      "dispatch_dlq_publish_failed_total",
		Help:      "Failures publishing to JetStream DLQ after Postgres dead-letter (row still quarantined).",
	})
)

// ObservePipelineGauges updates point-in-time gauges from the pipeline stats query.
func ObservePipelineGauges(now time.Time, pl domainreliability.OutboxPipelineStats) {
	pendingTotalGauge.Set(float64(pl.PendingTotal))
	pendingDueNowGauge.Set(float64(pl.PendingDueNow))
	deadLetteredTotalGauge.Set(float64(pl.DeadLetteredTotal))
	maxPendingAttemptsGauge.Set(float64(pl.MaxPendingAttempts))
	if pl.OldestPendingCreatedAt != nil {
		oldestPendingAgeSecondsGauge.Set(now.Sub(*pl.OldestPendingCreatedAt).Seconds())
	} else {
		oldestPendingAgeSecondsGauge.Set(0)
	}
}

// IncDispatchPublishFailed records a JetStream publish failure for one outbox row.
func IncDispatchPublishFailed() { dispatchPublishFailed.Inc() }

// IncDispatchPublished records a successful publish+mark for one outbox row.
func IncDispatchPublished() { dispatchPublished.Inc() }

// IncDispatchDeadLettered records a row moved to dead-letter state in Postgres.
func IncDispatchDeadLettered() { dispatchDeadLettered.Inc() }

// IncDispatchDLQPublishFailed records failure to publish the companion DLQ message.
func IncDispatchDLQPublishFailed() { dispatchDLQPublishFailed.Inc() }

// ObservePublishSuccessLag records seconds from outbox created_at to successful mark.
func ObservePublishSuccessLag(seconds float64) { publishSuccessLag.Observe(seconds) }
