package fleet

import "errors"

var (
	// ErrInvalidArgument indicates caller input failed validation before persistence.
	ErrInvalidArgument = errors.New("fleet: invalid argument")
	// ErrNotFound indicates the referenced aggregate does not exist in the system of record.
	ErrNotFound = errors.New("fleet: not found")
	// ErrOrgMismatch indicates the resource is outside the caller's organization scope.
	ErrOrgMismatch = errors.New("fleet: organization scope mismatch")
	// ErrConflict indicates a uniqueness or concurrency conflict.
	ErrConflict = errors.New("fleet: conflict")
	// ErrSiteHasMachines blocks deactivating a site that still has non-retired machines.
	ErrSiteHasMachines = errors.New("fleet: site still has machines")
	// ErrForbiddenTechnicianSelfAssignment blocks assigning your own technician identity to a machine.
	ErrForbiddenTechnicianSelfAssignment = errors.New("fleet: technician cannot self-assign")
)
