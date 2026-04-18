// Package mqttingestprom registers Prometheus counters for MQTT device ingest (cmd/mqtt-ingest only).
package mqttingestprom

import (
	"strings"

	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var dispatchTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "mqtt_ingest",
	Name:      "dispatch_total",
	Help:      "MQTT messages passed to Dispatch (success or error), by channel kind.",
}, []string{"kind", "result"})

// topicKind maps full MQTT topic to a low-cardinality label (telemetry | shadow_reported | command_receipt | other).
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

// NewIngestHooks returns hooks for platform/mqtt.Subscriber that record dispatch outcomes.
func NewIngestHooks() *platformmqtt.IngestHooks {
	return &platformmqtt.IngestHooks{
		OnDispatchOutcome: func(success bool, topic string, _ int) {
			k := topicKind(topic)
			r := "error"
			if success {
				r = "ok"
			}
			dispatchTotal.WithLabelValues(k, r).Inc()
		},
	}
}
