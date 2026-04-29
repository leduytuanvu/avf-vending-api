package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/observability/mqttprom"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var mqttInvalidTopicsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "mqtt",
	Name:      "invalid_topics_total",
	Help:      "MQTT messages rejected at ingest parse/validation (topic shape or envelope machine_id vs topic).",
}, []string{"reason"})

var allowedReceiptStatuses = map[string]struct{}{
	"acked": {}, "nacked": {}, "failed": {}, "timeout": {},
}

var receiptStatusAliases = map[string]string{
	"ack":     "acked",
	"success": "acked",
	"ok":      "acked",
}

type deviceWire struct {
	SchemaVersion     int             `json:"schema_version"`
	EventID           string          `json:"event_id"`
	MachineID         *uuid.UUID      `json:"machine_id"`
	BootID            *uuid.UUID      `json:"boot_id"`
	SeqNo             *int64          `json:"seq_no"`
	OccurredAt        *time.Time      `json:"occurred_at"`
	CorrelationID     *uuid.UUID      `json:"correlation_id"`
	OperatorSessionID *uuid.UUID      `json:"operator_session_id"`
	Payload           json.RawMessage `json:"payload"`
	EventType         string          `json:"event_type"`
	DedupeKey         *string         `json:"dedupe_key"`
	Reported          json.RawMessage `json:"reported"`
	Desired           json.RawMessage `json:"desired"`
}

func decodeDeviceWire(payload []byte) (deviceWire, error) {
	var w deviceWire
	if err := json.Unmarshal(payload, &w); err != nil {
		return deviceWire{}, err
	}
	return w, nil
}

func wireTelemetryFields(mid uuid.UUID, w deviceWire) TelemetryIngest {
	return TelemetryIngest{
		MachineID:         mid,
		SchemaVersion:     w.SchemaVersion,
		EventID:           strings.TrimSpace(w.EventID),
		BootID:            w.BootID,
		SeqNo:             w.SeqNo,
		OccurredAt:        w.OccurredAt,
		CorrelationID:     w.CorrelationID,
		OperatorSessionID: w.OperatorSessionID,
		DedupeKey:         w.DedupeKey,
	}
}

func assertWireMachine(mid uuid.UUID, w deviceWire) error {
	if w.MachineID == nil {
		return nil
	}
	if *w.MachineID != mid {
		return fmt.Errorf("mqtt: machine_id mismatch (topic %s vs body %s)", mid, w.MachineID)
	}
	return nil
}

func innerPayloadFromWire(w deviceWire, fallback json.RawMessage) []byte {
	if len(w.Payload) > 0 && string(w.Payload) != "null" {
		return []byte(w.Payload)
	}
	if len(fallback) > 0 {
		return []byte(fallback)
	}
	return []byte("{}")
}

func extractReportedJSON(w deviceWire) ([]byte, error) {
	if len(w.Reported) > 0 && string(w.Reported) != "null" {
		var probe map[string]any
		if err := json.Unmarshal(w.Reported, &probe); err != nil || probe == nil {
			return nil, errors.New("mqtt: shadow.reported must be a JSON object")
		}
		return w.Reported, nil
	}
	if len(w.Payload) > 0 {
		var nested struct {
			Reported map[string]any `json:"reported"`
		}
		if err := json.Unmarshal(w.Payload, &nested); err == nil && nested.Reported != nil {
			b, err := json.Marshal(nested.Reported)
			if err != nil {
				return nil, err
			}
			return b, nil
		}
		var asMap map[string]any
		if err := json.Unmarshal(w.Payload, &asMap); err == nil && asMap != nil {
			return w.Payload, nil
		}
	}
	return nil, errors.New("mqtt: shadow.reported object is required")
}

func extractDesiredJSON(w deviceWire) ([]byte, error) {
	if len(w.Desired) > 0 && string(w.Desired) != "null" {
		var probe map[string]any
		if err := json.Unmarshal(w.Desired, &probe); err != nil || probe == nil {
			return nil, errors.New("mqtt: shadow.desired must be a JSON object")
		}
		return w.Desired, nil
	}
	if len(w.Payload) > 0 {
		var nested struct {
			Desired map[string]any `json:"desired"`
		}
		if err := json.Unmarshal(w.Payload, &nested); err == nil && nested.Desired != nil {
			b, err := json.Marshal(nested.Desired)
			if err != nil {
				return nil, err
			}
			return b, nil
		}
		var asMap map[string]any
		if err := json.Unmarshal(w.Payload, &asMap); err == nil && asMap != nil {
			return w.Payload, nil
		}
	}
	return nil, errors.New("mqtt: shadow.desired object is required")
}

func normalizeReceiptStatus(raw string) (string, bool) {
	st := strings.TrimSpace(strings.ToLower(raw))
	if st == "" {
		return "", false
	}
	if mapped, ok := receiptStatusAliases[st]; ok {
		st = mapped
	}
	if _, ok := allowedReceiptStatuses[st]; !ok {
		return "", false
	}
	return st, true
}

// NormalizeReceiptStatus maps wire receipt/ack status strings to canonical persistence values.
func NormalizeReceiptStatus(raw string) (string, bool) {
	return normalizeReceiptStatus(raw)
}

func eventTypeForChannel(channel string, w deviceWire) (string, error) {
	if et := strings.TrimSpace(w.EventType); et != "" {
		return et, nil
	}
	switch channel {
	case "telemetry":
		return "", errors.New("mqtt: telemetry.event_type is required when not topic-derived")
	case "presence":
		return "presence", nil
	case "state/heartbeat":
		return "state.heartbeat", nil
	case "telemetry/snapshot":
		return "telemetry.snapshot", nil
	case "telemetry/incident":
		return "telemetry.incident", nil
	case "events/vend":
		return "events.vend", nil
	case "events/cash":
		return "events.cash", nil
	case "events/inventory":
		return "events.inventory", nil
	case "events":
		return "", errors.New("mqtt: event_type is required when channel is events")
	default:
		return "", fmt.Errorf("mqtt: unsupported channel %q", channel)
	}
}

func parseLegacyDeviceTopic(prefix, topic string) (machineID uuid.UUID, channel string, err error) {
	p := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	if err := ValidateTopicPrefix(p); err != nil {
		return uuid.Nil, "", fmt.Errorf("mqtt: invalid topic prefix: %w", err)
	}
	if !strings.HasPrefix(topic, p+"/") {
		return uuid.Nil, "", fmt.Errorf("mqtt: topic %q does not match prefix %q", topic, p)
	}
	rest := topic[len(p)+1:]
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return uuid.Nil, "", errors.New("mqtt: topic too short")
	}
	machineID, err = uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("mqtt: machine id: %w", err)
	}
	ch, ok := channelFromPathParts(parts[1:])
	if !ok {
		return uuid.Nil, "", fmt.Errorf("mqtt: unknown device channel in topic %q", topic)
	}
	return machineID, ch, nil
}

func parseEnterpriseDeviceTopic(prefix, topic string) (machineID uuid.UUID, channel string, err error) {
	p := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	if err := ValidateTopicPrefix(p); err != nil {
		return uuid.Nil, "", fmt.Errorf("mqtt: invalid topic prefix: %w", err)
	}
	if !strings.HasPrefix(topic, p+"/") {
		return uuid.Nil, "", fmt.Errorf("mqtt: topic %q does not match prefix %q", topic, p)
	}
	rest := topic[len(p)+1:]
	parts := strings.Split(rest, "/")
	if len(parts) < 3 {
		return uuid.Nil, "", errors.New("mqtt: enterprise topic too short")
	}
	if parts[0] != "machines" {
		return uuid.Nil, "", fmt.Errorf("mqtt: enterprise topic missing machines segment in %q", topic)
	}
	machineID, err = uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("mqtt: machine id: %w", err)
	}
	ch, ok := channelFromPathParts(parts[2:])
	if !ok {
		return uuid.Nil, "", fmt.Errorf("mqtt: unknown enterprise device channel in topic %q", topic)
	}
	return machineID, ch, nil
}

// ParseDeviceTopicWithLayout extracts machine id and channel for legacy or enterprise layouts.
func ParseDeviceTopicWithLayout(layout TopicLayout, prefix, topic string) (machineID uuid.UUID, channel string, err error) {
	if NormalizeTopicLayout(string(layout)) == TopicLayoutEnterprise {
		return parseEnterpriseDeviceTopic(prefix, topic)
	}
	return parseLegacyDeviceTopic(prefix, topic)
}

// ParseDeviceTopic extracts machine id and channel key from a subscription topic under TopicPrefix (legacy layout).
func ParseDeviceTopic(prefix, topic string) (machineID uuid.UUID, channel string, err error) {
	return parseLegacyDeviceTopic(prefix, topic)
}

func ingestReject(hooks *IngestHooks, topic, reason string, payloadBytes int) {
	if hooks != nil && hooks.OnIngressRejected != nil {
		hooks.OnIngressRejected(topic, reason, payloadBytes)
	}
}

func channelFromPathParts(parts []string) (string, bool) {
	switch {
	case len(parts) == 1 && parts[0] == "telemetry":
		return "telemetry", true
	case len(parts) == 1 && parts[0] == "presence":
		return "presence", true
	case len(parts) == 2 && parts[0] == "state" && parts[1] == "heartbeat":
		return "state/heartbeat", true
	case len(parts) == 2 && parts[0] == "telemetry" && parts[1] == "snapshot":
		return "telemetry/snapshot", true
	case len(parts) == 2 && parts[0] == "telemetry" && parts[1] == "incident":
		return "telemetry/incident", true
	case len(parts) == 2 && parts[0] == "events" && parts[1] == "vend":
		return "events/vend", true
	case len(parts) == 2 && parts[0] == "events" && parts[1] == "cash":
		return "events/cash", true
	case len(parts) == 2 && parts[0] == "events" && parts[1] == "inventory":
		return "events/inventory", true
	case len(parts) == 1 && parts[0] == "events":
		return "events", true
	case len(parts) == 2 && parts[0] == "shadow" && parts[1] == "reported":
		return "shadow/reported", true
	case len(parts) == 2 && parts[0] == "shadow" && parts[1] == "desired":
		return "shadow/desired", true
	case len(parts) == 2 && parts[0] == "commands" && parts[1] == "receipt":
		return "commands/receipt", true
	case len(parts) == 2 && parts[0] == "commands" && parts[1] == "ack":
		return "commands/ack", true
	default:
		return "", false
	}
}

// Dispatch parses the topic and JSON body and invokes the appropriate ingest hook.
// lim may be nil to skip optional ingress bounds (backwards compatible for tests).
// hooks may be nil; when set, OnIngressRejected receives early rejections before DeviceIngest.
func Dispatch(ctx context.Context, layout TopicLayout, prefix, topic string, payload []byte, ing DeviceIngest, lim *TelemetryIngressLimits, hooks *IngestHooks) error {
	if ing == nil {
		return errors.New("mqtt: nil DeviceIngest")
	}
	mid, channel, err := ParseDeviceTopicWithLayout(layout, prefix, topic)
	if err != nil {
		mqttInvalidTopicsTotal.WithLabelValues("parse").Inc()
		ingestReject(hooks, topic, "topic_parse", len(payload))
		return err
	}
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	if lim != nil && lim.MaxPayloadBytes > 0 && len(payload) > lim.MaxPayloadBytes {
		ingestReject(hooks, topic, "payload_too_large", len(payload))
		return fmt.Errorf("mqtt: payload exceeds max bytes (%d > %d)", len(payload), lim.MaxPayloadBytes)
	}
	if !json.Valid(payload) {
		ingestReject(hooks, topic, "invalid_json", len(payload))
		return errors.New("mqtt: payload is not valid JSON")
	}

	w, err := decodeDeviceWire(payload)
	if err != nil {
		ingestReject(hooks, topic, "json_decode", len(payload))
		return fmt.Errorf("mqtt: json: %w", err)
	}
	if err := assertWireMachine(mid, w); err != nil {
		mqttInvalidTopicsTotal.WithLabelValues("machine_id_mismatch").Inc()
		ingestReject(hooks, topic, "machine_id_mismatch", len(payload))
		return err
	}

	switch channel {
	case "telemetry", "presence", "state/heartbeat", "telemetry/snapshot", "telemetry/incident", "events/vend", "events/cash", "events/inventory":
		et, err := eventTypeForChannel(channel, w)
		if err != nil {
			ingestReject(hooks, topic, "telemetry_missing_event_type", len(payload))
			return err
		}
		raw := innerPayloadFromWire(w, nil)
		if !json.Valid(raw) {
			ingestReject(hooks, topic, "telemetry_invalid_inner_json", len(payload))
			return errors.New("mqtt: telemetry.payload must be valid JSON")
		}
		if lim != nil && lim.MaxPoints > 0 && lim.MaxTags > 0 {
			if err := ValidateTelemetryPayloadComplexity(raw, lim.MaxPoints, lim.MaxTags); err != nil {
				ingestReject(hooks, topic, "telemetry_complexity", len(payload))
				return fmt.Errorf("mqtt: %w", err)
			}
		}
		ti := wireTelemetryFields(mid, w)
		ti.EventType = et
		ti.Payload = raw
		return ing.IngestTelemetry(ctx, ti)

	case "shadow/reported":
		rep, err := extractReportedJSON(w)
		if err != nil {
			ingestReject(hooks, topic, "shadow_missing_reported", len(payload))
			return err
		}
		if !json.Valid(rep) {
			ingestReject(hooks, topic, "shadow_invalid_inner_json", len(payload))
			return errors.New("mqtt: shadow.reported must be valid JSON")
		}
		return ing.IngestShadowReported(ctx, ShadowReportedIngest{
			MachineID:    mid,
			ReportedJSON: rep,
		})

	case "shadow/desired":
		des, err := extractDesiredJSON(w)
		if err != nil {
			ingestReject(hooks, topic, "shadow_missing_desired", len(payload))
			return err
		}
		if !json.Valid(des) {
			ingestReject(hooks, topic, "shadow_invalid_desired_json", len(payload))
			return errors.New("mqtt: shadow.desired must be valid JSON")
		}
		return ing.IngestShadowDesired(ctx, ShadowDesiredIngest{
			MachineID:   mid,
			DesiredJSON: des,
		})

	case "commands/receipt", "commands/ack":
		var body struct {
			Sequence      int64           `json:"sequence"`
			Status        string          `json:"status"`
			CorrelationID *uuid.UUID      `json:"correlation_id"`
			Data          json.RawMessage `json:"payload"`
			DedupeKey     string          `json:"dedupe_key"`
			CommandID     uuid.UUID       `json:"command_id"`
			MachineID     uuid.UUID       `json:"machine_id"`
			OccurredAt    time.Time       `json:"occurred_at"`
		}
		if err := json.Unmarshal(payload, &body); err != nil {
			ingestReject(hooks, topic, "receipt_json", len(payload))
			return fmt.Errorf("mqtt: command receipt json: %w", err)
		}
		if body.Sequence < 0 {
			ingestReject(hooks, topic, "receipt_bad_sequence", len(payload))
			return errors.New("mqtt: commands.receipt.sequence must be >= 0")
		}
		st, ok := normalizeReceiptStatus(body.Status)
		if !ok {
			ingestReject(hooks, topic, "receipt_bad_status", len(payload))
			return fmt.Errorf("mqtt: invalid receipt status %q", body.Status)
		}
		if strings.TrimSpace(body.DedupeKey) == "" {
			ingestReject(hooks, topic, "receipt_missing_dedupe", len(payload))
			return errors.New("mqtt: commands.receipt.dedupe_key is required")
		}
		if err := ValidateEdgeCommandReceipt(mid, body.CommandID, body.MachineID, body.OccurredAt); err != nil {
			mqttprom.RecordCommandAckRejected("invalid_edge_command_receipt")
			ingestReject(hooks, topic, "receipt_identity_fields", len(payload))
			return err
		}
		raw := []byte(body.Data)
		if len(raw) == 0 {
			raw = []byte("{}")
		}
		if !json.Valid(raw) {
			ingestReject(hooks, topic, "receipt_invalid_inner_json", len(payload))
			return errors.New("mqtt: commands.receipt.payload must be valid JSON")
		}
		return ing.IngestCommandReceipt(ctx, CommandReceiptIngest{
			MachineID:     mid,
			Sequence:      body.Sequence,
			Status:        st,
			CorrelationID: body.CorrelationID,
			Payload:       raw,
			DedupeKey:     strings.TrimSpace(body.DedupeKey),
			CommandID:     body.CommandID,
			OccurredAt:    body.OccurredAt,
		})

	default:
		ingestReject(hooks, topic, "unsupported_channel", len(payload))
		return fmt.Errorf("mqtt: unsupported channel %q", channel)
	}
}
