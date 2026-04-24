package telemetryapp

import (
	"strings"

	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	telemetryIngestReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_ingest",
		Name:      "received_total",
		Help:      "MQTT device messages accepted into the bounded mqtt-ingest pipeline, by channel.",
	}, []string{"channel"})

	telemetryIngestRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_ingest",
		Name:      "rejected_total",
		Help:      "Messages rejected before downstream ingest (validation, queue timeout), by reason.",
	}, []string{"reason"})

	telemetryIngestDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_ingest",
		Name:      "dropped_total",
		Help:      "Messages dropped under backpressure (bounded queue full), by reason.",
	}, []string{"reason"})

	telemetryIngestPublishFailures = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_ingest",
		Name:      "publish_failures_total",
		Help:      "JetStream publish failures from the NATS telemetry bridge.",
	})

	telemetryIngestRateLimited = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_ingest",
		Name:      "rate_limited_total",
		Help:      "Per-machine token bucket denials at mqtt-ingest ingress.",
	})

	telemetryIngestCriticalMissingIdentity = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "telemetry_ingest",
		Name:      "critical_missing_identity_total",
		Help:      "Critical telemetry rejected at the JetStream bridge: no dedupe_key, event_id, or boot_id+seq_no.",
	})

	telemetryIngestQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "avf",
		Subsystem: "telemetry_ingest",
		Name:      "queue_depth",
		Help:      "Buffered jobs waiting in the bounded mqtt-ingest work queue.",
	})

	telemetryIngestPayloadBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "avf",
		Subsystem: "telemetry_ingest",
		Name:      "payload_bytes",
		Help:      "Observed MQTT payload sizes for successful Dispatch outcomes (bytes).",
		Buckets:   prometheus.ExponentialBuckets(256, 2, 14),
	})

	dispatchTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_ingest",
		Name:      "dispatch_total",
		Help:      "MQTT messages passed to Dispatch (success or error), by channel kind.",
	}, []string{"kind", "result"})
)

func topicKind(topic string) string {
	switch {
	case strings.Contains(topic, "/telemetry"):
		return "telemetry"
	case strings.Contains(topic, "/shadow/reported"):
		return "shadow_reported"
	case strings.Contains(topic, "/commands/receipt"):
		return "command_receipt"
	default:
		return "other"
	}
}

// RecordTelemetryReceived increments telemetry_ingest_received_total{channel}.
func RecordTelemetryReceived(channel string) {
	ch := strings.TrimSpace(channel)
	if ch == "" {
		ch = "unknown"
	}
	telemetryIngestReceived.WithLabelValues(ch).Inc()
}

// RecordTelemetryRejected increments telemetry_ingest_rejected_total{reason}.
func RecordTelemetryRejected(reason string) {
	r := strings.TrimSpace(reason)
	if r == "" {
		r = "unknown"
	}
	telemetryIngestRejected.WithLabelValues(r).Inc()
}

// RecordTelemetryCriticalMissingIdentity increments telemetry_ingest_critical_missing_identity_total.
func RecordTelemetryCriticalMissingIdentity() {
	telemetryIngestCriticalMissingIdentity.Inc()
}

// RecordTelemetryDropped increments telemetry_ingest_dropped_total{reason}.
func RecordTelemetryDropped(reason string) {
	r := strings.TrimSpace(reason)
	if r == "" {
		r = "unknown"
	}
	telemetryIngestDropped.WithLabelValues(r).Inc()
}

// RecordTelemetryPublishFailure increments telemetry_ingest_publish_failures_total.
func RecordTelemetryPublishFailure() {
	telemetryIngestPublishFailures.Inc()
}

// RecordTelemetryRateLimited increments telemetry_ingest_rate_limited_total.
func RecordTelemetryRateLimited() {
	telemetryIngestRateLimited.Inc()
}

// SetTelemetryQueueDepth sets telemetry_ingest_queue_depth.
func SetTelemetryQueueDepth(depth float64) {
	telemetryIngestQueueDepth.Set(depth)
}

// ObserveTelemetryPayloadBytes records telemetry_ingest_payload_bytes.
func ObserveTelemetryPayloadBytes(n int) {
	if n < 0 {
		n = 0
	}
	telemetryIngestPayloadBytes.Observe(float64(n))
}

// NewIngestHooks returns hooks for platform/mqtt.Subscriber (Prometheus for cmd/mqtt-ingest).
func NewIngestHooks() *platformmqtt.IngestHooks {
	return &platformmqtt.IngestHooks{
		OnDispatchOutcome: func(success bool, topic string, payloadBytes int) {
			k := topicKind(topic)
			r := "error"
			if success {
				r = "ok"
			}
			dispatchTotal.WithLabelValues(k, r).Inc()
			if success {
				ObserveTelemetryPayloadBytes(payloadBytes)
			}
		},
		OnIngressRejected: func(topic string, reason string, payloadBytes int) {
			RecordTelemetryRejected(reason)
			ObserveTelemetryPayloadBytes(payloadBytes)
		},
	}
}
