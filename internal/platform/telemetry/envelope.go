package telemetry

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const DefaultSchemaVersion = 1

// Envelope is the canonical wire shape for telemetry republished on NATS (and for worker consumers).
type Envelope struct {
	SchemaVersion int             `json:"schema_version"`
	Class         Class           `json:"class"`
	MachineID     uuid.UUID       `json:"machine_id"`
	TenantID      *uuid.UUID      `json:"tenant_id,omitempty"`
	SiteID        *uuid.UUID      `json:"site_id,omitempty"`
	AppVersion    string          `json:"app_version,omitempty"`
	FirmwareVer   string          `json:"firmware_version,omitempty"`
	EventID       string          `json:"event_id,omitempty"`
	Idempotency   string          `json:"idempotency_key,omitempty"`
	EmittedAt     *time.Time      `json:"emitted_at,omitempty"`
	ReceivedAt    time.Time       `json:"received_at"`
	Severity      string          `json:"severity,omitempty"`
	SourceEvent   string          `json:"source_event_type,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

// Marshal serializes the envelope to JSON bytes.
func (e Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
