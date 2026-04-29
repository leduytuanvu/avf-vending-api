package device

import "errors"

var (
	// ErrInvalidArgument indicates caller input failed validation before side effects.
	ErrInvalidArgument = errors.New("device: invalid argument")
	// ErrNotFound indicates no row exists for the requested aggregate (e.g. no shadow row yet).
	ErrNotFound = errors.New("device: not found")
	// ErrNotConfigured indicates an optional dependency was not provided to the service.
	ErrNotConfigured = errors.New("device: dependency not configured")
	// ErrVersionMismatch indicates an optimistic concurrency check on shadow.version failed.
	ErrVersionMismatch = errors.New("device: shadow version mismatch")
	// ErrMachineNotCommandable indicates the machine is not active enough to receive runtime MQTT commands.
	ErrMachineNotCommandable = errors.New("device: machine is not commandable")
	// ErrCommandNotRetryable indicates the command has reached a terminal transport/business outcome that must not be redispatched.
	ErrCommandNotRetryable = errors.New("device: command is not retryable")
	// ErrCommandRetryRequiresIdempotency indicates command_ledger.idempotency_key is empty so AppendCommand replay routing cannot run.
	ErrCommandRetryRequiresIdempotency = errors.New("device: command retry requires idempotency_key")
)
