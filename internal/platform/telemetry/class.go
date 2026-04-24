package telemetry

import "strings"

// Class is a high-level telemetry routing bucket (MQTT → NATS JetStream subjects).
type Class string

const (
	ClassHeartbeat             Class = "heartbeat"
	ClassState                 Class = "state"
	ClassMetrics               Class = "metrics"
	ClassIncident              Class = "incident"
	ClassCommandReceipt        Class = "command_receipt"
	ClassDiagnosticBundleReady Class = "diagnostic_bundle_ready"
	ClassUnknown               Class = "unknown"
)

// Criticality controls backpressure behavior at mqtt-ingest ingress.
type Criticality string

const (
	CriticalityCriticalNoDrop    Criticality = "critical_no_drop"
	CriticalityCompactableLatest Criticality = "compactable_latest"
	CriticalityDroppableMetrics  Criticality = "droppable_metrics"
)

// ClassifyEventType maps device telemetry event_type strings into a Class.
// Convention: explicit prefixes win; otherwise metrics (operational noise bucket).
func ClassifyEventType(eventType string) Class {
	t := strings.TrimSpace(strings.ToLower(eventType))
	switch {
	case t == "":
		return ClassUnknown
	case t == "heartbeat" || t == "ping" || t == "presence" || strings.HasPrefix(t, "heartbeat.") || strings.HasPrefix(t, "health."):
		return ClassHeartbeat
	case t == "state.heartbeat" || strings.HasPrefix(t, "state.heartbeat."):
		return ClassHeartbeat
	case strings.HasPrefix(t, "incident.") || strings.HasPrefix(t, "alert.") || t == "incident":
		return ClassIncident
	case t == "telemetry.incident" || strings.HasPrefix(t, "telemetry.incident."):
		return ClassIncident
	case strings.HasPrefix(t, "diagnostic.") || t == "diagnostic_bundle_ready":
		return ClassDiagnosticBundleReady
	case strings.HasPrefix(t, "shadow.desired"):
		return ClassState
	case strings.HasPrefix(t, "shadow.") && !strings.HasPrefix(t, "shadow.desired"):
		return ClassState
	case strings.HasPrefix(t, "state.") || t == "state":
		return ClassState
	case strings.HasPrefix(t, "metric") || strings.HasPrefix(t, "metrics."):
		return ClassMetrics
	case strings.HasPrefix(t, "telemetry.snapshot") || strings.HasPrefix(t, "events."):
		return ClassMetrics
	default:
		// High-frequency unknown telemetry must not default to OLTP incident path.
		return ClassMetrics
	}
}

func machineCriticalIncident(eventType string) bool {
	t := eventType
	if strings.Contains(t, "indoor") {
		return false
	}
	return strings.Contains(t, "jam") ||
		strings.Contains(t, "door.open") || strings.Contains(t, "door_open") || strings.Contains(t, "door-open") ||
		(strings.Contains(t, ".door") || strings.Contains(t, "/door")) ||
		strings.Contains(t, "motor.fault") || strings.Contains(t, "motor_fault") || strings.Contains(t, "motor-fault") ||
		strings.Contains(t, "temperature.critical") || strings.Contains(t, "temperature_critical") || strings.Contains(t, "temperature-critical")
}

// CriticalityForEventType maps device event types into backpressure handling categories.
func CriticalityForEventType(eventType string) Criticality {
	t := strings.TrimSpace(strings.ToLower(eventType))
	switch {
	case t == "":
		return CriticalityDroppableMetrics
	case t == "heartbeat" || t == "ping" || t == "presence" || t == "state.heartbeat" || strings.HasPrefix(t, "heartbeat.") || strings.HasPrefix(t, "health.") || strings.HasPrefix(t, "state.heartbeat."):
		return CriticalityDroppableMetrics
	case strings.HasPrefix(t, "metric") || strings.HasPrefix(t, "metrics.") || strings.HasPrefix(t, "debug.") || strings.HasPrefix(t, "noise."):
		return CriticalityDroppableMetrics
	case strings.HasPrefix(t, "shadow.") || t == "state" || strings.HasPrefix(t, "state.") || strings.HasPrefix(t, "telemetry.snapshot"):
		return CriticalityCompactableLatest
	case strings.HasPrefix(t, "webhook."):
		// Payment / cashless / refund webhooks are financial; other webhooks stay low priority.
		if strings.Contains(t, "payment") || strings.Contains(t, "cashless") || strings.Contains(t, "refund") {
			return CriticalityCriticalNoDrop
		}
		return CriticalityDroppableMetrics
	case t == "events.vend" || strings.HasPrefix(t, "vend.") ||
		strings.HasPrefix(t, "payment.") || strings.HasPrefix(t, "payments.") ||
		strings.HasPrefix(t, "cashless.") || strings.HasPrefix(t, "refund."):
		return CriticalityCriticalNoDrop
	case t == "events.cash" || strings.HasPrefix(t, "cash.") || strings.Contains(t, "cash.inserted") || strings.Contains(t, "cash.payout") || strings.Contains(t, "cash.collection"):
		return CriticalityCriticalNoDrop
	case t == "events.inventory" || strings.HasPrefix(t, "inventory.") || strings.Contains(t, "inventory.delta") || strings.Contains(t, "inventory.refill") || strings.Contains(t, "inventory.adjust"):
		return CriticalityCriticalNoDrop
	case strings.HasPrefix(t, "commands.ack") || strings.HasPrefix(t, "command.ack") || strings.HasPrefix(t, "config.ack"):
		return CriticalityCriticalNoDrop
	case strings.HasPrefix(t, "incident.") || strings.HasPrefix(t, "alert.") || t == "incident" || t == "telemetry.incident" || strings.HasPrefix(t, "telemetry.incident."):
		if machineCriticalIncident(t) {
			return CriticalityCriticalNoDrop
		}
		return CriticalityCompactableLatest
	case strings.HasPrefix(t, "diagnostic.") || t == "diagnostic_bundle_ready":
		return CriticalityCompactableLatest
	default:
		return CriticalityDroppableMetrics
	}
}
