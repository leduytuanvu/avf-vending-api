package fleet

import (
	"context"
	"time"

	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/google/uuid"
)

// FleetRepository is the persistence port for fleet workflows. Postgres adapters implement this
// contract using the transactional system of record.
type FleetRepository interface {
	GetMachine(ctx context.Context, machineID uuid.UUID) (domainfleet.Machine, error)
	GetTechnician(ctx context.Context, technicianID uuid.UUID) (domainfleet.Technician, error)

	AssertSiteInCompany(ctx context.Context, companyID, siteID uuid.UUID) error

	InsertMachine(ctx context.Context, p InsertMachineParams) (domainfleet.Machine, error)
	UpdateMachineMetadata(ctx context.Context, p UpdateMachineMetadataParams) (domainfleet.Machine, error)

	ListMachinesInScope(ctx context.Context, filter ListMachinesScope) ([]domainfleet.Machine, error)

	InsertTechnicianMachineAssignment(ctx context.Context, p InsertAssignmentParams) (domainfleet.TechnicianMachineAssignment, error)

	InsertSite(ctx context.Context, p InsertSiteParams) (domainfleet.Site, error)
	GetSiteForOrg(ctx context.Context, companyID, siteID uuid.UUID) (domainfleet.Site, error)
	ListSitesForOrg(ctx context.Context, p ListSitesParams) ([]domainfleet.Site, error)
	CountSitesForOrg(ctx context.Context, p ListSitesParams) (int64, error)
	UpdateSite(ctx context.Context, p UpdateSiteParams) (domainfleet.Site, error)
	CountNonRetiredMachinesForSite(ctx context.Context, companyID, siteID uuid.UUID) (int64, error)

	InsertTechnicianRow(ctx context.Context, p InsertTechnicianParams) (domainfleet.Technician, error)
	GetTechnicianForOrg(ctx context.Context, companyID, technicianID uuid.UUID) (domainfleet.Technician, error)
	ListTechniciansForOrg(ctx context.Context, p ListTechniciansParams) ([]domainfleet.Technician, error)
	CountTechniciansForOrg(ctx context.Context, p ListTechniciansParams) (int64, error)
	UpdateTechnicianRow(ctx context.Context, p UpdateTechnicianRowParams) (domainfleet.Technician, error)
	SetTechnicianStatus(ctx context.Context, companyID, technicianID uuid.UUID, status string) (domainfleet.Technician, error)

	GetTechnicianAssignmentForOrg(ctx context.Context, companyID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error)
	UpdateTechnicianAssignment(ctx context.Context, p UpdateAssignmentParams) (domainfleet.TechnicianMachineAssignment, error)
	ReleaseTechnicianAssignment(ctx context.Context, companyID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error)
	ReleaseTechnicianAssignmentForMachineUser(ctx context.Context, companyID, machineID, technicianID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error)

	BumpMachineCredentialVersion(ctx context.Context, companyID, machineID uuid.UUID) (int64, error)
	RevokeMachineCredentials(ctx context.Context, companyID, machineID uuid.UUID) (int64, error)
	RevokeActiveMachineActivationCodes(ctx context.Context, companyID, machineID uuid.UUID) error

	RotateMachineCredentialLifecycle(ctx context.Context, companyID, machineID uuid.UUID) (domainfleet.Machine, error)
	RevokeMachineCredentialLifecycle(ctx context.Context, companyID, machineID uuid.UUID, compromiseMachineCredentials bool) (domainfleet.Machine, error)
	RevokeAllMachineSessionsOnly(ctx context.Context, companyID, machineID uuid.UUID) error
}

// InsertMachineParams captures fields required to provision a machine row.
type InsertMachineParams struct {
	SiteID            uuid.UUID
	HardwareProfileID *uuid.UUID
	SerialNumber      string
	Code              string
	Model             string
	CabinetType       string
	Timezone          string
	Name              string
	Status            string
}

// UpdateMachineMetadataParams applies a partial metadata update within an company scope.
// Nil pointer fields mean "leave unchanged".
type UpdateMachineMetadataParams struct {
	MachineID         uuid.UUID
	Name              *string
	Status            *string
	HardwareProfileID *uuid.UUID
	SiteID            *uuid.UUID
	SerialNumber      *string
	Code              *string
	Model             *string
	CabinetType       *string
	Timezone          *string
}

// InsertAssignmentParams creates a technician–machine assignment row.
type InsertAssignmentParams struct {
	TechnicianID     uuid.UUID
	MachineID        uuid.UUID
	Role             string
	AssignmentDomain string
	CreatedBy        *uuid.UUID
}

// ListMachinesScope selects machines visible within an company, optionally narrowed to a site.
type ListMachinesScope struct {
	SiteID *uuid.UUID
}

// FleetWorkflows is the application service surface intended for wiring from HTTP or other transports.
type FleetWorkflows interface {
	CreateMachine(ctx context.Context, in CreateMachineInput) (domainfleet.Machine, error)
	UpdateMachineMetadata(ctx context.Context, in UpdateMachineMetadataInput) (domainfleet.Machine, error)
	AssignTechnicianToMachine(ctx context.Context, in AssignTechnicianInput) (domainfleet.TechnicianMachineAssignment, error)
	ListMachinesInScope(ctx context.Context, scope ListMachinesScope) ([]domainfleet.Machine, error)

	CreateSite(ctx context.Context, in CreateSiteInput) (domainfleet.Site, error)
	UpdateSite(ctx context.Context, in UpdateSiteInput) (domainfleet.Site, error)
	GetSite(ctx context.Context, companyID, siteID uuid.UUID) (domainfleet.Site, error)
	ListSites(ctx context.Context, in ListSitesInput) ([]domainfleet.Site, int64, error)
	DeactivateSite(ctx context.Context, companyID, siteID uuid.UUID) (domainfleet.Site, error)

	CreateTechnician(ctx context.Context, in CreateTechnicianInput) (domainfleet.Technician, error)
	UpdateTechnician(ctx context.Context, in UpdateTechnicianInput) (domainfleet.Technician, error)
	GetTechnician(ctx context.Context, companyID, technicianID uuid.UUID) (domainfleet.Technician, error)
	ListTechnicians(ctx context.Context, in ListTechniciansInput) ([]domainfleet.Technician, int64, error)
	DisableTechnician(ctx context.Context, companyID, technicianID uuid.UUID) (domainfleet.Technician, error)
	EnableTechnician(ctx context.Context, companyID, technicianID uuid.UUID) (domainfleet.Technician, error)

	GetTechnicianAssignment(ctx context.Context, companyID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error)
	UpdateTechnicianAssignment(ctx context.Context, in UpdateAssignmentHTTPInput) (domainfleet.TechnicianMachineAssignment, error)
	ReleaseTechnicianAssignment(ctx context.Context, companyID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error)
	ReleaseTechnicianAssignmentForMachineUser(ctx context.Context, companyID, machineID, technicianID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error)

	DisableMachine(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error)
	EnableMachine(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error)
	RetireMachine(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error)
	MarkMachineCompromised(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error)
	RotateMachineCredential(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error)
	RevokeMachineCredential(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error)
	RevokeMachineSessions(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error)
}

// InsertSiteParams is persisted site metadata for admin create.
type InsertSiteParams struct {
	RegionID *uuid.UUID
	Name     string
	Address  []byte
	Timezone string
	Code     string
}

// ListSitesParams filters admin site listing.
type ListSitesParams struct {
	StatusFilter *string
	Limit        int32
	Offset       int32
}

// UpdateSiteParams replaces mutable site columns (HTTP layer merges PATCH).
type UpdateSiteParams struct {
	SiteID   uuid.UUID
	RegionID *uuid.UUID
	Name     string
	Address  []byte
	Timezone string
	Code     string
	Status   string
}

// InsertTechnicianParams creates a technician row.
type InsertTechnicianParams struct {
	DisplayName     string
	Email           string
	Phone           string
	ExternalSubject string
}

// ListTechniciansParams filters technician directory listing.
type ListTechniciansParams struct {
	TechnicianID *uuid.UUID
	StatusFilter *string
	Search       string
	Limit        int32
	Offset       int32
}

// UpdateTechnicianRowParams updates technician profile fields.
type UpdateTechnicianRowParams struct {
	TechnicianID    uuid.UUID
	DisplayName     string
	Email           string
	Phone           string
	ExternalSubject string
}

// UpdateAssignmentParams updates assignment role/window/status.
type UpdateAssignmentParams struct {
	AssignmentID uuid.UUID
	Role         string
	ValidTo      *time.Time
	Status       string
}

// CreateSiteInput is the application input for POST /v1/admin/sites.
type CreateSiteInput struct {
	RegionID *uuid.UUID
	Name     string
	Address  []byte
	Timezone string
	Code     string
}

// UpdateSiteInput applies PATCH semantics for a site.
type UpdateSiteInput struct {
	SiteID   uuid.UUID
	RegionID *uuid.UUID
	Name     *string
	Address  []byte // nil = omit; empty slice = clear to {}
	Timezone *string
	Code     *string
	Status   *string
}

// ListSitesInput lists sites with pagination.
type ListSitesInput struct {
	Status *string
	Limit  int32
	Offset int32
}

// CreateTechnicianInput is POST /v1/admin/technicians.
type CreateTechnicianInput struct {
	DisplayName     string
	Email           string
	Phone           string
	ExternalSubject string
}

// UpdateTechnicianInput PATCH /v1/admin/technicians/{id}.
type UpdateTechnicianInput struct {
	TechnicianID    uuid.UUID
	DisplayName     *string
	Email           *string
	Phone           *string
	ExternalSubject *string
}

// ListTechniciansInput lists technicians with pagination.
type ListTechniciansInput struct {
	TechnicianID *uuid.UUID
	Status       *string
	Search       string
	Limit        int32
	Offset       int32
}

// UpdateAssignmentHTTPInput is PATCH /v1/admin/technician-assignments/{id}.
type UpdateAssignmentHTTPInput struct {
	AssignmentID uuid.UUID
	Role         *string
	ValidTo      *time.Time
	Status       *string
}
