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
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domainfleet.Site{}, errors.Join(ErrInvalidArgument, errors.New("name is required"))
	}
	addr := in.Address
	if len(addr) == 0 {
		addr = []byte("{}")
	}
	return s.repo.InsertSite(ctx, InsertSiteParams{
		RegionID: in.RegionID,
		Name:     name,
		Address:  addr,
		Timezone: strings.TrimSpace(in.Timezone),
		Code:     strings.TrimSpace(in.Code),
	})
}

// GetSite returns a site by ID (single-tenant; companyID is unused but kept for API stability).
func (s *Service) GetSite(ctx context.Context, companyID, siteID uuid.UUID) (domainfleet.Site, error) {
	if err := validateNonZero("site_id", siteID); err != nil {
		return domainfleet.Site{}, err
	}
	return s.repo.GetSiteForOrg(ctx, companyID, siteID)
}

// ListSites returns paginated sites for an company.
func (s *Service) ListSites(ctx context.Context, in ListSitesInput) ([]domainfleet.Site, int64, error) {
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
		StatusFilter: st,
		Limit:        lim,
		Offset:       off,
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
	if err := validateNonZero("site_id", in.SiteID); err != nil {
		return domainfleet.Site{}, err
	}
	cur, err := s.repo.GetSiteForOrg(ctx, uuid.Nil, in.SiteID)
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
		SiteID:   in.SiteID,
		RegionID: regionID,
		Name:     name,
		Address:  addr,
		Timezone: tz,
		Code:     code,
		Status:   st,
	})
}

// DeactivateSite sets a site to archived when no non-retired machines reference it.
func (s *Service) DeactivateSite(ctx context.Context, companyID, siteID uuid.UUID) (domainfleet.Site, error) {
	if err := validateNonZero("site_id", siteID); err != nil {
		return domainfleet.Site{}, err
	}
	n, err := s.repo.CountNonRetiredMachinesForSite(ctx, companyID, siteID)
	if err != nil {
		return domainfleet.Site{}, err
	}
	if n > 0 {
		return domainfleet.Site{}, ErrSiteHasMachines
	}
	cur, err := s.repo.GetSiteForOrg(ctx, companyID, siteID)
	if err != nil {
		return domainfleet.Site{}, err
	}
	return s.repo.UpdateSite(ctx, UpdateSiteParams{
		SiteID:   siteID,
		RegionID: cur.RegionID,
		Name:     cur.Name,
		Address:  cur.Address,
		Timezone: cur.Timezone,
		Code:     cur.Code,
		Status:   "archived",
	})
}

// CreateTechnician inserts a technician.
func (s *Service) CreateTechnician(ctx context.Context, in CreateTechnicianInput) (domainfleet.Technician, error) {
	if strings.TrimSpace(in.DisplayName) == "" {
		return domainfleet.Technician{}, errors.Join(ErrInvalidArgument, errors.New("display_name is required"))
	}
	return s.repo.InsertTechnicianRow(ctx, InsertTechnicianParams{
		DisplayName:     in.DisplayName,
		Email:           in.Email,
		Phone:           in.Phone,
		ExternalSubject: in.ExternalSubject,
	})
}

// GetTechnician returns a technician in org scope.
func (s *Service) GetTechnician(ctx context.Context, companyID, technicianID uuid.UUID) (domainfleet.Technician, error) {
	if err := validateNonZero("technician_id", technicianID); err != nil {
		return domainfleet.Technician{}, err
	}
	return s.repo.GetTechnicianForOrg(ctx, companyID, technicianID)
}

// ListTechnicians lists technicians with pagination.
func (s *Service) ListTechnicians(ctx context.Context, in ListTechniciansInput) ([]domainfleet.Technician, int64, error) {
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
		TechnicianID: in.TechnicianID,
		StatusFilter: st,
		Search:       in.Search,
		Limit:        lim,
		Offset:       off,
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
	if err := validateNonZero("technician_id", in.TechnicianID); err != nil {
		return domainfleet.Technician{}, err
	}
	cur, err := s.repo.GetTechnicianForOrg(ctx, uuid.Nil, in.TechnicianID)
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
func (s *Service) DisableTechnician(ctx context.Context, companyID, technicianID uuid.UUID) (domainfleet.Technician, error) {
	if err := validateNonZero("technician_id", technicianID); err != nil {
		return domainfleet.Technician{}, err
	}
	return s.repo.SetTechnicianStatus(ctx, companyID, technicianID, "inactive")
}

// EnableTechnician sets technician status to active.
func (s *Service) EnableTechnician(ctx context.Context, companyID, technicianID uuid.UUID) (domainfleet.Technician, error) {
	if err := validateNonZero("technician_id", technicianID); err != nil {
		return domainfleet.Technician{}, err
	}
	return s.repo.SetTechnicianStatus(ctx, companyID, technicianID, "active")
}

// GetTechnicianAssignment returns one assignment in org scope.
func (s *Service) GetTechnicianAssignment(ctx context.Context, companyID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	if err := validateNonZero("assignment_id", assignmentID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return s.repo.GetTechnicianAssignmentForOrg(ctx, companyID, assignmentID)
}

// UpdateTechnicianAssignment applies PATCH to an assignment row.
func (s *Service) UpdateTechnicianAssignment(ctx context.Context, in UpdateAssignmentHTTPInput) (domainfleet.TechnicianMachineAssignment, error) {
	if err := validateNonZero("assignment_id", in.AssignmentID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	cur, err := s.repo.GetTechnicianAssignmentForOrg(ctx, uuid.Nil, in.AssignmentID)
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
		AssignmentID: in.AssignmentID,
		Role:         role,
		ValidTo:      vto,
		Status:       st,
	})
}

// ReleaseTechnicianAssignment ends an assignment (released + valid_to).
func (s *Service) ReleaseTechnicianAssignment(ctx context.Context, companyID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	if err := validateNonZero("assignment_id", assignmentID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return s.repo.ReleaseTechnicianAssignment(ctx, companyID, assignmentID)
}

// ReleaseTechnicianAssignmentForMachineUser ends an active assignment for the nested machine technician API.
func (s *Service) ReleaseTechnicianAssignmentForMachineUser(ctx context.Context, companyID, machineID, technicianID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	if err := validateNonZero("machine_id", machineID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	if err := validateNonZero("technician_id", technicianID); err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return s.repo.ReleaseTechnicianAssignmentForMachineUser(ctx, companyID, machineID, technicianID)
}

// DisableMachine sets machine status to suspended.
func (s *Service) DisableMachine(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error) {
	if err := ValidateLifecycleMutation("suspend", in, false); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	cur, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	prev := cur.Status
	st := "suspended"
	m, err := s.UpdateMachineMetadata(ctx, UpdateMachineMetadataInput{
		MachineID: machineID,
		Status:    &st,
	})
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	return LifecycleMutationOutcome{Machine: m, Result: lifecycleResult(prev, st, m, in)}, nil
}

// EnableMachine returns a suspended machine to active runtime state. Retired and compromised machines are terminal.
func (s *Service) EnableMachine(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error) {
	if err := validateNonZero("machine_id", machineID); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	cur, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	if uuid.Nil != companyID {
		return LifecycleMutationOutcome{}, ErrOrgMismatch
	}
	if strings.EqualFold(strings.TrimSpace(cur.Status), "retired") || strings.EqualFold(strings.TrimSpace(cur.Status), "decommissioned") || strings.EqualFold(strings.TrimSpace(cur.Status), "compromised") {
		return LifecycleMutationOutcome{}, errors.Join(ErrConflict, errors.New("terminal machines cannot be enabled"))
	}
	prev := cur.Status
	st := "active"
	m, err := s.UpdateMachineMetadata(ctx, UpdateMachineMetadataInput{
		MachineID: machineID,
		Status:    &st,
	})
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	return LifecycleMutationOutcome{Machine: m, Result: lifecycleResult(prev, st, m, in)}, nil
}

// RetireMachine sets machine status to decommissioned (terminal operational retirement).
func (s *Service) RetireMachine(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error) {
	if err := ValidateLifecycleMutation("archive", in, false); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	cur, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	prev := cur.Status
	st := "decommissioned"
	m, err := s.UpdateMachineMetadata(ctx, UpdateMachineMetadataInput{
		MachineID: machineID,
		Status:    &st,
	})
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	return LifecycleMutationOutcome{Machine: m, Result: lifecycleResult(prev, st, m, in)}, nil
}

// MarkMachineCompromised blocks machine runtime authentication and revokes credentials.
func (s *Service) MarkMachineCompromised(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error) {
	if err := ValidateLifecycleMutation("mark-compromised", in, false); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	cur, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	prev := cur.Status
	st := "compromised"
	if _, err := s.UpdateMachineMetadata(ctx, UpdateMachineMetadataInput{
		MachineID: machineID,
		Status:    &st,
	}); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	m, err := s.repo.RevokeMachineCredentialLifecycle(ctx, companyID, machineID, true)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	out := lifecycleResult(prev, st, m, in)
	out.SessionsRevokedCount = 1
	out.CredentialsRevokedCount = 1
	return LifecycleMutationOutcome{Machine: m, Result: out}, nil
}

// RotateMachineCredential bumps credential_version and revokes active activation codes.
func (s *Service) RotateMachineCredential(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error) {
	if err := ValidateLifecycleMutation("rotate-credentials", in, false); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	if err := validateNonZero("machine_id", machineID); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	cur, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	prev := cur.Status
	if uuid.Nil != companyID {
		return LifecycleMutationOutcome{}, ErrOrgMismatch
	}
	m, err := s.repo.RotateMachineCredentialLifecycle(ctx, companyID, machineID)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	out := lifecycleResult(prev, m.Status, m, in)
	out.SessionsRevokedCount = 1
	return LifecycleMutationOutcome{Machine: m, Result: out}, nil
}

// RevokeMachineCredential invalidates current machine JWTs until credentials are rotated again.
func (s *Service) RevokeMachineCredential(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error) {
	if err := ValidateLifecycleMutation("revoke-credentials", in, false); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	if err := validateNonZero("machine_id", machineID); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	cur, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	prev := cur.Status
	if uuid.Nil != companyID {
		return LifecycleMutationOutcome{}, ErrOrgMismatch
	}
	m, err := s.repo.RevokeMachineCredentialLifecycle(ctx, companyID, machineID, false)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	out := lifecycleResult(prev, m.Status, m, in)
	out.CredentialsRevokedCount = 1
	return LifecycleMutationOutcome{Machine: m, Result: out}, nil
}

// RevokeMachineSessions invalidates all active machine refresh sessions without rotating credentials.
func (s *Service) RevokeMachineSessions(ctx context.Context, companyID, machineID uuid.UUID, in LifecycleMutationInput) (LifecycleMutationOutcome, error) {
	if err := ValidateLifecycleMutation("revoke-sessions", in, false); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	if err := validateNonZero("machine_id", machineID); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	cur, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	if uuid.Nil != companyID {
		return LifecycleMutationOutcome{}, ErrOrgMismatch
	}
	if err := s.repo.RevokeAllMachineSessionsOnly(ctx, companyID, machineID); err != nil {
		return LifecycleMutationOutcome{}, err
	}
	m, err := s.repo.GetMachine(ctx, machineID)
	if err != nil {
		return LifecycleMutationOutcome{}, err
	}
	out := lifecycleResult(cur.Status, m.Status, m, in)
	out.SessionsRevokedCount = 1
	return LifecycleMutationOutcome{Machine: m, Result: out}, nil
}
