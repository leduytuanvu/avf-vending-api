package fleet

import (
	"context"
	"errors"
	"strings"

	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/google/uuid"
)

var allowedMachineStatuses = map[string]struct{}{
	"draft":          {},
	"provisioned":    {},
	"active":         {},
	"maintenance":    {},
	"suspended":      {},
	"retired":        {},
	"decommissioned": {},
	"compromised":    {},
	"provisioning":   {},
	"online":         {},
	"offline":        {},
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

// UpdateMachineMetadataInput updates human-facing metadata for an existing machine within org scope.
type UpdateMachineMetadataInput struct {
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

// AssignTechnicianInput binds a technician to a machine with a role label.
type AssignTechnicianInput struct {
	MachineID    uuid.UUID
	TechnicianID uuid.UUID
	Role         string
	Scope        string
	CreatedBy    *uuid.UUID
	// ActorTechnicianID is the caller's technician identity from JWT (if any). When it matches TechnicianID, assignment is rejected.
	ActorTechnicianID uuid.UUID
}

// CreateMachine validates scope and inserts a machine row.
func (s *Service) CreateMachine(ctx context.Context, in CreateMachineInput) (domainfleet.Machine, error) {
	serial := strings.TrimSpace(in.SerialNumber)
	if serial == "" {
		return domainfleet.Machine{}, errors.Join(ErrInvalidArgument, errors.New("serial_number is required"))
	}
	if strings.TrimSpace(in.Status) == "" {
		in.Status = "draft"
	}
	if err := validateMachineStatus(in.Status); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := validateNonZero("site_id", in.SiteID); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := s.repo.AssertSiteInCompany(ctx, uuid.Nil, in.SiteID); err != nil {
		return domainfleet.Machine{}, err
	}
	return s.repo.InsertMachine(ctx, InsertMachineParams{
		SiteID:            in.SiteID,
		HardwareProfileID: in.HardwareProfileID,
		SerialNumber:      serial,
		Code:              strings.TrimSpace(in.Code),
		Model:             strings.TrimSpace(in.Model),
		CabinetType:       strings.TrimSpace(in.CabinetType),
		Timezone:          strings.TrimSpace(in.Timezone),
		Name:              strings.TrimSpace(in.Name),
		Status:            in.Status,
	})
}

// UpdateMachineMetadata loads the machine, enforces company scope, and applies a partial update.
func (s *Service) UpdateMachineMetadata(ctx context.Context, in UpdateMachineMetadataInput) (domainfleet.Machine, error) {
	if err := validateNonZero("machine_id", in.MachineID); err != nil {
		return domainfleet.Machine{}, err
	}
	if in.Name == nil && in.Status == nil && in.HardwareProfileID == nil && in.SiteID == nil && in.SerialNumber == nil && in.Code == nil && in.Model == nil && in.CabinetType == nil && in.Timezone == nil {
		return domainfleet.Machine{}, errors.Join(ErrInvalidArgument, errors.New("at least one field must be updated"))
	}
	if in.Status != nil {
		if err := validateMachineStatus(*in.Status); err != nil {
			return domainfleet.Machine{}, err
		}
	}
	_, err := s.repo.GetMachine(ctx, in.MachineID)
	if err != nil {
		return domainfleet.Machine{}, err
	}
	if in.SiteID != nil {
		if err := s.repo.AssertSiteInCompany(ctx, uuid.Nil, *in.SiteID); err != nil {
			return domainfleet.Machine{}, err
		}
	}
	return s.repo.UpdateMachineMetadata(ctx, UpdateMachineMetadataParams{
		MachineID:         in.MachineID,
		Name:              trimStringPtr(in.Name),
		Status:            in.Status,
		HardwareProfileID: in.HardwareProfileID,
		SiteID:            in.SiteID,
		SerialNumber:      trimStringPtr(in.SerialNumber),
		Code:              trimStringPtr(in.Code),
		Model:             trimStringPtr(in.Model),
		CabinetType:       trimStringPtr(in.CabinetType),
		Timezone:          trimStringPtr(in.Timezone),
	})
}

// AssignTechnicianToMachine verifies both aggregates belong to the company and creates an assignment.
func (s *Service) AssignTechnicianToMachine(ctx context.Context, in AssignTechnicianInput) (domainfleet.TechnicianMachineAssignment, error) {
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
	if in.ActorTechnicianID != uuid.Nil && in.ActorTechnicianID == in.TechnicianID {
		return domainfleet.TechnicianMachineAssignment{}, ErrForbiddenTechnicianSelfAssignment
	}
	_, err := s.repo.GetMachine(ctx, in.MachineID)
	if err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	_, err = s.repo.GetTechnician(ctx, in.TechnicianID)
	if err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return s.repo.InsertTechnicianMachineAssignment(ctx, InsertAssignmentParams{
		TechnicianID: in.TechnicianID,
		MachineID:    in.MachineID,
		Role:         role,
		Scope:        strings.TrimSpace(in.Scope),
		CreatedBy:    in.CreatedBy,
	})
}

// ListMachinesInScope returns machines for an company, optionally filtered to a single site.
func (s *Service) ListMachinesInScope(ctx context.Context, scope ListMachinesScope) ([]domainfleet.Machine, error) {
	if scope.SiteID != nil {
		if err := validateNonZero("site_id", *scope.SiteID); err != nil {
			return nil, err
		}
		if err := s.repo.AssertSiteInCompany(ctx, uuid.Nil, *scope.SiteID); err != nil {
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
