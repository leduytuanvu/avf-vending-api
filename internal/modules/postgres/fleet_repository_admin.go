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
		RegionID: optionalUUIDToPg(p.RegionID),
		Name:     strings.TrimSpace(p.Name),
		Address:  p.Address,
		Timezone: strings.TrimSpace(p.Timezone),
		Code:     strings.TrimSpace(p.Code),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domainfleet.Site{}, appfleet.ErrConflict
		}
		return domainfleet.Site{}, err
	}
	return mapFleetSite(row), nil
}

func (r *fleetRepository) GetSiteForOrg(ctx context.Context, companyID, siteID uuid.UUID) (domainfleet.Site, error) {
	q := db.New(r.pool)
	row, err := q.AdminGetSiteForOrg(ctx, siteID)
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
		Column1: filter,
		Column2: st,
		Limit:   p.Limit,
		Offset:  p.Offset,
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
		Column1: filter,
		Column2: st,
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r *fleetRepository) UpdateSite(ctx context.Context, p appfleet.UpdateSiteParams) (domainfleet.Site, error) {
	q := db.New(r.pool)
	row, err := q.AdminUpdateSiteRow(ctx, db.AdminUpdateSiteRowParams{RegionID: optionalUUIDToPg(p.RegionID),
		Name:     strings.TrimSpace(p.Name),
		Address:  p.Address,
		Timezone: strings.TrimSpace(p.Timezone),
		Code:     strings.TrimSpace(p.Code),
		Status:   strings.TrimSpace(p.Status),

		ID: p.SiteID,
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

func (r *fleetRepository) CountNonRetiredMachinesForSite(ctx context.Context, companyID, siteID uuid.UUID) (int64, error) {
	q := db.New(r.pool)
	return q.AdminCountNonRetiredMachinesForSite(ctx, siteID)
}

func (r *fleetRepository) InsertTechnicianRow(ctx context.Context, p appfleet.InsertTechnicianParams) (domainfleet.Technician, error) {
	q := db.New(r.pool)
	row, err := q.AdminInsertTechnician(ctx, db.AdminInsertTechnicianParams{Column2: p.Email,
		Column3: p.Phone,
		Column4: p.ExternalSubject,

		DisplayName: strings.TrimSpace(p.DisplayName),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domainfleet.Technician{}, appfleet.ErrConflict
		}
		return domainfleet.Technician{}, err
	}
	return mapTechnician(row), nil
}

func (r *fleetRepository) GetTechnicianForOrg(ctx context.Context, companyID, technicianID uuid.UUID) (domainfleet.Technician, error) {
	q := db.New(r.pool)
	row, err := q.AdminGetTechnicianForOrg(ctx, technicianID)
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
		Column1: idF,
		Column2: tid,
		Column3: stF,
		Column4: st,
		Column5: searchF,
		Column6: search,
		Limit:   p.Limit,
		Offset:  p.Offset,
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
		Column1: idF,
		Column2: tid,
		Column3: stF,
		Column4: st,
		Column5: searchF,
		Column6: search,
	})
}

func (r *fleetRepository) UpdateTechnicianRow(ctx context.Context, p appfleet.UpdateTechnicianRowParams) (domainfleet.Technician, error) {
	q := db.New(r.pool)
	row, err := q.AdminUpdateTechnicianRow(ctx, db.AdminUpdateTechnicianRowParams{Column2: p.Email,
		Column3:     p.Phone,
		Column4:     p.ExternalSubject,
		DisplayName: strings.TrimSpace(p.DisplayName),

		ID: p.TechnicianID,
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

func (r *fleetRepository) SetTechnicianStatus(ctx context.Context, companyID, technicianID uuid.UUID, status string) (domainfleet.Technician, error) {
	q := db.New(r.pool)
	row, err := q.AdminSetTechnicianStatus(ctx, db.AdminSetTechnicianStatusParams{Status: strings.TrimSpace(status),

		ID: technicianID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Technician{}, appfleet.ErrNotFound
		}
		return domainfleet.Technician{}, err
	}
	return mapTechnician(row), nil
}

func (r *fleetRepository) GetTechnicianAssignmentForOrg(ctx context.Context, companyID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	q := db.New(r.pool)
	row, err := q.AdminGetTechnicianAssignmentForOrg(ctx, assignmentID)
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
	row, err := q.AdminUpdateTechnicianAssignment(ctx, db.AdminUpdateTechnicianAssignmentParams{Role: strings.TrimSpace(p.Role),
		ValidTo: vto,
		Status:  strings.TrimSpace(p.Status),

		ID: p.AssignmentID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.TechnicianMachineAssignment{}, appfleet.ErrNotFound
		}
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return mapTechnicianMachineAssignment(row), nil
}

func (r *fleetRepository) ReleaseTechnicianAssignment(ctx context.Context, companyID, assignmentID uuid.UUID) (domainfleet.TechnicianMachineAssignment, error) {
	q := db.New(r.pool)
	row, err := q.AdminReleaseTechnicianAssignment(ctx, assignmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.TechnicianMachineAssignment{}, appfleet.ErrNotFound
		}
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return mapTechnicianMachineAssignment(row), nil
}

func (r *fleetRepository) BumpMachineCredentialVersion(ctx context.Context, companyID, machineID uuid.UUID) (int64, error) {
	q := db.New(r.pool)
	return q.BumpMachineCredentialVersion(ctx, machineID)
}

func (r *fleetRepository) RevokeActiveMachineActivationCodes(ctx context.Context, companyID, machineID uuid.UUID) error {
	q := db.New(r.pool)
	return q.AdminRevokeActiveMachineActivationCodes(ctx, machineID)
}
