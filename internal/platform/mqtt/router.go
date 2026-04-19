package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var allowedReceiptStatuses = map[string]struct{}{
	"acked": {}, "nacked": {}, "failed": {}, "timeout": {},
}

// ParseDeviceTopic extracts machine id and channel key from a subscription topic under TopicPrefix.
// Supported channels: "telemetry", "shadow/reported", "commands/receipt".
func ParseDeviceTopic(prefix, topic string) (machineID uuid.UUID, channel string, err error) {
	p := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	if p == "" {
		return uuid.Nil, "", errors.New("mqtt: empty topic prefix")
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

func ingestReject(hooks *IngestHooks, topic, reason string, payloadBytes int) {
	if hooks != nil && hooks.OnIngressRejected != nil {
		hooks.OnIngressRejected(topic, reason, payloadBytes)
	}
}

func channelFromPathParts(parts []string) (string, bool) {
	switch {
	case len(parts) == 1 && parts[0] == "telemetry":
		return "telemetry", true
	case len(parts) == 2 && parts[0] == "shadow" && parts[1] == "reported":
		return "shadow/reported", true
	case len(parts) == 2 && parts[0] == "commands" && parts[1] == "receipt":
		return "commands/receipt", true
	default:
		return "", false
	}
}

// Dispatch parses the topic and JSON body and invokes the appropriate ingest hook.
// lim may be nil to skip optional ingress bounds (backwards compatible for tests).
// hooks may be nil; when set, OnIngressRejected receives early rejections before DeviceIngest.
func Dispatch(ctx context.Context, prefix, topic string, payload []byte, ing DeviceIngest, lim *TelemetryIngressLimits, hooks *IngestHooks) error {
	if ing == nil {
		return errors.New("mqtt: nil DeviceIngest")
	}
	mid, channel, err := ParseDeviceTopic(prefix, topic)
	if err != nil {
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
	switch channel {
	case "telemetry":
		var body struct {
			EventType string          `json:"event_type"`
			Data      json.RawMessage `json:"payload"`
			DedupeKey *string         `json:"dedupe_key"`
		}
		if err := json.Unmarshal(payload, &body); err != nil {
			ingestReject(hooks, topic, "telemetry_json", len(payload))
			return fmt.Errorf("mqtt: telemetry json: %w", err)
		}
		if strings.TrimSpace(body.EventType) == "" {
			ingestReject(hooks, topic, "telemetry_missing_event_type", len(payload))
			return errors.New("mqtt: telemetry.event_type is required")
		}
		raw := []byte(body.Data)
		if len(raw) == 0 {
			raw = []byte("{}")
		}
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
		return ing.IngestTelemetry(ctx, TelemetryIngest{
			MachineID: mid,
			EventType: strings.TrimSpace(body.EventType),
			Payload:   raw,
			DedupeKey: body.DedupeKey,
		})
	case "shadow/reported":
		var body struct {
			Reported map[string]any `json:"reported"`
		}
		if err := json.Unmarshal(payload, &body); err != nil {
			ingestReject(hooks, topic, "shadow_json", len(payload))
			return fmt.Errorf("mqtt: shadow json: %w", err)
		}
		if body.Reported == nil {
			ingestReject(hooks, topic, "shadow_missing_reported", len(payload))
			return errors.New("mqtt: shadow.reported object is required")
		}
		b, err := json.Marshal(body.Reported)
		if err != nil {
			ingestReject(hooks, topic, "shadow_marshal", len(payload))
			return fmt.Errorf("mqtt: shadow marshal: %w", err)
		}
		return ing.IngestShadowReported(ctx, ShadowReportedIngest{
			MachineID:    mid,
			ReportedJSON: b,
		})
	case "commands/receipt":
		var body struct {
			Sequence      int64           `json:"sequence"`
			Status        string          `json:"status"`
			CorrelationID *uuid.UUID      `json:"correlation_id"`
			Data          json.RawMessage `json:"payload"`
			DedupeKey     string          `json:"dedupe_key"`
		}
		if err := json.Unmarshal(payload, &body); err != nil {
			ingestReject(hooks, topic, "receipt_json", len(payload))
			return fmt.Errorf("mqtt: command receipt json: %w", err)
		}
		if body.Sequence < 0 {
			ingestReject(hooks, topic, "receipt_bad_sequence", len(payload))
			return errors.New("mqtt: commands.receipt.sequence must be >= 0")
		}
		st := strings.TrimSpace(strings.ToLower(body.Status))
		if _, ok := allowedReceiptStatuses[st]; !ok {
			ingestReject(hooks, topic, "receipt_bad_status", len(payload))
			return fmt.Errorf("mqtt: invalid receipt status %q", body.Status)
		}
		if strings.TrimSpace(body.DedupeKey) == "" {
			ingestReject(hooks, topic, "receipt_missing_dedupe", len(payload))
			return errors.New("mqtt: commands.receipt.dedupe_key is required")
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
		})
	default:
		ingestReject(hooks, topic, "unsupported_channel", len(payload))
		return fmt.Errorf("mqtt: unsupported channel %q", channel)
	}
}
