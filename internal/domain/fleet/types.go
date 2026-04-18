package fleet

import (
	"time"

	"github.com/google/uuid"
)

// MachineHardwareProfile describes a class of vending hardware.
type MachineHardwareProfile struct {
	ID             uuid.UUID
	OrganizationID *uuid.UUID
	Name           string
	CreatedAt      time.Time
}

// Machine is a deployed vending asset.
type Machine struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	SiteID            uuid.UUID
	HardwareProfileID *uuid.UUID
	SerialNumber      string
	Name              string
	Status            string
	CommandSequence   int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Technician is a field operator identity.
type Technician struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	DisplayName    string
	CreatedAt      time.Time
}

// TechnicianMachineAssignment links technicians to machines for a time window.
type TechnicianMachineAssignment struct {
	ID           uuid.UUID
	TechnicianID uuid.UUID
	MachineID    uuid.UUID
	Role         string
	ValidFrom    time.Time
	ValidTo      *time.Time
	CreatedAt    time.Time
}
