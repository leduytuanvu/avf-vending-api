package postgres

import (
	"context"
	"errors"
	"strings"

	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *fleetRepository) InsertSite(ctx context.Context, p appfleet.InsertSiteParams) (domainfleet.Site, error) {
	q := db.New(r.pool)
	row, err := q.AdminInsertSite(ctx, db.AdminInsertSiteParams{
		OrganizationID: p.OrganizationID,
		RegionID:       optionalUUIDToPg(p.RegionID),
		Name:           strings.TrimSpace(p.Name),
		Address:        p.Address,
		Timezone:       strings.TrimSpace(p.Timezone),
		Code:           strings.TrimSpace(p.Code),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domainfleet.Site{}, appfleet.ErrConflict
		}
		return domainfleet.Site{}, err
	}
	return mapFleetSite(row), nil
}

func (r *fleetRepository) GetSiteForOrg(ctx context.Context, organizationID, siteID uuid.UUID) (domainfleet.Site, error) {
	q := db.New(r.pool)
	row, err := q.AdminGetSiteForOrg(ctx, db.AdminGetSiteForOrgParams{
		ID:             siteID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Site{}, appfleet.ErrNotFound
		}
		return domainfleet.Site{}, err
	}
	return mapFleetSite(row), nil
}

func (r *fleetRepository) ListSitesForOrg(ctx context.Context, p appfleet.ListSitesParams) ([]domainfleet.Site, error) {
	q := db.New(r.pool)
	var filter bool
	var st string
	if p.StatusFilter != nil {
		filter = true
		st = strings.TrimSpace(*p.StatusFilter)
	}
	rows, err := q.AdminListSitesForOrg(ctx, db.AdminListSitesForOrgParams{
		OrganizationID: p.OrganizationID,
		Column2:        filter,
		Column3:        st,
		Limit:          p.Limit,
		Offset:         p.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainfleet.Site, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapFleetSite(row))
	}
	return out, nil
}

func (r *fleetRepository) CountSitesForOrg(ctx context.Context, p appfleet.ListSitesParams) (int64, error) {
	q := db.New(r.pool)
	var filter bool
	var st string
	if p.StatusFilter != nil {
		filter = true
		st = strings.TrimSpace(*p.StatusFilter)
	}
	n, err := q.AdminCountSitesForOrg(ctx, db.AdminCountSitesForOrgParams{
		OrganizationID: p.OrganizationID,
		Column2:        filter,
		Column3:        st,
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r *fleetRepository) UpdateSite(ctx context.Context, p appfleet.UpdateSiteParams) (domainfleet.Site, error) {
	q := db.New(r.pool)
	row, err := q.AdminUpdateSiteRow(ctx, db.AdminUpdateSiteRowParams{
		ID:             p.SiteID,
		OrganizationID: p.OrganizationID,
		RegionID:       optionalUUIDToPg(p.RegionID),
		Name:           strings.TrimSpace(p.Name),
		Address:        p.Address,
		Timezone:       strings.TrimSpace(p.Timezone),
		Code:           strings.TrimSpace(p.Code),
		Status:         strings.TrimSpace(p.Status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Site{}, appfleet.ErrNotFound
		}
		if isUniqueViolation(err) {
			return domainfleet.Site{}, appfleet.ErrConflict
		}
		return domainfleet.Site{}, err
	}
	return mapFleetSite(row), nil
}

func (r *fleetRepository) CountNonRetiredMachinesForSite(ctx context.Context, organizationID, siteID uuid.UUID) (int64, error) {
	q := db.New(r.pool)
	return q.AdminCountNonRetiredMachinesForSite(ctx, db.AdminCountNonRetiredMachinesForSiteParams{
		OrganizationID: organizationID,
		SiteID:         siteID,
	})
}

func (r *fleetRepository) InsertTechnicianRow(ctx context.Context, p appfleet.InsertTechnicianParams) (domainfleet.Technician, error) {
	q := db.New(r.pool)
	row, err := q.AdminInsertTechnician(ctx, db.AdminInsertTechnicianParams{
		OrganizationID: p.OrganizationID,
		DisplayName:    strings.TrimSpace(p.DisplayName),
		Column3:        p.Email,
		Column4:        p.Phone,
		Column5:        p.ExternalSubject,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domainfleet.Technician{}, appfleet.ErrConflict
		}
		return domainfleet.Technician{}, err
	}
	return mapTechnician(row), nil
}

func (r *fleetRepository) GetTechnicianForOrg(ctx context.Context, organizationID, technicianID uuid.UUID) (domainfleet.Technician, error) {
	q := db.New(r.pool)
	row, err := q.AdminGetTechnicianForOrg(ctx, db.AdminGetTechnicianForOrgParams{
		ID:             technicianID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Technician{}, appfleet.ErrNotFound
		}
		return domainfleet.Technician{}, err
	}
	return mapTechnician(row), nil
}

func (r *fleetRepository) ListTechniciansForOrg(ctx context.Context, p appfleet.ListTechniciansParams) ([]domainfleet.Technician, error) {
	q := db.New(r.pool)
	var idF bool
	var tid uuid.UUID
	if p.TechnicianID != nil {
		idF = true
		tid = *p.TechnicianID
	}
	var stF bool
	var st string
	if p.StatusFilter != nil {
		stF = true
		st = strings.TrimSpace(*p.StatusFilter)
	}
	search := strings.TrimSpace(p.Search)
	var searchF bool
	if search != "" {
		searchF = true
	}
	rows, err := q.AdminListTechniciansForOrg(ctx, db.AdminListTechniciansForOrgParams{
		OrganizationID: p.OrganizationID,
		Column2:        idF,
		Column3:        tid,
		Column4:        stF,
		Column5:        st,
		Column6:        searchF,
		Column7:        search,
		Limit:          p.Limit,
		Offset:         p.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domainfleet.Technician, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapTechnician(row))
	}
	return out, nil
}

func (r *fleetRepository) CountTechniciansForOrg(ctx context.Context, p appfleet.ListTechniciansParams) (int64, error) {
	q := db.New(r.pool)
	var idF bool
	var tid uuid.UUID
	if p.TechnicianID != nil {
		idF = true
		tid = *p.TechnicianID
	}
	var stF bool
	var st string
	if p.StatusFilter != nil {
		stF = true
		st = strings.TrimSpace(*p.StatusFilter)
	}
	search := strings.TrimSpace(p.Search)
	var searchF bool
	if search != "" {
		searchF = true
	}
	return q.AdminCountTechniciansForOrg(ctx, db.AdminCountTechniciansForOrgParams{
		OrganizationID: p.OrganizationID,
		Column2:        idF,
		Column3:        tid,
		Column4:        stF,
		Column5:        st,
		Column6:        searchF,
		Column7:        search,
	})
}

func (r *fleetRepository) UpdateTechnicianRow(ctx context.Context, p appfleet.UpdateTechnicianRowParams) (domainfleet.Technician, error) {
	q := db.New(r.pool)
	row, err := q.AdminUpdateTechnicianRow(ctx, db.AdminUpdateTechnicianRowParams{
		ID:             p.TechnicianID,
		OrganizationID: p.OrganizationID,
		DisplayName:    strings.TrimSpace(p.DisplayName),
		Column4:        p.Email,
		Column5:        p.Phone,
		Column6:        p.ExternalSubject,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Technician{}, appfleet.ErrNotFound
		}
		if isUniqueViolation(err) {
			return domainfleet.Technician{}, appfleet.ErrConflict
		}
		return domainfleet.Technician{}, err
	}
	return mapTechnician(row), nil
}

func (r *fleetRepository) SetTechnicianStatus(ctx context.Context, organizationID, technicianID uuid.UUID, status string) (domainfleet.Technician, error) {
	q := db.New(r.pool)
	row, err := q.AdminSetTechnicianStatus(ctx, db.AdminSetTechnicianStatusParams{
		ID:             technicianID,
		OrganizationID: organizationID,
		Status:         strings.TrimSpace(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Technician{}, appfleet.ErrNotFound
		}
		return domainfleet.Technician{}, err
	}
	return mapTechnician(row), nil
}

func (r *fleetRepository) GetTechnicianAssignmentForOrg(ctx context.Context, organizationID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	q := db.New(r.pool)
	row, err := q.AdminGetTechnicianAssignmentForOrg(ctx, db.AdminGetTechnicianAssignmentForOrgParams{
		ID:             assignmentID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.TechnicianMachineAssignment{}, appfleet.ErrNotFound
		}
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return mapTechnicianMachineAssignment(row), nil
}

func (r *fleetRepository) UpdateTechnicianAssignment(ctx context.Context, p appfleet.UpdateAssignmentParams) (domainfleet.TechnicianMachineAssignment, error) {
	q := db.New(r.pool)
	var vto pgtype.Timestamptz
	if p.ValidTo != nil {
		vto = pgtype.Timestamptz{Time: p.ValidTo.UTC(), Valid: true}
	}
	row, err := q.AdminUpdateTechnicianAssignment(ctx, db.AdminUpdateTechnicianAssignmentParams{
		ID:             p.AssignmentID,
		OrganizationID: p.OrganizationID,
		Role:           strings.TrimSpace(p.Role),
		ValidTo:        vto,
		Status:         strings.TrimSpace(p.Status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.TechnicianMachineAssignment{}, appfleet.ErrNotFound
		}
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return mapTechnicianMachineAssignment(row), nil
}

func (r *fleetRepository) ReleaseTechnicianAssignment(ctx context.Context, organizationID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	q := db.New(r.pool)
	row, err := q.AdminReleaseTechnicianAssignment(ctx, db.AdminReleaseTechnicianAssignmentParams{
		ID:             assignmentID,
		OrganizationID: organizationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.TechnicianMachineAssignment{}, appfleet.ErrNotFound
		}
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return mapTechnicianMachineAssignment(row), nil
}

func (r *fleetRepository) BumpMachineCredentialVersion(ctx context.Context, organizationID, machineID uuid.UUID) (int64, error) {
	q := db.New(r.pool)
	return q.BumpMachineCredentialVersion(ctx, db.BumpMachineCredentialVersionParams{
		ID:             machineID,
		OrganizationID: organizationID,
	})
}

func (r *fleetRepository) RevokeActiveMachineActivationCodes(ctx context.Context, organizationID, machineID uuid.UUID) error {
	q := db.New(r.pool)
	return q.AdminRevokeActiveMachineActivationCodes(ctx, db.AdminRevokeActiveMachineActivationCodesParams{
		MachineID:      machineID,
		OrganizationID: organizationID,
	})
}
