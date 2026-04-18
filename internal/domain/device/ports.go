package device

import (
	"context"

	"github.com/google/uuid"
)

// AppendCommandInput appends a command ledger row and updates machine shadow desired state atomically.
type AppendCommandInput struct {
	MachineID         uuid.UUID
	CommandType       string
	Payload           []byte
	CorrelationID     *uuid.UUID
	IdempotencyKey    string
	DesiredState      []byte
	OperatorSessionID *uuid.UUID
}

// AppendCommandResult contains the persisted ledger entry and shadow snapshot.
type AppendCommandResult struct {
	CommandID uuid.UUID
	Sequence  int64
	Replay    bool
}

// CommandShadowWorkflow coordinates command_ledger + machine_shadow updates.
type CommandShadowWorkflow interface {
	AppendCommandUpdateShadow(ctx context.Context, in AppendCommandInput) (AppendCommandResult, error)
}
