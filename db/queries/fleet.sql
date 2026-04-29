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

-- name: GetMachineCredentialGate :one
SELECT
    credential_version,
    status,
    organization_id,
    credential_revoked_at
FROM
    machines
WHERE
    id = $1;

-- name: MarkMachineCredentialUsed :exec
UPDATE machines
SET
    credential_last_used_at = now()
WHERE
    id = $1
    AND organization_id = $2;

-- name: RevokeMachineCredentials :one
UPDATE machines
SET
    credential_version = credential_version + 1,
    credential_revoked_at = now(),
    revoked_at = now(),
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING credential_version;

-- name: BumpMachineCredentialVersion :one
UPDATE machines
SET
    credential_version = credential_version + 1,
    credential_rotated_at = now(),
    rotated_at = now(),
    credential_revoked_at = NULL,
    revoked_at = NULL,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING credential_version;

-- name: GetTechnicianByID :one
SELECT
    id,
    organization_id,
    display_name,
    email,
    phone,
    external_subject,
    status,
    created_at,
    updated_at
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
SELECT *
FROM machines
WHERE
    organization_id = $1
ORDER BY
    name ASC;

-- name: ListMachinesBySiteAndOrganization :many
SELECT *
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
    m.code,
    m.model,
    m.credential_version,
    m.last_seen_at,
    m.timezone_override,
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
    m.code,
    m.model,
    m.credential_version,
    m.last_seen_at,
    m.timezone_override,
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
    code,
    model,
    cabinet_type,
    timezone_override,
    name,
    status
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10
)
RETURNING *;

-- name: UpdateMachineMetadataRow :one
UPDATE machines
SET
    name = $3,
    status = $4,
    hardware_profile_id = $5,
    site_id = $6,
    serial_number = $7,
    code = $8,
    model = $9,
    cabinet_type = $10,
    timezone_override = $11,
    activated_at = CASE WHEN $4 = 'active' AND activated_at IS NULL THEN now() ELSE activated_at END,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: InsertTechnicianMachineAssignment :one
INSERT INTO technician_machine_assignments (
    organization_id,
    technician_id,
    machine_id,
    role,
    scope,
    created_by,
    status
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    'active'
)
RETURNING *;

-- name: AdminInsertSite :one
INSERT INTO sites (organization_id, region_id, name, address, timezone, code, status)
VALUES ($1, $2, $3, $4, $5, $6, 'active')
RETURNING *;

-- name: AdminGetSiteForOrg :one
SELECT *
FROM sites
WHERE
    id = $1
    AND organization_id = $2;

-- name: AdminListSitesForOrg :many
SELECT *
FROM sites
WHERE
    organization_id = $1
    AND ($2::boolean IS FALSE OR status = $3::text)
ORDER BY
    name ASC
LIMIT $4 OFFSET $5;

-- name: AdminCountSitesForOrg :one
SELECT count(*)::bigint AS cnt
FROM sites
WHERE
    organization_id = $1
    AND ($2::boolean IS FALSE OR status = $3::text);

-- name: AdminUpdateSiteRow :one
UPDATE sites
SET
    region_id = $3,
    name = $4,
    address = $5,
    timezone = $6,
    code = $7,
    status = $8,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: AdminCountNonRetiredMachinesForSite :one
SELECT count(*)::bigint AS cnt
FROM machines
WHERE
    organization_id = $1
    AND site_id = $2
    AND status NOT IN ('retired', 'decommissioned');

-- name: AdminInsertTechnician :one
INSERT INTO technicians (
    organization_id,
    display_name,
    email,
    phone,
    external_subject,
    status
) VALUES (
    $1,
    $2,
    NULLIF(btrim($3::text), ''),
    NULLIF(btrim($4::text), ''),
    NULLIF(btrim($5::text), ''),
    'active'
)
RETURNING *;

-- name: AdminGetTechnicianForOrg :one
SELECT *
FROM technicians
WHERE
    id = $1
    AND organization_id = $2;

-- name: AdminListTechniciansForOrg :many
SELECT *
FROM technicians
WHERE
    organization_id = $1
    AND ($2::boolean IS FALSE OR id = $3::uuid)
    AND ($4::boolean IS FALSE OR status = $5::text)
    AND (
        $6::boolean IS FALSE
        OR display_name ILIKE ('%' || $7::text || '%')
        OR (
            email IS NOT NULL
            AND email::text ILIKE ('%' || $7::text || '%')
        )
    )
ORDER BY
    display_name ASC
LIMIT $8 OFFSET $9;

-- name: AdminCountTechniciansForOrg :one
SELECT count(*)::bigint AS cnt
FROM technicians
WHERE
    organization_id = $1
    AND ($2::boolean IS FALSE OR id = $3::uuid)
    AND ($4::boolean IS FALSE OR status = $5::text)
    AND (
        $6::boolean IS FALSE
        OR display_name ILIKE ('%' || $7::text || '%')
        OR (
            email IS NOT NULL
            AND email::text ILIKE ('%' || $7::text || '%')
        )
    );

-- name: AdminUpdateTechnicianRow :one
UPDATE technicians
SET
    display_name = $3,
    email = NULLIF(btrim($4::text), ''),
    phone = NULLIF(btrim($5::text), ''),
    external_subject = NULLIF(btrim($6::text), ''),
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: AdminSetTechnicianStatus :one
UPDATE technicians
SET
    status = $3,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: AdminGetTechnicianAssignmentForOrg :one
SELECT *
FROM technician_machine_assignments
WHERE
    id = $1
    AND organization_id = $2;

-- name: AdminUpdateTechnicianAssignment :one
UPDATE technician_machine_assignments
SET
    role = $3,
    valid_to = $4,
    status = $5,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: AdminReleaseTechnicianAssignment :one
UPDATE technician_machine_assignments
SET
    status = 'released',
    valid_to = COALESCE(valid_to, now()),
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: AdminReleaseTechnicianAssignmentForMachineUser :one
UPDATE technician_machine_assignments
SET
    status = 'released',
    valid_to = COALESCE(valid_to, now()),
    updated_at = now()
WHERE
    organization_id = $1
    AND machine_id = $2
    AND technician_id = $3
    AND status = 'active'
    AND valid_to IS NULL
RETURNING *;

-- name: AdminRevokeActiveMachineActivationCodes :exec
UPDATE machine_activation_codes
SET
    status = 'revoked',
    updated_at = now()
WHERE
    machine_id = $1
    AND organization_id = $2
    AND status = 'active';
