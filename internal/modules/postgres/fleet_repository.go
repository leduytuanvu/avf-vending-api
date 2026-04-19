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
	"github.com/jackc/pgx/v5/pgxpool"
)

type fleetRepository struct {
	pool *pgxpool.Pool
}

// NewFleetRepository returns a Postgres-backed app/fleet.FleetRepository.
func NewFleetRepository(pool *pgxpool.Pool) appfleet.FleetRepository {
	if pool == nil {
		panic("postgres.NewFleetRepository: nil pool")
	}
	return &fleetRepository{pool: pool}
}

var _ appfleet.FleetRepository = (*fleetRepository)(nil)

func (r *fleetRepository) GetMachine(ctx context.Context, machineID uuid.UUID) (domainfleet.Machine, error) {
	row, err := db.New(r.pool).GetMachineByID(ctx, machineID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Machine{}, appfleet.ErrNotFound
		}
		return domainfleet.Machine{}, err
	}
	return mapMachine(row), nil
}

func (r *fleetRepository) GetTechnician(ctx context.Context, technicianID uuid.UUID) (domainfleet.Technician, error) {
	row, err := db.New(r.pool).GetTechnicianByID(ctx, technicianID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Technician{}, appfleet.ErrNotFound
		}
		return domainfleet.Technician{}, err
	}
	return mapTechnician(row), nil
}

func (r *fleetRepository) AssertSiteInOrganization(ctx context.Context, organizationID, siteID uuid.UUID) error {
	site, err := db.New(r.pool).GetSiteByID(ctx, siteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appfleet.ErrNotFound
		}
		return err
	}
	if site.OrganizationID != organizationID {
		return appfleet.ErrOrgMismatch
	}
	return nil
}

func (r *fleetRepository) InsertMachine(ctx context.Context, p appfleet.InsertMachineParams) (domainfleet.Machine, error) {
	row, err := db.New(r.pool).InsertMachine(ctx, db.InsertMachineParams{
		OrganizationID:    p.OrganizationID,
		SiteID:            p.SiteID,
		HardwareProfileID: optionalUUIDToPg(p.HardwareProfileID),
		SerialNumber:      p.SerialNumber,
		Name:              p.Name,
		Status:            p.Status,
	})
	if err != nil {
		return domainfleet.Machine{}, err
	}
	return mapMachine(row), nil
}

func (r *fleetRepository) UpdateMachineMetadata(ctx context.Context, p appfleet.UpdateMachineMetadataParams) (domainfleet.Machine, error) {
	cur, err := r.GetMachine(ctx, p.MachineID)
	if err != nil {
		return domainfleet.Machine{}, err
	}
	if cur.OrganizationID != p.OrganizationID {
		return domainfleet.Machine{}, appfleet.ErrOrgMismatch
	}

	name := cur.Name
	if p.Name != nil {
		name = strings.TrimSpace(*p.Name)
	}
	status := cur.Status
	if p.Status != nil {
		status = *p.Status
	}
	var hw pgtype.UUID
	if p.HardwareProfileID != nil {
		hw = optionalUUIDToPg(p.HardwareProfileID)
	} else {
		hw = optionalUUIDToPg(cur.HardwareProfileID)
	}

	row, err := db.New(r.pool).UpdateMachineMetadataRow(ctx, db.UpdateMachineMetadataRowParams{
		ID:                p.MachineID,
		OrganizationID:    p.OrganizationID,
		Name:              name,
		Status:            status,
		HardwareProfileID: hw,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainfleet.Machine{}, appfleet.ErrNotFound
		}
		return domainfleet.Machine{}, err
	}
	return mapMachine(row), nil
}

func (r *fleetRepository) ListMachinesInScope(ctx context.Context, filter appfleet.ListMachinesScope) ([]domainfleet.Machine, error) {
	q := db.New(r.pool)
	var rows []db.Machine
	var err error
	if filter.SiteID != nil {
		rows, err = q.ListMachinesBySiteAndOrganization(ctx, db.ListMachinesBySiteAndOrganizationParams{
			SiteID:         *filter.SiteID,
			OrganizationID: filter.OrganizationID,
		})
	} else {
		rows, err = q.ListMachinesByOrganizationID(ctx, filter.OrganizationID)
	}
	if err != nil {
		return nil, err
	}
	out := make([]domainfleet.Machine, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapMachine(row))
	}
	return out, nil
}

func (r *fleetRepository) InsertTechnicianMachineAssignment(ctx context.Context, p appfleet.InsertAssignmentParams) (domainfleet.TechnicianMachineAssignment, error) {
	row, err := db.New(r.pool).InsertTechnicianMachineAssignment(ctx, db.InsertTechnicianMachineAssignmentParams{
		TechnicianID: p.TechnicianID,
		MachineID:    p.MachineID,
		Role:         p.Role,
	})
	if err != nil {
		return domainfleet.TechnicianMachineAssignment{}, err
	}
	return mapTechnicianMachineAssignment(row), nil
}

func mapTechnicianMachineAssignment(row db.TechnicianMachineAssignment) domainfleet.TechnicianMachineAssignment {
	return domainfleet.TechnicianMachineAssignment{
		ID:           row.ID,
		TechnicianID: row.TechnicianID,
		MachineID:    row.MachineID,
		Role:         row.Role,
		ValidFrom:    row.ValidFrom,
		ValidTo:      pgTimestamptzToTimePtr(row.ValidTo),
		CreatedAt:    row.CreatedAt,
	}
}
