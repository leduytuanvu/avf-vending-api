package fleet

import (
	"context"
	"errors"
	"strings"

	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/google/uuid"
)

var allowedMachineStatuses = map[string]struct{}{
	"provisioning": {},
	"online":       {},
	"offline":      {},
	"maintenance":  {},
	"retired":      {},
}

// Service orchestrates fleet workflows on top of FleetRepository.
type Service struct {
	repo FleetRepository
}

// NewService returns a fleet application service backed by repo. Repo must not be nil.
func NewService(repo FleetRepository) *Service {
	if repo == nil {
		panic("fleet.NewService: nil FleetRepository")
	}
	return &Service{repo: repo}
}

var _ FleetWorkflows = (*Service)(nil)

// CreateMachineInput describes provisioning inputs supplied by an authenticated admin context.
type CreateMachineInput struct {
	OrganizationID    uuid.UUID
	SiteID            uuid.UUID
	HardwareProfileID *uuid.UUID
	SerialNumber      string
	Name              string
	Status            string
}

// UpdateMachineMetadataInput updates human-facing metadata for an existing machine within org scope.
type UpdateMachineMetadataInput struct {
	OrganizationID    uuid.UUID
	MachineID         uuid.UUID
	Name              *string
	Status            *string
	HardwareProfileID *uuid.UUID
}

// AssignTechnicianInput binds a technician to a machine with a role label.
type AssignTechnicianInput struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	TechnicianID   uuid.UUID
	Role           string
}

// CreateMachine validates scope and inserts a machine row.
func (s *Service) CreateMachine(ctx context.Context, in CreateMachineInput) (domainfleet.Machine, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := validateNonZero("site_id", in.SiteID); err != nil {
		return domainfleet.Machine{}, err
	}
	serial := strings.TrimSpace(in.SerialNumber)
	if serial == "" {
		return domainfleet.Machine{}, errors.Join(ErrInvalidArgument, errors.New("serial_number is required"))
	}
	if err := validateMachineStatus(in.Status); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := s.repo.AssertSiteInOrganization(ctx, in.OrganizationID, in.SiteID); err != nil {
		return domainfleet.Machine{}, err
	}
	// TODO: When hardware profiles are exposed on FleetRepository, assert profile belongs to organization_id.
	return s.repo.InsertMachine(ctx, InsertMachineParams{
		OrganizationID:    in.OrganizationID,
		SiteID:            in.SiteID,
		HardwareProfileID: in.HardwareProfileID,
		SerialNumber:      serial,
		Name:              strings.TrimSpace(in.Name),
		Status:            in.Status,
	})
}

// UpdateMachineMetadata loads the machine, enforces organization scope, and applies a partial update.
func (s *Service) UpdateMachineMetadata(ctx context.Context, in UpdateMachineMetadataInput) (domainfleet.Machine, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := validateNonZero("machine_id", in.MachineID); err != nil {
		return domainfleet.Machine{}, err
	}
	if in.Name == nil && in.Status == nil && in.HardwareProfileID == nil {
		return domainfleet.Machine{}, errors.Join(ErrInvalidArgument, errors.New("at least one field must be updated"))
	}
	if in.Status != nil {
		if err := validateMachineStatus(*in.Status); err != nil {
			return domainfleet.Machine{}, err
		}
	}
	current, err := s.repo.GetMachine(ctx, in.MachineID)
	if err != nil {
		return domainfleet.Machine{}, err
	}
	if current.OrganizationID != in.OrganizationID {
		return domainfleet.Machine{}, ErrOrgMismatch
	}
	return s.repo.UpdateMachineMetadata(ctx, UpdateMachineMetadataParams{
		OrganizationID:    in.OrganizationID,
		MachineID:         in.MachineID,
		Name:              trimStringPtr(in.Name),
		Status:            in.Status,
		HardwareProfileID: in.HardwareProfileID,
	})
}

// AssignTechnicianToMachine verifies both aggregates belong to the organization and creates an assignment.
func (s *Service) AssignTechnicianToMachine(ctx context.Context, in AssignTechnicianInput) (domainfleet.TechnicianMachineAssignment, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	if err := validateNonZero("machine_id", in.MachineID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	if err := validateNonZero("technician_id", in.TechnicianID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	role := strings.TrimSpace(in.Role)
	if role == "" {
		return domainfleet.TechnicianMachineAssignment{}, errors.Join(ErrInvalidArgument, errors.New("role is required"))
	}
	machine, err := s.repo.GetMachine(ctx, in.MachineID)
	if err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	technician, err := s.repo.GetTechnician(ctx, in.TechnicianID)
	if err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	if machine.OrganizationID != in.OrganizationID || technician.OrganizationID != in.OrganizationID {
		return domainfleet.TechnicianMachineAssignment{}, ErrOrgMismatch
	}
	return s.repo.InsertTechnicianMachineAssignment(ctx, InsertAssignmentParams{
		TechnicianID: in.TechnicianID,
		MachineID:    in.MachineID,
		Role:         role,
	})
}

// ListMachinesInScope returns machines for an organization, optionally filtered to a single site.
func (s *Service) ListMachinesInScope(ctx context.Context, scope ListMachinesScope) ([]domainfleet.Machine, error) {
	if err := validateNonZero("organization_id", scope.OrganizationID); err != nil {
		return nil, err
	}
	if scope.SiteID != nil {
		if err := validateNonZero("site_id", *scope.SiteID); err != nil {
			return nil, err
		}
		if err := s.repo.AssertSiteInOrganization(ctx, scope.OrganizationID, *scope.SiteID); err != nil {
			return nil, err
		}
	}
	return s.repo.ListMachinesInScope(ctx, scope)
}

func validateNonZero(field string, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.Join(ErrInvalidArgument, errors.New(field+" must be set"))
	}
	return nil
}

func validateMachineStatus(status string) error {
	if _, ok := allowedMachineStatuses[status]; !ok {
		return errors.Join(ErrInvalidArgument, errors.New("invalid machine status"))
	}
	return nil
}

func trimStringPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	return &v
}
