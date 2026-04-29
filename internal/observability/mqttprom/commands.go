package mqttprom

import (
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	commandDispatchQueued = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "dispatch_queued_total",
		Help:      "New machine_command_attempts rows created in pending state for MQTT dispatch.",
	})
	commandDispatchPublished = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "dispatch_published_total",
		Help:      "MQTT command attempts transitioned to sent after a successful broker publish.",
	})
	commandAckLatencySeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "ack_latency_seconds",
		Help:      "Latency from machine_command_attempts.sent_at to device command receipt completing the attempt.",
		Buckets:   prometheus.ExponentialBuckets(0.05, 2, 16),
	})
	commandAttemptsExpired = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "attempts_expired_total",
		Help:      "Command attempts marked expired when command_ledger.timeout_at passed while still sent.",
	})
	commandAckRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "ack_rejected_total",
		Help:      "Device command ACK/receipt messages rejected at persistence (deadline, unknown sequence, etc.).",
	}, []string{"reason"})
	commandAckConflict = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "ack_conflict_total",
		Help:      "Conflicting command receipts for an already-terminal attempt (recorded as enterprise audit).",
	})
	commandAckDuplicate = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "ack_duplicate_total",
		Help:      "Duplicate command ACK/receipt messages ignored idempotently.",
	})
	commandAckDeadlinesExceeded = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "ack_deadline_exceeded_total",
		Help:      "machine_command_attempts rows marked ack_timeout when ack_deadline_at passed.",
	})
	commandDispatchRefused = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "dispatch_refused_total",
		Help:      "Dispatch attempts refused before persistence (e.g. max_dispatch_attempts ceiling).",
	}, []string{"reason"})
	commandAckTimeout = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "ack_timeout_total",
		Help:      "Command ACK/receipt messages rejected because the ledger or attempt acceptance window expired.",
	}, []string{"reason"})
	commandStateTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "mqtt_command",
		Name:      "state_total",
		Help:      "Machine command state transitions observed by the command ledger and ACK processor.",
	}, []string{"state"})
)

// RecordCommandDispatchQueued increments dispatch_queued_total.
func RecordCommandDispatchQueued() {
	commandDispatchQueued.Inc()
	commandStateTotal.WithLabelValues("pending").Inc()
}

// RecordCommandDispatchPublished increments dispatch_published_total.
func RecordCommandDispatchPublished() {
	commandDispatchPublished.Inc()
	commandStateTotal.WithLabelValues("published").Inc()
	productionmetrics.RecordCommandDispatched()
}

// ObserveCommandAckLatency records ack_latency_seconds.
func ObserveCommandAckLatency(d time.Duration) {
	if d < 0 {
		d = 0
	}
	commandAckLatencySeconds.Observe(d.Seconds())
	commandStateTotal.WithLabelValues("acked").Inc()
	productionmetrics.ObserveCommandAckLatency(d)
	productionmetrics.RecordCommandAcked()
}

// AddCommandAttemptsExpired adds N to attempts_expired_total.
func AddCommandAttemptsExpired(n int64) {
	if n <= 0 {
		return
	}
	commandAttemptsExpired.Add(float64(n))
	commandStateTotal.WithLabelValues("timeout").Add(float64(n))
	productionmetrics.AddCommandsExpired(n)
}

// RecordCommandAckRejected increments ack_rejected_total{reason}.
func RecordCommandAckRejected(reason string) {
	r := strings.TrimSpace(reason)
	if r == "" {
		r = "unknown"
	}
	commandAckRejected.WithLabelValues(r).Inc()
	commandStateTotal.WithLabelValues("failed").Inc()
	productionmetrics.RecordCommandFailed(r)
}

// RecordCommandAckConflict increments ack_conflict_total.
func RecordCommandAckConflict() {
	commandAckConflict.Inc()
	productionmetrics.RecordCommandFailed("conflict")
}

// AddMachineCommandAckDeadlinesExceeded adds N to ack_deadline_exceeded_total.
func AddMachineCommandAckDeadlinesExceeded(n int64) {
	if n <= 0 {
		return
	}
	commandAckDeadlinesExceeded.Add(float64(n))
	commandStateTotal.WithLabelValues("timed_out").Add(float64(n))
	productionmetrics.AddCommandsFailed("ack_deadline_exceeded", n)
}

// RecordMQTTDispatchRefused increments dispatch_refused_total{reason}.
func RecordMQTTDispatchRefused(reason string) {
	r := strings.TrimSpace(reason)
	if r == "" {
		r = "unknown"
	}
	commandDispatchRefused.WithLabelValues(r).Inc()
	productionmetrics.RecordCommandRetry(r)
}

// RecordCommandAckDuplicate increments ack_duplicate_total.
func RecordCommandAckDuplicate() {
	commandAckDuplicate.Inc()
}

// RecordCommandAckTimeout increments ack_timeout_total{reason}.
func RecordCommandAckTimeout(reason string) {
	r := strings.TrimSpace(reason)
	if r == "" {
		r = "unknown"
	}
	commandAckTimeout.WithLabelValues(r).Inc()
	commandStateTotal.WithLabelValues("timeout").Inc()
}
