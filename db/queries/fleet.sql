-- name: GetOrganizationByID :one
SELECT *
FROM organizations
WHERE id = $1;

-- name: GetSiteByID :one
SELECT *
FROM sites
WHERE id = $1;

-- name: GetMachineByID :one
SELECT *
FROM machines
WHERE id = $1;

-- name: GetMachineByIDForUpdate :one
SELECT *
FROM machines
WHERE id = $1
FOR UPDATE;

-- name: GetTechnicianByID :one
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
    id = $1;

-- name: TechnicianActiveAssignmentExists :one
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
) AS exists;

-- name: BumpMachineCommandSequence :one
UPDATE machines
SET
    command_sequence = command_sequence + 1,
    updated_at = now()
WHERE
    id = $1
RETURNING command_sequence;

-- name: ListMachinesByOrganizationID :many
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
    name ASC;

-- name: ListMachinesBySiteAndOrganization :many
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
    name ASC;

-- name: ListMachinesForTechnicianExternalSubject :many
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
    m.name ASC;

-- name: ListMachinesForTechnicianID :many
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
    m.name ASC;

-- name: InsertMachine :one
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
    updated_at;

-- name: UpdateMachineMetadataRow :one
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
    updated_at;

-- name: InsertTechnicianMachineAssignment :one
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
    created_at;
