package device

import (
	"time"

	"github.com/google/uuid"
)

// ShadowDocument is a portable view of the machine_shadow row (JSON payloads as raw bytes).
type ShadowDocument struct {
	MachineID     uuid.UUID
	DesiredState  []byte
	ReportedState []byte
	Version       int64
	UpdatedAt     time.Time
}

// CommandLedgerView is the latest known command row for dispatch and timeout evaluation.
// There is no persisted dispatch state in the current schema; assessments are advisory only.
type CommandLedgerView struct {
	ID          uuid.UUID
	MachineID   uuid.UUID
	Sequence    int64
	CommandType string
	Payload     []byte
	CreatedAt   time.Time
}

// MachinePresence captures operator-facing machine status and last row mutation time.
// TODO: Replace UpdatedAt with a dedicated last_seen_at once ingest writes it.
type MachinePresence struct {
	MachineID uuid.UUID
	Status    string
	UpdatedAt time.Time
}

// CommandDispatchAssessment summarizes whether the newest command appears stuck awaiting edge pickup.
type CommandDispatchAssessment struct {
	HasCommand     bool
	Sequence       int64
	CreatedAt      time.Time
	PendingTooLong bool
	OverTimeoutBy  time.Duration
}

// MachineReachabilityAssessment combines row status with recency for connectivity heuristics.
type MachineReachabilityAssessment struct {
	MachineID            uuid.UUID
	Status               string
	LastRecordActivityAt time.Time
	Stale                bool
	Reason               string
}

// RecordReportedInput applies device-reported shadow JSON with optional optimistic locking.
type RecordReportedInput struct {
	MachineID       uuid.UUID
	Reported        []byte
	ExpectedVersion *int64
}
