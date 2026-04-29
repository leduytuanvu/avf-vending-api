package telemetryapp

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMQTTIngestMetricsRegisterAndGather(t *testing.T) {
	t.Parallel()
	RecordTelemetryReceived("telemetry")
	RecordTelemetryDropped("droppable_queue_full")
	RecordTelemetryRejected("payload_too_large")
	h := NewIngestHooks()
	h.OnDispatchOutcome(true, "p/m/c/telemetry", 100)
	h.OnIngressRejected("p/m/c/telemetry", "handler_error", 10)
	SetTelemetryQueueDepth(3)
	if _, err := prometheus.DefaultGatherer.Gather(); err != nil {
		t.Fatal(err)
	}
}
