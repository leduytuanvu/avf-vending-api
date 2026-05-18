package postgres

import (
	"context"
	"errors"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FleetLifecycleRepository persists fleet lifecycle mutations from admin APIs (P0.3).
// Prefer migrating queries into sqlc (see db/queries/fleet_lifecycle.sql) long-term.
type FleetLifecycleRepository struct {
	pool *pgxpool.Pool
}

// NewFleetLifecycleRepository constructs the lifecycle repository.
func NewFleetLifecycleRepository(pool *pgxpool.Pool) *FleetLifecycleRepository {
	if pool == nil {
		panic("postgres.NewFleetLifecycleRepository: nil pool")
	}
	return &FleetLifecycleRepository{pool: pool}
}

func scanSite(row pgx.Row) (db.Site, error) {
	var s db.Site
	err := row.Scan(
		&s.ID,
		&s.RegionID,
		&s.Name,
		&s.Address,
		&s.Timezone,
		&s.Code,
		&s.ContactInfo,
		&s.Status,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	return s, err
}

func scanMachine(row pgx.Row) (db.Machine, error) {
	var m db.Machine
	err := row.Scan(
		&m.ID,
		&m.SiteID,
		&m.HardwareProfileID,
		&m.SerialNumber,
		&m.Code,
		&m.Model,
		&m.CabinetType,
		&m.CredentialVersion,
		&m.LastSeenAt,
		&m.TimezoneOverride,
		&m.Name,
		&m.Status,
		&m.CommandSequence,
		&m.CredentialRevokedAt,
		&m.CredentialRotatedAt,
		&m.CredentialLastUsedAt,
		&m.ActivatedAt,
		&m.RevokedAt,
		&m.RotatedAt,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	return m, err
}

func scanTechnician(row pgx.Row) (db.Technician, error) {
	var t db.Technician
	err := row.Scan(
		&t.ID,
		&t.DisplayName,
		&t.Email,
		&t.Phone,
		&t.ExternalSubject,
		&t.Status,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	return t, err
}

// InsertSite inserts a new active site; site codes must be unique per company when non-empty.
func (r *FleetLifecycleRepository) InsertSite(ctx context.Context, companyID uuid.UUID, regionID pgtype.UUID, name string, address []byte, timezone string, code string) (db.Site, error) {
	const sqlInsert = `
INSERT INTO sites (region_id, name, address, timezone, code, status)
VALUES ($1, $2, $3, $4, $5, 'active')
RETURNING *
`
	row := r.pool.QueryRow(ctx, sqlInsert, regionID, name, address, timezone, code)
	_ = companyID
	s, err := scanSite(row)
	if err != nil {
		return db.Site{}, err
	}
	return s, nil
}

// InsertMachine inserts a machine row in provisioning by default when status empty.
func (r *FleetLifecycleRepository) InsertMachine(ctx context.Context, companyID, siteID uuid.UUID, hardwareProfileID pgtype.UUID, serialNumber, code string, model pgtype.Text, name string, status string, tz pgtype.Text) (db.Machine, error) {
	if status == "" {
		status = "draft"
	}
	const sqlInsert = `
INSERT INTO machines (
    site_id,
    hardware_profile_id,
    serial_number,
    code,
    model,
    cabinet_type,
    name,
    status,
    timezone_override
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
RETURNING *
`
	row := r.pool.QueryRow(ctx, sqlInsert,
		siteID,
		hardwareProfileID,
		serialNumber,
		code,
		model,
		"",
		name,
		status,
		tz,
	)
	_ = companyID
	return scanMachine(row)
}

// BumpCredentialVersion increments machine credential_version for credential rotation flows.
func (r *FleetLifecycleRepository) BumpCredentialVersion(ctx context.Context, companyID, machineID uuid.UUID) (db.Machine, error) {
	const sqlBump = `
UPDATE machines
SET credential_version = credential_version + 1,
    credential_rotated_at = now(),
    credential_revoked_at = NULL,
    updated_at = now()
WHERE id = $1
RETURNING *
`
	row := r.pool.QueryRow(ctx, sqlBump, machineID)
	_ = companyID
	return scanMachine(row)
}

// RevokeActiveActivationCodesForMachine marks active kiosk activation codes revoked after rotation/deprovision hooks.
func (r *FleetLifecycleRepository) RevokeActiveActivationCodesForMachine(ctx context.Context, companyID, machineID uuid.UUID) error {
	const sqlRevoke = `
UPDATE machine_activation_codes
SET status = 'revoked', updated_at = now()
WHERE machine_id = $1 AND status = 'active'
`
	_, err := r.pool.Exec(ctx, sqlRevoke, machineID)
	_ = companyID
	return err
}

// InsertMachineLineage records successor linkage after replacement.
func (r *FleetLifecycleRepository) InsertMachineLineage(ctx context.Context, companyID, priorID, successorID uuid.UUID, reason pgtype.Text) error {
	const sqlIns = `
INSERT INTO machine_lineage (prior_machine_id, successor_machine_id, reason)
VALUES ($1,$2,$3)
`
	_, err := r.pool.Exec(ctx, sqlIns, priorID, successorID, reason)
	_ = companyID
	return err
}

// SerialNumberTakenExcluding tests global serial uniqueness when non-empty serial is provided.
func (r *FleetLifecycleRepository) SerialNumberTakenExcluding(ctx context.Context, serial string, excludeMachineID uuid.UUID) (bool, error) {
	if serial == "" {
		return false, nil
	}
	const q = `
SELECT EXISTS (
    SELECT 1 FROM machines
    WHERE lower(trim(serial_number)) = lower(trim($1::text))
      AND btrim(serial_number) <> ''
      AND id <> $2
)`
	row := r.pool.QueryRow(ctx, q, serial, excludeMachineID)
	var taken bool
	if err := row.Scan(&taken); err != nil {
		return false, err
	}
	return taken, nil
}

// CountNonRetiredMachinesForSite counts machines still attached to a site (excluding retired assets).
func (r *FleetLifecycleRepository) CountNonRetiredMachinesForSite(ctx context.Context, companyID, siteID uuid.UUID) (int64, error) {
	const q = `
SELECT count(*)::bigint FROM machines
WHERE site_id = $1 AND status NOT IN ('retired', 'decommissioned')
`
	row := r.pool.QueryRow(ctx, q, siteID)
	_ = companyID
	var n int64
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ErrSiteBlockedDeactivate indicates machines still reference the site.
var ErrSiteBlockedDeactivate = errors.New("fleet lifecycle: site still has non-retired machines")

// DeactivateSite sets site archived when no non-retired machines reference it.
func (r *FleetLifecycleRepository) DeactivateSite(ctx context.Context, companyID, siteID uuid.UUID) (db.Site, error) {
	n, err := r.CountNonRetiredMachinesForSite(ctx, companyID, siteID)
	if err != nil {
		return db.Site{}, err
	}
	if n > 0 {
		return db.Site{}, ErrSiteBlockedDeactivate
	}
	const sqlUp = `
UPDATE sites SET status = 'archived', updated_at = now()
WHERE id = $1
RETURNING *
`
	row := r.pool.QueryRow(ctx, sqlUp, siteID)
	_ = companyID
	s, err := scanSite(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Site{}, pgx.ErrNoRows
	}
	return s, err
}
