package mediaadmin

import "errors"

var (
	// ErrNotConfigured is returned when object storage is not wired for this process.
	ErrNotConfigured   = errors.New("mediaadmin: object storage not configured")
	ErrInvalidArgument = errors.New("mediaadmin: invalid argument")
	ErrNotFound        = errors.New("mediaadmin: not found")
	ErrConflict        = errors.New("mediaadmin: conflict")
)
