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
)
