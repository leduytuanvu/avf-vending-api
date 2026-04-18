package fleet

import "errors"

var (
	// ErrInvalidArgument indicates caller input failed validation before persistence.
	ErrInvalidArgument = errors.New("fleet: invalid argument")
	// ErrNotFound indicates the referenced aggregate does not exist in the system of record.
	ErrNotFound = errors.New("fleet: not found")
	// ErrOrgMismatch indicates the resource is outside the caller's organization scope.
	ErrOrgMismatch = errors.New("fleet: organization scope mismatch")
)
