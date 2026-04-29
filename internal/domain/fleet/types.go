package fleet

import (
	"time"

	"github.com/google/uuid"
)

// Site is a physical location within an organization (warehouse, store, depot).
type Site struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	RegionID       *uuid.UUID
	Name           string
	Address        []byte
	Timezone       string
	Code           string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

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
	Code              string
	Model             *string
	CabinetType       string
	Timezone          *string
	Name              string
	Status            string
	CredentialVersion int64
	LastSeenAt        *time.Time
	ActivatedAt       *time.Time
	RevokedAt         *time.Time
	RotatedAt         *time.Time
	CommandSequence   int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Technician is a field operator identity.
type Technician struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	DisplayName     string
	Email           *string
	Phone           *string
	ExternalSubject *string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TechnicianMachineAssignment links technicians to machines for a time window.
type TechnicianMachineAssignment struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	TechnicianID   uuid.UUID
	MachineID      uuid.UUID
	Role           string
	Scope          string
	Status         string
	ValidFrom      time.Time
	ValidTo        *time.Time
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
