package mqtt

import (
	"context"

	"github.com/google/uuid"
)

// DeviceIngest is implemented by the Postgres store to persist telemetry, shadow, receipts, and connectivity.
type DeviceIngest interface {
	IngestTelemetry(ctx context.Context, in TelemetryIngest) error
	IngestShadowReported(ctx context.Context, in ShadowReportedIngest) error
	IngestCommandReceipt(ctx context.Context, in CommandReceiptIngest) error
}

// TelemetryIngest is validated JSON from {prefix}/{machineId}/telemetry.
type TelemetryIngest struct {
	MachineID uuid.UUID
	EventType string
	Payload   []byte
	DedupeKey *string
}

// ShadowReportedIngest is validated JSON from {prefix}/{machineId}/shadow/reported.
type ShadowReportedIngest struct {
	MachineID    uuid.UUID
	ReportedJSON []byte
}

// CommandReceiptIngest is validated JSON from {prefix}/{machineId}/commands/receipt.
type CommandReceiptIngest struct {
	MachineID     uuid.UUID
	Sequence      int64
	Status        string
	CorrelationID *uuid.UUID
	Payload       []byte
	DedupeKey     string
}
