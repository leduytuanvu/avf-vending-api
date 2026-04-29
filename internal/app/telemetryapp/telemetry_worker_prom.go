package telemetryapp

import (
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
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

	machineConnectivity = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "machine",
		Name:      "connectivity_total",
		Help:      "Machine fleet connectivity count by status (online/offline) from the latest fleet snapshot collector.",
	}, []string{"status"})

	machineLastSeenAge = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "avf",
		Subsystem: "machine",
		Name:      "last_seen_age_seconds",
		Help:      "Age of machine last-seen timestamps observed while processing heartbeat projections.",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 16),
	})
)

func recordTelemetryDuplicate(reason string) {
	r := strings.TrimSpace(reason)
	if r == "" {
		r = "unknown"
	}
	telemetryDuplicateTotal.WithLabelValues(r).Inc()
}

// SetMachineConnectivityCounts updates the online/offline fleet snapshot gauges.
func SetMachineConnectivityCounts(online, offline int64) {
	if online < 0 {
		online = 0
	}
	if offline < 0 {
		offline = 0
	}
	machineConnectivity.WithLabelValues("online").Set(float64(online))
	machineConnectivity.WithLabelValues("offline").Set(float64(offline))
}

func observeMachineLastSeenAge(at time.Time) {
	if at.IsZero() {
		return
	}
	age := time.Since(at.UTC())
	if age < 0 {
		age = 0
	}
	machineLastSeenAge.Observe(age.Seconds())
	productionmetrics.ObserveMachineLastSeenAge(age)
}
