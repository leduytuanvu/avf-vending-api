package telemetryapp

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestTelemetryJetStreamMetrics_register(t *testing.T) {
	t.Parallel()
	telemetryConsumerLag.WithLabelValues("AVF_TELEMETRY_METRICS", "avf-w-telemetry-metrics").Set(3)
	telemetryProjectionFailures.WithLabelValues("fetch_err").Inc()
	if _, err := prometheus.DefaultGatherer.Gather(); err != nil {
		t.Fatal(err)
	}
}
