package fleet

import (
	"context"
	"errors"
	"strings"
	"time"

	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/google/uuid"
)

const (
	maxAdminFleetPageSize int32 = 500
)

func clampLimit(lim int32) int32 {
	if lim <= 0 {
		return 50
	}
	if lim > maxAdminFleetPageSize {
		return maxAdminFleetPageSize
	}
	return lim
}

func normalizeSiteStatus(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "inactive" {
		return "archived"
	}
	return v
}

func validateSiteStatus(status string) error {
	switch normalizeSiteStatus(status) {
	case "active", "archived":
		return nil
	default:
		return errors.Join(ErrInvalidArgument, errors.New("invalid site status"))
	}
}

func validateTechnicianStatus(status string) error {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "inactive":
		return nil
	default:
		return errors.Join(ErrInvalidArgument, errors.New("invalid technician status"))
	}
}

func validateAssignmentStatus(status string) error {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "released":
		return nil
	default:
		return errors.Join(ErrInvalidArgument, errors.New("invalid assignment status"))
	}
}

// CreateSite creates a site row.
func (s *Service) CreateSite(ctx context.Context, in CreateSiteInput) (domainfleet.Site, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return domainfleet.Site{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domainfleet.Site{}, errors.Join(ErrInvalidArgument, errors.New("name is required"))
	}
	addr := in.Address
	if len(addr) == 0 {
		addr = []byte("{}")
	}
	return s.repo.InsertSite(ctx, InsertSiteParams{
		OrganizationID: in.OrganizationID,
		RegionID:       in.RegionID,
		Name:           name,
		Address:        addr,
		Timezone:       strings.TrimSpace(in.Timezone),
		Code:           strings.TrimSpace(in.Code),
	})
}

// GetSite returns a site scoped to the organization.
func (s *Service) GetSite(ctx context.Context, organizationID, siteID uuid.UUID) (domainfleet.Site, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.Site{}, err
	}
	if err := validateNonZero("site_id", siteID); err != nil {
		return domainfleet.Site{}, err
	}
	return s.repo.GetSiteForOrg(ctx, organizationID, siteID)
}

// ListSites returns paginated sites for an organization.
func (s *Service) ListSites(ctx context.Context, in ListSitesInput) ([]domainfleet.Site, int64, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return nil, 0, err
	}
	lim := clampLimit(in.Limit)
	off := in.Offset
	if off < 0 {
		off = 0
	}
	var st *string
	if in.Status != nil {
		v := normalizeSiteStatus(strings.TrimSpace(*in.Status))
		if v != "" {
			if err := validateSiteStatus(v); err != nil {
				return nil, 0, err
			}
			st = &v
		}
	}
	p := ListSitesParams{
		OrganizationID: in.OrganizationID,
		StatusFilter:   st,
		Limit:          lim,
		Offset:         off,
	}
	total, err := s.repo.CountSitesForOrg(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.repo.ListSitesForOrg(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// UpdateSite merges a PATCH into the current site row.
func (s *Service) UpdateSite(ctx context.Context, in UpdateSiteInput) (domainfleet.Site, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return domainfleet.Site{}, err
	}
	if err := validateNonZero("site_id", in.SiteID); err != nil {
		return domainfleet.Site{}, err
	}
	cur, err := s.repo.GetSiteForOrg(ctx, in.OrganizationID, in.SiteID)
	if err != nil {
		return domainfleet.Site{}, err
	}
	regionID := cur.RegionID
	if in.RegionID != nil {
		regionID = in.RegionID
	}
	name := cur.Name
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" {
			return domainfleet.Site{}, errors.Join(ErrInvalidArgument, errors.New("name cannot be empty"))
		}
	}
	addr := cur.Address
	if in.Address != nil {
		addr = in.Address
		if len(addr) == 0 {
			addr = []byte("{}")
		}
	}
	tz := cur.Timezone
	if in.Timezone != nil {
		tz = strings.TrimSpace(*in.Timezone)
	}
	code := cur.Code
	if in.Code != nil {
		code = strings.TrimSpace(*in.Code)
	}
	st := cur.Status
	if in.Status != nil {
		v := normalizeSiteStatus(strings.TrimSpace(*in.Status))
		if err := validateSiteStatus(v); err != nil {
			return domainfleet.Site{}, err
		}
		st = v
	}
	return s.repo.UpdateSite(ctx, UpdateSiteParams{
		OrganizationID: in.OrganizationID,
		SiteID:         in.SiteID,
		RegionID:       regionID,
		Name:           name,
		Address:        addr,
		Timezone:       tz,
		Code:           code,
		Status:         st,
	})
}

// DeactivateSite sets a site to archived when no non-retired machines reference it.
func (s *Service) DeactivateSite(ctx context.Context, organizationID, siteID uuid.UUID) (domainfleet.Site, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.Site{}, err
	}
	if err := validateNonZero("site_id", siteID); err != nil {
		return domainfleet.Site{}, err
	}
	n, err := s.repo.CountNonRetiredMachinesForSite(ctx, organizationID, siteID)
	if err != nil {
		return domainfleet.Site{}, err
	}
	if n > 0 {
		return domainfleet.Site{}, ErrSiteHasMachines
	}
	cur, err := s.repo.GetSiteForOrg(ctx, organizationID, siteID)
	if err != nil {
		return domainfleet.Site{}, err
	}
	return s.repo.UpdateSite(ctx, UpdateSiteParams{
		OrganizationID: organizationID,
		SiteID:         siteID,
		RegionID:       cur.RegionID,
		Name:           cur.Name,
		Address:        cur.Address,
		Timezone:       cur.Timezone,
		Code:           cur.Code,
		Status:         "archived",
	})
}

// CreateTechnician inserts a technician.
func (s *Service) CreateTechnician(ctx context.Context, in CreateTechnicianInput) (domainfleet.Technician, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return domainfleet.Technician{}, err
	}
	if strings.TrimSpace(in.DisplayName) == "" {
		return domainfleet.Technician{}, errors.Join(ErrInvalidArgument, errors.New("display_name is required"))
	}
	return s.repo.InsertTechnicianRow(ctx, InsertTechnicianParams{
		OrganizationID:  in.OrganizationID,
		DisplayName:     in.DisplayName,
		Email:           in.Email,
		Phone:           in.Phone,
		ExternalSubject: in.ExternalSubject,
	})
}

// GetTechnician returns a technician in org scope.
func (s *Service) GetTechnician(ctx context.Context, organizationID, technicianID uuid.UUID) (domainfleet.Technician, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.Technician{}, err
	}
	if err := validateNonZero("technician_id", technicianID); err != nil {
		return domainfleet.Technician{}, err
	}
	return s.repo.GetTechnicianForOrg(ctx, organizationID, technicianID)
}

// ListTechnicians lists technicians with pagination.
func (s *Service) ListTechnicians(ctx context.Context, in ListTechniciansInput) ([]domainfleet.Technician, int64, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return nil, 0, err
	}
	lim := clampLimit(in.Limit)
	off := in.Offset
	if off < 0 {
		off = 0
	}
	var st *string
	if in.Status != nil {
		v := strings.TrimSpace(*in.Status)
		if v != "" {
			if err := validateTechnicianStatus(v); err != nil {
				return nil, 0, err
			}
			st = &v
		}
	}
	p := ListTechniciansParams{
		OrganizationID: in.OrganizationID,
		TechnicianID:   in.TechnicianID,
		StatusFilter:   st,
		Search:         in.Search,
		Limit:          lim,
		Offset:         off,
	}
	total, err := s.repo.CountTechniciansForOrg(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.repo.ListTechniciansForOrg(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// UpdateTechnician applies PATCH fields.
func (s *Service) UpdateTechnician(ctx context.Context, in UpdateTechnicianInput) (domainfleet.Technician, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return domainfleet.Technician{}, err
	}
	if err := validateNonZero("technician_id", in.TechnicianID); err != nil {
		return domainfleet.Technician{}, err
	}
	cur, err := s.repo.GetTechnicianForOrg(ctx, in.OrganizationID, in.TechnicianID)
	if err != nil {
		return domainfleet.Technician{}, err
	}
	name := cur.DisplayName
	if in.DisplayName != nil {
		name = strings.TrimSpace(*in.DisplayName)
		if name == "" {
			return domainfleet.Technician{}, errors.Join(ErrInvalidArgument, errors.New("display_name cannot be empty"))
		}
	}
	email := derefString(cur.Email)
	if in.Email != nil {
		email = strings.TrimSpace(*in.Email)
	}
	phone := derefString(cur.Phone)
	if in.Phone != nil {
		phone = strings.TrimSpace(*in.Phone)
	}
	ext := derefString(cur.ExternalSubject)
	if in.ExternalSubject != nil {
		ext = strings.TrimSpace(*in.ExternalSubject)
	}
	return s.repo.UpdateTechnicianRow(ctx, UpdateTechnicianRowParams{
		OrganizationID:  in.OrganizationID,
		TechnicianID:    in.TechnicianID,
		DisplayName:     name,
		Email:           email,
		Phone:           phone,
		ExternalSubject: ext,
	})
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// DisableTechnician sets technician status to inactive.
func (s *Service) DisableTechnician(ctx context.Context, organizationID, technicianID uuid.UUID) (domainfleet.Technician, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.Technician{}, err
	}
	if err := validateNonZero("technician_id", technicianID); err != nil {
		return domainfleet.Technician{}, err
	}
	return s.repo.SetTechnicianStatus(ctx, organizationID, technicianID, "inactive")
}

// EnableTechnician sets technician status to active.
func (s *Service) EnableTechnician(ctx context.Context, organizationID, technicianID uuid.UUID) (domainfleet.Technician, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.Technician{}, err
	}
	if err := validateNonZero("technician_id", technicianID); err != nil {
		return domainfleet.Technician{}, err
	}
	return s.repo.SetTechnicianStatus(ctx, organizationID, technicianID, "active")
}

// GetTechnicianAssignment returns one assignment in org scope.
func (s *Service) GetTechnicianAssignment(ctx context.Context, organizationID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	if err := validateNonZero("assignment_id", assignmentID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return s.repo.GetTechnicianAssignmentForOrg(ctx, organizationID, assignmentID)
}

// UpdateTechnicianAssignment applies PATCH to an assignment row.
func (s *Service) UpdateTechnicianAssignment(ctx context.Context, in UpdateAssignmentHTTPInput) (domainfleet.TechnicianMachineAssignment, error) {
	if err := validateNonZero("organization_id", in.OrganizationID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	if err := validateNonZero("assignment_id", in.AssignmentID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	cur, err := s.repo.GetTechnicianAssignmentForOrg(ctx, in.OrganizationID, in.AssignmentID)
	if err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	role := cur.Role
	if in.Role != nil {
		role = strings.TrimSpace(*in.Role)
		if role == "" {
			return domainfleet.TechnicianMachineAssignment{}, errors.Join(ErrInvalidArgument, errors.New("role cannot be empty"))
		}
	}
	st := cur.Status
	if in.Status != nil {
		v := strings.TrimSpace(*in.Status)
		if err := validateAssignmentStatus(v); err != nil {
			return domainfleet.TechnicianMachineAssignment{}, err
		}
		st = v
	}
	var vto *time.Time
	if in.ValidTo != nil {
		utc := in.ValidTo.UTC()
		vto = &utc
	} else {
		vto = cur.ValidTo
	}
	return s.repo.UpdateTechnicianAssignment(ctx, UpdateAssignmentParams{
		OrganizationID: in.OrganizationID,
		AssignmentID:   in.AssignmentID,
		Role:           role,
		ValidTo:        vto,
		Status:         st,
	})
}

// ReleaseTechnicianAssignment ends an assignment (released + valid_to).
func (s *Service) ReleaseTechnicianAssignment(ctx context.Context, organizationID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	if err := validateNonZero("assignment_id", assignmentID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return s.repo.ReleaseTechnicianAssignment(ctx, organizationID, assignmentID)
}

// ReleaseTechnicianAssignmentForMachineUser ends an active assignment for the nested machine technician API.
func (s *Service) ReleaseTechnicianAssignmentForMachineUser(ctx context.Context, organizationID, machineID, technicianID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	if err := validateNonZero("machine_id", machineID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	if err := validateNonZero("technician_id", technicianID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return s.repo.ReleaseTechnicianAssignmentForMachineUser(ctx, organizationID, machineID, technicianID)
}

// DisableMachine sets machine status to maintenance.
func (s *Service) DisableMachine(ctx context.Context, organizationID, machineID uuid.UUID) (domainfleet.Machine, error) {
	st := "suspended"
	return s.UpdateMachineMetadata(ctx, UpdateMachineMetadataInput{
		OrganizationID: organizationID,
		MachineID:      machineID,
		Status:         &st,
	})
}

// EnableMachine returns a suspended machine to active runtime state. Retired and compromised machines are terminal.
func (s *Service) EnableMachine(ctx context.Context, organizationID, machineID uuid.UUID) (domainfleet.Machine, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := validateNonZero("machine_id", machineID); err != nil {
		return domainfleet.Machine{}, err
	}
	cur, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return domainfleet.Machine{}, err
	}
	if cur.OrganizationID != organizationID {
		return domainfleet.Machine{}, ErrOrgMismatch
	}
	if strings.EqualFold(strings.TrimSpace(cur.Status), "retired") || strings.EqualFold(strings.TrimSpace(cur.Status), "decommissioned") || strings.EqualFold(strings.TrimSpace(cur.Status), "compromised") {
		return domainfleet.Machine{}, errors.Join(ErrConflict, errors.New("terminal machines cannot be enabled"))
	}
	st := "active"
	return s.UpdateMachineMetadata(ctx, UpdateMachineMetadataInput{
		OrganizationID: organizationID,
		MachineID:      machineID,
		Status:         &st,
	})
}

// RetireMachine sets machine status to decommissioned (terminal operational retirement).
func (s *Service) RetireMachine(ctx context.Context, organizationID, machineID uuid.UUID) (domainfleet.Machine, error) {
	st := "decommissioned"
	return s.UpdateMachineMetadata(ctx, UpdateMachineMetadataInput{
		OrganizationID: organizationID,
		MachineID:      machineID,
		Status:         &st,
	})
}

// MarkMachineCompromised blocks machine runtime authentication and revokes credentials.
func (s *Service) MarkMachineCompromised(ctx context.Context, organizationID, machineID uuid.UUID) (domainfleet.Machine, error) {
	st := "compromised"
	if _, err := s.UpdateMachineMetadata(ctx, UpdateMachineMetadataInput{
		OrganizationID: organizationID,
		MachineID:      machineID,
		Status:         &st,
	}); err != nil {
		return domainfleet.Machine{}, err
	}
	return s.repo.RevokeMachineCredentialLifecycle(ctx, organizationID, machineID, true)
}

// RotateMachineCredential bumps credential_version and revokes active activation codes.
func (s *Service) RotateMachineCredential(ctx context.Context, organizationID, machineID uuid.UUID) (domainfleet.Machine, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := validateNonZero("machine_id", machineID); err != nil {
		return domainfleet.Machine{}, err
	}
	m, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return domainfleet.Machine{}, err
	}
	if m.OrganizationID != organizationID {
		return domainfleet.Machine{}, ErrOrgMismatch
	}
	return s.repo.RotateMachineCredentialLifecycle(ctx, organizationID, machineID)
}

// RevokeMachineCredential invalidates current machine JWTs until credentials are rotated again.
func (s *Service) RevokeMachineCredential(ctx context.Context, organizationID, machineID uuid.UUID) (domainfleet.Machine, error) {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return domainfleet.Machine{}, err
	}
	if err := validateNonZero("machine_id", machineID); err != nil {
		return domainfleet.Machine{}, err
	}
	m, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return domainfleet.Machine{}, err
	}
	if m.OrganizationID != organizationID {
		return domainfleet.Machine{}, ErrOrgMismatch
	}
	return s.repo.RevokeMachineCredentialLifecycle(ctx, organizationID, machineID, false)
}

// RevokeMachineSessions invalidates all active machine refresh sessions without rotating credentials.
func (s *Service) RevokeMachineSessions(ctx context.Context, organizationID, machineID uuid.UUID) error {
	if err := validateNonZero("organization_id", organizationID); err != nil {
		return err
	}
	if err := validateNonZero("machine_id", machineID); err != nil {
		return err
	}
	m, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return err
	}
	if m.OrganizationID != organizationID {
		return ErrOrgMismatch
	}
	return s.repo.RevokeAllMachineSessionsOnly(ctx, organizationID, machineID)
}
