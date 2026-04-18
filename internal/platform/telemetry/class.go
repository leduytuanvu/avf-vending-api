package telemetry

import "strings"

// Class is a high-level telemetry routing bucket (MQTT → NATS JetStream subjects).
type Class string

const (
	ClassHeartbeat              Class = "heartbeat"
	ClassState                  Class = "state"
	ClassMetrics                Class = "metrics"
	ClassIncident               Class = "incident"
	ClassCommandReceipt         Class = "command_receipt"
	ClassDiagnosticBundleReady Class = "diagnostic_bundle_ready"
	ClassUnknown                Class = "unknown"
)

// ClassifyEventType maps device telemetry event_type strings into a Class.
// Convention: explicit prefixes win; otherwise metrics (operational noise bucket).
func ClassifyEventType(eventType string) Class {
	t := strings.TrimSpace(strings.ToLower(eventType))
	switch {
	case t == "":
		return ClassUnknown
	case t == "heartbeat" || t == "ping" || strings.HasPrefix(t, "heartbeat.") || strings.HasPrefix(t, "health."):
		return ClassHeartbeat
	case strings.HasPrefix(t, "incident.") || strings.HasPrefix(t, "alert.") || t == "incident":
		return ClassIncident
	case strings.HasPrefix(t, "diagnostic.") || t == "diagnostic_bundle_ready":
		return ClassDiagnosticBundleReady
	case strings.HasPrefix(t, "state.") || strings.HasPrefix(t, "shadow.") || t == "state":
		return ClassState
	case strings.HasPrefix(t, "metric") || strings.HasPrefix(t, "metrics."):
		return ClassMetrics
	default:
		// High-frequency unknown telemetry must not default to OLTP incident path.
		return ClassMetrics
	}
}
