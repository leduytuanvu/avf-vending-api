package db

import (
	"context"

	"github.com/google/uuid"
)

const getOrganizationByID = `-- name: GetOrganizationByID :one
SELECT id, name, slug, status, created_at, updated_at
FROM organizations
WHERE id = $1
`

func (q *Queries) GetOrganizationByID(ctx context.Context, id uuid.UUID) (Organization, error) {
	row := q.db.QueryRow(ctx, getOrganizationByID, id)
	var o Organization
	err := row.Scan(
		&o.ID,
		&o.Name,
		&o.Slug,
		&o.Status,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	return o, err
}

const getSiteByID = `-- name: GetSiteByID :one
SELECT id, organization_id, region_id, name, address, created_at
FROM sites
WHERE id = $1
`

func (q *Queries) GetSiteByID(ctx context.Context, id uuid.UUID) (Site, error) {
	row := q.db.QueryRow(ctx, getSiteByID, id)
	var s Site
	err := row.Scan(
		&s.ID,
		&s.OrganizationID,
		&s.RegionID,
		&s.Name,
		&s.Address,
		&s.CreatedAt,
	)
	return s, err
}

const getMachineByID = `-- name: GetMachineByID :one
SELECT id, organization_id, site_id, hardware_profile_id, serial_number, name, status, command_sequence, created_at, updated_at
FROM machines
WHERE id = $1
`

func (q *Queries) GetMachineByID(ctx context.Context, id uuid.UUID) (Machine, error) {
	return q.scanMachine(q.db.QueryRow(ctx, getMachineByID, id))
}

const getMachineByIDForUpdate = `-- name: GetMachineByIDForUpdate :one
SELECT id, organization_id, site_id, hardware_profile_id, serial_number, name, status, command_sequence, created_at, updated_at
FROM machines
WHERE id = $1
FOR UPDATE
`

func (q *Queries) GetMachineByIDForUpdate(ctx context.Context, id uuid.UUID) (Machine, error) {
	return q.scanMachine(q.db.QueryRow(ctx, getMachineByIDForUpdate, id))
}

const getTechnicianByID = `-- name: GetTechnicianByID :one
SELECT
    id,
    organization_id,
    display_name,
    email,
    phone,
    external_subject,
    created_at
FROM technicians
WHERE
    id = $1
`

func (q *Queries) GetTechnicianByID(ctx context.Context, id uuid.UUID) (Technician, error) {
	row := q.db.QueryRow(ctx, getTechnicianByID, id)
	var t Technician
	err := row.Scan(
		&t.ID,
		&t.OrganizationID,
		&t.DisplayName,
		&t.Email,
		&t.Phone,
		&t.ExternalSubject,
		&t.CreatedAt,
	)
	return t, err
}

const technicianActiveAssignmentExists = `-- name: TechnicianActiveAssignmentExists :one
SELECT EXISTS (
    SELECT
        1
    FROM technician_machine_assignments tma
    WHERE
        tma.technician_id = $1
        AND tma.machine_id = $2
        AND (
            tma.valid_to IS NULL
            OR tma.valid_to > now()
        )
)
`

func (q *Queries) TechnicianActiveAssignmentExists(ctx context.Context, technicianID uuid.UUID, machineID uuid.UUID) (bool, error) {
	row := q.db.QueryRow(ctx, technicianActiveAssignmentExists, technicianID, machineID)
	var ok bool
	err := row.Scan(&ok)
	return ok, err
}

type machineScanner interface {
	Scan(dest ...any) error
}

func (q *Queries) scanMachine(row machineScanner) (Machine, error) {
	var m Machine
	err := row.Scan(
		&m.ID,
		&m.OrganizationID,
		&m.SiteID,
		&m.HardwareProfileID,
		&m.SerialNumber,
		&m.Name,
		&m.Status,
		&m.CommandSequence,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	return m, err
}

const bumpMachineCommandSequence = `-- name: BumpMachineCommandSequence :one
UPDATE machines
SET
    command_sequence = command_sequence + 1,
    updated_at = now()
WHERE
    id = $1
RETURNING command_sequence
`

func (q *Queries) BumpMachineCommandSequence(ctx context.Context, machineID uuid.UUID) (int64, error) {
	row := q.db.QueryRow(ctx, bumpMachineCommandSequence, machineID)
	var seq int64
	err := row.Scan(&seq)
	return seq, err
}

const listMachinesByOrganizationID = `-- name: ListMachinesByOrganizationID :many
SELECT
    id,
    organization_id,
    site_id,
    hardware_profile_id,
    serial_number,
    name,
    status,
    command_sequence,
    created_at,
    updated_at
FROM machines
WHERE
    organization_id = $1
ORDER BY
    name ASC
`

func (q *Queries) ListMachinesByOrganizationID(ctx context.Context, organizationID uuid.UUID) ([]Machine, error) {
	rows, err := q.db.Query(ctx, listMachinesByOrganizationID, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Machine
	for rows.Next() {
		m, err := q.scanMachine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

const listMachinesBySiteAndOrganization = `-- name: ListMachinesBySiteAndOrganization :many
SELECT
    id,
    organization_id,
    site_id,
    hardware_profile_id,
    serial_number,
    name,
    status,
    command_sequence,
    created_at,
    updated_at
FROM machines
WHERE
    site_id = $1
    AND organization_id = $2
ORDER BY
    name ASC
`

type ListMachinesBySiteAndOrganizationParams struct {
	SiteID         uuid.UUID `json:"site_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
}

func (q *Queries) ListMachinesBySiteAndOrganization(ctx context.Context, arg ListMachinesBySiteAndOrganizationParams) ([]Machine, error) {
	rows, err := q.db.Query(ctx, listMachinesBySiteAndOrganization, arg.SiteID, arg.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Machine
	for rows.Next() {
		m, err := q.scanMachine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

const listMachinesForTechnicianExternalSubject = `-- name: ListMachinesForTechnicianExternalSubject :many
SELECT
    m.id,
    m.organization_id,
    m.site_id,
    m.hardware_profile_id,
    m.serial_number,
    m.name,
    m.status,
    m.command_sequence,
    m.created_at,
    m.updated_at
FROM machines m
INNER JOIN technician_machine_assignments tma ON tma.machine_id = m.id
INNER JOIN technicians t ON t.id = tma.technician_id
WHERE
    t.external_subject = $1
    AND t.organization_id = $2
    AND (
        tma.valid_to IS NULL
        OR tma.valid_to > now()
    )
ORDER BY
    m.name ASC
`

type ListMachinesForTechnicianExternalSubjectParams struct {
	ExternalSubject string    `json:"external_subject"`
	OrganizationID  uuid.UUID `json:"organization_id"`
}

func (q *Queries) ListMachinesForTechnicianExternalSubject(ctx context.Context, arg ListMachinesForTechnicianExternalSubjectParams) ([]Machine, error) {
	rows, err := q.db.Query(ctx, listMachinesForTechnicianExternalSubject, arg.ExternalSubject, arg.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Machine
	for rows.Next() {
		m, err := q.scanMachine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

const listMachinesForTechnicianID = `-- name: ListMachinesForTechnicianID :many
SELECT
    m.id,
    m.organization_id,
    m.site_id,
    m.hardware_profile_id,
    m.serial_number,
    m.name,
    m.status,
    m.command_sequence,
    m.created_at,
    m.updated_at
FROM machines m
INNER JOIN technician_machine_assignments tma ON tma.machine_id = m.id
WHERE
    tma.technician_id = $1
    AND (
        tma.valid_to IS NULL
        OR tma.valid_to > now()
    )
ORDER BY
    m.name ASC
`

func (q *Queries) ListMachinesForTechnicianID(ctx context.Context, technicianID uuid.UUID) ([]Machine, error) {
	rows, err := q.db.Query(ctx, listMachinesForTechnicianID, technicianID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Machine
	for rows.Next() {
		m, err := q.scanMachine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type InsertMachineParams struct {
	OrganizationID    uuid.UUID
	SiteID            uuid.UUID
	HardwareProfileID *uuid.UUID
	SerialNumber      string
	Name              string
	Status            string
}

const insertMachine = `-- name: InsertMachine :one
INSERT INTO machines (
    organization_id,
    site_id,
    hardware_profile_id,
    serial_number,
    name,
    status
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING
    id,
    organization_id,
    site_id,
    hardware_profile_id,
    serial_number,
    name,
    status,
    command_sequence,
    created_at,
    updated_at
`

func (q *Queries) InsertMachine(ctx context.Context, arg InsertMachineParams) (Machine, error) {
	row := q.db.QueryRow(ctx, insertMachine,
		arg.OrganizationID,
		arg.SiteID,
		arg.HardwareProfileID,
		arg.SerialNumber,
		arg.Name,
		arg.Status,
	)
	return q.scanMachine(row)
}

type UpdateMachineMetadataRowParams struct {
	MachineID         uuid.UUID
	OrganizationID    uuid.UUID
	Name              string
	Status            string
	HardwareProfileID *uuid.UUID
}

const updateMachineMetadataRow = `-- name: UpdateMachineMetadataRow :one
UPDATE machines
SET
    name = $3,
    status = $4,
    hardware_profile_id = $5,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING
    id,
    organization_id,
    site_id,
    hardware_profile_id,
    serial_number,
    name,
    status,
    command_sequence,
    created_at,
    updated_at
`

func (q *Queries) UpdateMachineMetadataRow(ctx context.Context, arg UpdateMachineMetadataRowParams) (Machine, error) {
	row := q.db.QueryRow(ctx, updateMachineMetadataRow,
		arg.MachineID,
		arg.OrganizationID,
		arg.Name,
		arg.Status,
		arg.HardwareProfileID,
	)
	return q.scanMachine(row)
}

type InsertTechnicianMachineAssignmentParams struct {
	TechnicianID uuid.UUID
	MachineID    uuid.UUID
	Role         string
}

const insertTechnicianMachineAssignment = `-- name: InsertTechnicianMachineAssignment :one
INSERT INTO technician_machine_assignments (
    technician_id,
    machine_id,
    role
) VALUES (
    $1,
    $2,
    $3
)
RETURNING
    id,
    technician_id,
    machine_id,
    role,
    valid_from,
    valid_to,
    created_at
`

func (q *Queries) InsertTechnicianMachineAssignment(ctx context.Context, arg InsertTechnicianMachineAssignmentParams) (TechnicianMachineAssignment, error) {
	row := q.db.QueryRow(ctx, insertTechnicianMachineAssignment, arg.TechnicianID, arg.MachineID, arg.Role)
	var i TechnicianMachineAssignment
	err := row.Scan(
		&i.ID,
		&i.TechnicianID,
		&i.MachineID,
		&i.Role,
		&i.ValidFrom,
		&i.ValidTo,
		&i.CreatedAt,
	)
	return i, err
}
