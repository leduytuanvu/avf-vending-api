package mqtt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// TopicLayout selects how machine-scoped topics are joined to MQTT_TOPIC_PREFIX.
type TopicLayout string

const (
	// TopicLayoutLegacy uses {prefix}/{machineId}/... (default; existing deployments).
	TopicLayoutLegacy TopicLayout = "legacy"
	// TopicLayoutEnterprise uses {prefix}/machines/{machineId}/... (strict enterprise tree).
	TopicLayoutEnterprise TopicLayout = "enterprise"
)

// NormalizeTopicLayout coerces env strings to a supported layout (default legacy).
func NormalizeTopicLayout(raw string) TopicLayout {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(TopicLayoutEnterprise):
		return TopicLayoutEnterprise
	default:
		return TopicLayoutLegacy
	}
}

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

func validateTopicSegment(name, v string) error {
	s := strings.TrimSpace(v)
	if s == "" {
		return fmt.Errorf("%s must be non-empty", name)
	}
	if strings.ContainsAny(s, "+#") {
		return fmt.Errorf("%s must not contain MQTT wildcards", name)
	}
	for _, part := range strings.Split(s, "/") {
		if strings.TrimSpace(part) == "" {
			return fmt.Errorf("%s must not contain empty path segments", name)
		}
		if part == "." || part == ".." {
			return fmt.Errorf("%s must not contain relative path segments", name)
		}
	}
	return nil
}

// ValidateTopicPrefix rejects wildcard and malformed prefixes before they can broaden broker ACLs.
func ValidateTopicPrefix(prefix string) error {
	return validateTopicSegment("topic prefix", NormalizeDeviceTopicPrefix(prefix))
}

// ValidateRelativeTopic rejects wildcard and malformed relative topic paths.
func ValidateRelativeTopic(rel string) error {
	return validateTopicSegment("relative topic", strings.Trim(rel, "/"))
}

// DeviceTopicStrict joins prefix, machine id, and relative path after validating all ACL-significant segments.
func DeviceTopicStrict(prefix string, machineID uuid.UUID, rel string) (string, error) {
	if machineID == uuid.Nil {
		return "", errors.New("machine id must be non-empty")
	}
	p := NormalizeDeviceTopicPrefix(prefix)
	r := strings.Trim(rel, "/")
	if err := ValidateTopicPrefix(p); err != nil {
		return "", err
	}
	if err := ValidateRelativeTopic(r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/%s", p, machineID.String(), r), nil
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

// OutboundEnterpriseCommandTopic is the enterprise outbound command channel:
// {prefix}/machines/{machineId}/commands (no /dispatch segment).
func OutboundEnterpriseCommandTopic(prefix string, machineID uuid.UUID) string {
	p := NormalizeDeviceTopicPrefix(prefix)
	return fmt.Sprintf("%s/machines/%s/commands", p, machineID.String())
}

// OutboundCommandPublishTopic selects legacy dispatch vs enterprise commands topic.
func OutboundCommandPublishTopic(layout TopicLayout, prefix string, machineID uuid.UUID) string {
	if NormalizeTopicLayout(string(layout)) == TopicLayoutEnterprise {
		return OutboundEnterpriseCommandTopic(prefix, machineID)
	}
	return OutboundCommandDispatchTopic(prefix, machineID)
}

// OutboundCommandPublishTopicStrict is the publish-side variant used by production clients.
func OutboundCommandPublishTopicStrict(layout TopicLayout, prefix string, machineID uuid.UUID) (string, error) {
	if NormalizeTopicLayout(string(layout)) == TopicLayoutEnterprise {
		if machineID == uuid.Nil {
			return "", errors.New("machine id must be non-empty")
		}
		p := NormalizeDeviceTopicPrefix(prefix)
		if err := ValidateTopicPrefix(p); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/machines/%s/commands", p, machineID.String()), nil
	}
	return DeviceTopicStrict(prefix, machineID, RelTopicCommandsDispatch)
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

// InboundEnterpriseDeviceTopicPatterns subscribes under {prefix}/machines/+ /... per enterprise contract.
func InboundEnterpriseDeviceTopicPatterns(prefix string) []string {
	base := NormalizeDeviceTopicPrefix(prefix) + "/machines"
	return []string{
		fmt.Sprintf("%s/+/telemetry", base),
		fmt.Sprintf("%s/+/presence", base),
		fmt.Sprintf("%s/+/state/heartbeat", base),
		fmt.Sprintf("%s/+/telemetry/snapshot", base),
		fmt.Sprintf("%s/+/telemetry/incident", base),
		fmt.Sprintf("%s/+/events/vend", base),
		fmt.Sprintf("%s/+/events/cash", base),
		fmt.Sprintf("%s/+/events/inventory", base),
		fmt.Sprintf("%s/+/events", base),
		fmt.Sprintf("%s/+/shadow/reported", base),
		fmt.Sprintf("%s/+/shadow/desired", base),
		fmt.Sprintf("%s/+/commands/receipt", base),
		fmt.Sprintf("%s/+/commands/ack", base),
	}
}

// InboundTopicPatterns selects pattern set for the configured layout.
func InboundTopicPatterns(layout TopicLayout, prefix string) []string {
	if NormalizeTopicLayout(string(layout)) == TopicLayoutEnterprise {
		return InboundEnterpriseDeviceTopicPatterns(prefix)
	}
	return InboundDeviceTopicPatterns(prefix)
}
