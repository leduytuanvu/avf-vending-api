package fleet

import (
	"context"

	"github.com/google/uuid"
)

// MachineRepository reads machine rows from the system of record.
type MachineRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Machine, error)
}

// TechnicianRepository reads technician identities for validation and display.
type TechnicianRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Technician, error)
}

// TechnicianMachineAssignmentChecker verifies active technician–machine assignments.
type TechnicianMachineAssignmentChecker interface {
	HasActiveAssignment(ctx context.Context, technicianID, machineID uuid.UUID) (bool, error)
}
