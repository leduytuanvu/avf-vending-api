package telemetryapp

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	telemetryConsumerLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "telemetry_consumer",
		Name:      "lag",
		Help:      "JetStream consumer NumPending (unacked backlog at broker) per telemetry durable.",
	}, []string{"stream", "durable"})

	telemetryProjectionBacklog = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "telemetry_projection",
		Name:      "backlog",
		Help:      "In-flight telemetry messages being processed (acquired projection semaphore slots).",
	})

	telemetryProjectionFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_projection",
		Name:      "failures_total",
		Help:      "Telemetry projection failures by reason.",
	}, []string{"reason"})

	telemetryProjectionBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "avf",
		Subsystem: "telemetry_projection",
		Name:      "batch_size",
		Help:      "Number of JetStream messages fetched per pull batch.",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10),
	})

	telemetryProjectionFlushSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "avf",
		Subsystem: "telemetry_projection",
		Name:      "flush_seconds",
		Help:      "Wall time to process one pull batch (fetch through acks).",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 16),
	})

	telemetryDuplicateTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry",
		Name:      "duplicate_total",
		Help:      "Telemetry messages treated as duplicates at projection (skipped apply).",
	}, []string{"reason"})

	telemetryIdempotencyConflictTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry",
		Name:      "idempotency_conflict_total",
		Help:      "Telemetry envelopes reused idempotency_key with different payload hash (skipped apply).",
	})

	telemetryProjectionFailConsecutiveMax = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "telemetry_projection",
		Name:      "db_fail_consecutive_max",
		Help:      "Max consecutive handler/Nak failures seen across telemetry durables since last success per durable.",
	})
)

func recordTelemetryDuplicate(reason string) {
	r := strings.TrimSpace(reason)
	if r == "" {
		r = "unknown"
	}
	telemetryDuplicateTotal.WithLabelValues(r).Inc()
}
