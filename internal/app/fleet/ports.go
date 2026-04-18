package fleet

import (
	"context"

	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/google/uuid"
)

// FleetRepository is the persistence port for fleet workflows. Postgres adapters implement this
// contract using the transactional system of record.
type FleetRepository interface {
	GetMachine(ctx context.Context, machineID uuid.UUID) (domainfleet.Machine, error)
	GetTechnician(ctx context.Context, technicianID uuid.UUID) (domainfleet.Technician, error)

	AssertSiteInOrganization(ctx context.Context, organizationID, siteID uuid.UUID) error

	InsertMachine(ctx context.Context, p InsertMachineParams) (domainfleet.Machine, error)
	UpdateMachineMetadata(ctx context.Context, p UpdateMachineMetadataParams) (domainfleet.Machine, error)

	ListMachinesInScope(ctx context.Context, filter ListMachinesScope) ([]domainfleet.Machine, error)

	InsertTechnicianMachineAssignment(ctx context.Context, p InsertAssignmentParams) (domainfleet.TechnicianMachineAssignment, error)
}

// InsertMachineParams captures fields required to provision a machine row.
type InsertMachineParams struct {
	OrganizationID    uuid.UUID
	SiteID            uuid.UUID
	HardwareProfileID *uuid.UUID
	SerialNumber      string
	Name              string
	Status            string
}

// UpdateMachineMetadataParams applies a partial metadata update within an organization scope.
// Nil pointer fields mean "leave unchanged".
type UpdateMachineMetadataParams struct {
	OrganizationID    uuid.UUID
	MachineID         uuid.UUID
	Name              *string
	Status            *string
	HardwareProfileID *uuid.UUID
}

// InsertAssignmentParams creates a technician–machine assignment row.
type InsertAssignmentParams struct {
	TechnicianID uuid.UUID
	MachineID    uuid.UUID
	Role         string
}

// ListMachinesScope selects machines visible within an organization, optionally narrowed to a site.
type ListMachinesScope struct {
	OrganizationID uuid.UUID
	SiteID         *uuid.UUID
}

// FleetWorkflows is the application service surface intended for wiring from HTTP or other transports.
type FleetWorkflows interface {
	CreateMachine(ctx context.Context, in CreateMachineInput) (domainfleet.Machine, error)
	UpdateMachineMetadata(ctx context.Context, in UpdateMachineMetadataInput) (domainfleet.Machine, error)
	AssignTechnicianToMachine(ctx context.Context, in AssignTechnicianInput) (domainfleet.TechnicianMachineAssignment, error)
	ListMachinesInScope(ctx context.Context, scope ListMachinesScope) ([]domainfleet.Machine, error)
}
