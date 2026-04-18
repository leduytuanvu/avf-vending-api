package device

import (
	"context"
	"time"

	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/google/uuid"
)

// ShadowReader loads the current shadow document for merge and parity decisions.
type ShadowReader interface {
	GetShadow(ctx context.Context, machineID uuid.UUID) (ShadowDocument, error)
}

// ReportedShadowWriter persists reported_state only (desired must remain under command workflow).
type ReportedShadowWriter interface {
	ReplaceReportedState(ctx context.Context, machineID uuid.UUID, reported []byte) (ShadowDocument, error)
}

// CommandLedgerReader returns the highest-sequence command for a machine, if any.
type CommandLedgerReader interface {
	GetLatestCommand(ctx context.Context, machineID uuid.UUID) (CommandLedgerView, error)
}

// MachinePresenceReader supplies machine status and updated_at for last-seen style checks.
type MachinePresenceReader interface {
	GetMachinePresence(ctx context.Context, machineID uuid.UUID) (MachinePresence, error)
}

// Clock isolates time for timeout and staleness tests.
type Clock interface {
	Now() time.Time
}

// Deps wires orchestration dependencies. Workflow is required; others may be nil until adapters exist.
type Deps struct {
	Workflow domaindevice.CommandShadowWorkflow
	Shadow   ShadowReader
	Reported ReportedShadowWriter
	Commands CommandLedgerReader
	Machines MachinePresenceReader
	Clock    Clock
}

// Orchestrator is the application surface for HTTP/MQTT workers to delegate to.
type Orchestrator interface {
	EnqueueCommand(ctx context.Context, in domaindevice.AppendCommandInput) (domaindevice.AppendCommandResult, error)
	EnqueueCommandMergeDesired(ctx context.Context, base domaindevice.AppendCommandInput, desiredPatchJSON []byte) (domaindevice.AppendCommandResult, error)
	GetShadow(ctx context.Context, machineID uuid.UUID) (ShadowDocument, error)
	RecordReportedState(ctx context.Context, in RecordReportedInput) (ShadowDocument, error)
	AssessCommandDispatch(ctx context.Context, machineID uuid.UUID, dispatchWaitTimeout time.Duration) (CommandDispatchAssessment, error)
	AssessMachineReachability(ctx context.Context, machineID uuid.UUID, staleAfter time.Duration) (MachineReachabilityAssessment, error)
}
