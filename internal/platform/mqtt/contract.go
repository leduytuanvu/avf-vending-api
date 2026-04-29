package mqtt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DeviceIngest is implemented by the Postgres store to persist telemetry, shadow, receipts, and connectivity.
type DeviceIngest interface {
	IngestTelemetry(ctx context.Context, in TelemetryIngest) error
	IngestShadowReported(ctx context.Context, in ShadowReportedIngest) error
	IngestShadowDesired(ctx context.Context, in ShadowDesiredIngest) error
	IngestCommandReceipt(ctx context.Context, in CommandReceiptIngest) error
}

// TelemetryIngest is validated JSON routed as operational telemetry (JetStream metrics/heartbeat/incident buckets).
type TelemetryIngest struct {
	MachineID         uuid.UUID
	EventType         string
	Payload           []byte
	DedupeKey         *string
	SchemaVersion     int
	EventID           string
	BootID            *uuid.UUID
	SeqNo             *int64
	OccurredAt        *time.Time
	CorrelationID     *uuid.UUID
	OperatorSessionID *uuid.UUID
}

// TelemetryIdempotencyKey prefers explicit dedupe_key, else machine+boot+seq, else stable event_id.
func TelemetryIdempotencyKey(machineID uuid.UUID, in TelemetryIngest) string {
	if in.DedupeKey != nil {
		if s := strings.TrimSpace(*in.DedupeKey); s != "" {
			return s
		}
	}
	if in.BootID != nil && in.SeqNo != nil {
		return fmt.Sprintf("%s:%s:%d", machineID.String(), in.BootID.String(), *in.SeqNo)
	}
	return strings.TrimSpace(in.EventID)
}

// ShadowReportedIngest is validated JSON from {prefix}/{machineId}/shadow/reported.
type ShadowReportedIngest struct {
	MachineID    uuid.UUID
	ReportedJSON []byte
}

// ShadowDesiredIngest is validated JSON from {prefix}/{machineId}/shadow/desired.
type ShadowDesiredIngest struct {
	MachineID   uuid.UUID
	DesiredJSON []byte
}

// CommandReceiptIngest is validated JSON from {prefix}/{machineId}/commands/receipt or commands/ack.
type CommandReceiptIngest struct {
	MachineID     uuid.UUID
	Sequence      int64
	Status        string
	CorrelationID *uuid.UUID
	Payload       []byte
	DedupeKey     string
	CommandID     uuid.UUID
	OccurredAt    time.Time
}

// ValidateEdgeCommandReceipt enforces topic/body identity fields required for MQTT command ACK safety.
func ValidateEdgeCommandReceipt(topicMachineID, commandID, payloadMachineID uuid.UUID, occurredAt time.Time) error {
	if commandID == uuid.Nil {
		return fmt.Errorf("mqtt: command_id is required")
	}
	if payloadMachineID == uuid.Nil || payloadMachineID != topicMachineID {
		return fmt.Errorf("mqtt: machine_id must match topic machine")
	}
	if occurredAt.IsZero() {
		return fmt.Errorf("mqtt: occurred_at is required")
	}
	return nil
}
