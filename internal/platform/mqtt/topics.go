package mqtt

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Canonical relative topic tails under {prefix}/{machineId}/...
const (
	RelTopicPresence          = "presence"
	RelTopicStateHeartbeat    = "state/heartbeat"
	RelTopicTelemetrySnapshot = "telemetry/snapshot"
	RelTopicTelemetryIncident = "telemetry/incident"
	RelTopicEventsVend        = "events/vend"
	RelTopicEventsCash        = "events/cash"
	RelTopicEventsInventory   = "events/inventory"
	RelTopicCommandsDown      = "commands/down"
	RelTopicCommandsAck       = "commands/ack"
	RelTopicCommandsReceipt   = "commands/receipt"
	RelTopicCommandsDispatch  = "commands/dispatch"
	RelTopicShadowDesired     = "shadow/desired"
	RelTopicShadowReported    = "shadow/reported"
	RelTopicTelemetryLegacy   = "telemetry"
)

// NormalizeDeviceTopicPrefix trims space and a trailing slash from the configured MQTT prefix.
func NormalizeDeviceTopicPrefix(prefix string) string {
	return strings.TrimSuffix(strings.TrimSpace(prefix), "/")
}

// DeviceTopic joins prefix, machine id, and a relative channel path.
func DeviceTopic(prefix string, machineID uuid.UUID, rel string) string {
	p := NormalizeDeviceTopicPrefix(prefix)
	rel = strings.Trim(rel, "/")
	return fmt.Sprintf("%s/%s/%s", p, machineID.String(), rel)
}

// OutboundCommandDispatchTopic is the MQTT topic the API publishes remote commands to.
// Convention mirrors inbound channels under TopicPrefix: {prefix}/{machineId}/commands/dispatch
func OutboundCommandDispatchTopic(prefix string, machineID uuid.UUID) string {
	return DeviceTopic(prefix, machineID, RelTopicCommandsDispatch)
}

// OutboundCommandDownTopic is the newer alias for the outbound command topic (same semantics as dispatch).
func OutboundCommandDownTopic(prefix string, machineID uuid.UUID) string {
	return DeviceTopic(prefix, machineID, RelTopicCommandsDown)
}

// InboundDeviceTopicPatterns returns MQTT subscribe patterns for device-originated channels.
// Ops / mqtt-ingest should subscribe to these (plus any broker-specific overlays).
func InboundDeviceTopicPatterns(prefix string) []string {
	p := NormalizeDeviceTopicPrefix(prefix)
	return []string{
		fmt.Sprintf("%s/+/telemetry", p),
		fmt.Sprintf("%s/+/presence", p),
		fmt.Sprintf("%s/+/state/heartbeat", p),
		fmt.Sprintf("%s/+/telemetry/snapshot", p),
		fmt.Sprintf("%s/+/telemetry/incident", p),
		fmt.Sprintf("%s/+/events/vend", p),
		fmt.Sprintf("%s/+/events/cash", p),
		fmt.Sprintf("%s/+/events/inventory", p),
		fmt.Sprintf("%s/+/shadow/reported", p),
		fmt.Sprintf("%s/+/shadow/desired", p),
		fmt.Sprintf("%s/+/commands/receipt", p),
		fmt.Sprintf("%s/+/commands/ack", p),
	}
}
