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
    AND TRUE;

-- name: RevokeMachineCredentials :one
UPDATE machines
SET
    credential_version = credential_version + 1,
    credential_revoked_at = now(),
    revoked_at = now(),
    updated_at = now()
WHERE
    id = $1
    AND TRUE
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
    AND TRUE
RETURNING credential_version;

-- name: GetTechnicianByID :one
SELECT
    id,
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

-- name: ListMachinesByScopeID :many
SELECT *
FROM machines
ORDER BY
    name ASC;

-- name: ListMachinesBySiteAndCompany :many
SELECT *
FROM machines
WHERE
    site_id = $1
    AND TRUE
ORDER BY
    name ASC;

-- name: ListMachinesForTechnicianExternalSubject :many
SELECT
    m.id,
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
    AND TRUE
    AND (
        tma.valid_to IS NULL
        OR tma.valid_to > now()
    )
ORDER BY
    m.name ASC;

-- name: ListMachinesForTechnicianID :many
SELECT
    m.id,
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
    $9
)
RETURNING *;

-- name: UpdateMachineMetadataRow :one
UPDATE machines
SET
    name = $1,
    status = $2,
    hardware_profile_id = $3,
    site_id = $4,
    serial_number = $5,
    code = $6,
    model = $7,
    cabinet_type = $8,
    timezone_override = $9,
    activated_at = CASE WHEN $2 = 'active' AND activated_at IS NULL THEN now() ELSE activated_at END,
    updated_at = now()
WHERE
    id = $10
    AND TRUE
RETURNING *;

-- name: InsertTechnicianMachineAssignment :one
INSERT INTO technician_machine_assignments (
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
    'active'
)
RETURNING *;

-- name: AdminInsertSite :one
INSERT INTO sites (
    region_id,
    name,
    address,
    timezone,
    code,
    status
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    'active'
)
RETURNING *;

-- name: AdminGetSiteForOrg :one
SELECT *
FROM sites
WHERE
    id = $1
    AND TRUE;

-- name: AdminListSitesForOrg :many
SELECT *
FROM sites
WHERE
    ($1::boolean IS FALSE OR status = $2::text)
ORDER BY
    name ASC
LIMIT $3 OFFSET $4;

-- name: AdminCountSitesForOrg :one
SELECT count(*)::bigint AS cnt
FROM sites
WHERE
    ($1::boolean IS FALSE OR status = $2::text);

-- name: AdminUpdateSiteRow :one
UPDATE sites
SET
    region_id = $1,
    name = $2,
    address = $3,
    timezone = $4,
    code = $5,
    status = $6,
    updated_at = now()
WHERE
    id = $7
    AND TRUE
RETURNING *;

-- name: AdminCountNonRetiredMachinesForSite :one
SELECT count(*)::bigint AS cnt
FROM machines
WHERE
    site_id = $1
    AND status NOT IN ('retired', 'decommissioned');

-- name: AdminInsertTechnician :one
INSERT INTO technicians (
    display_name,
    email,
    phone,
    external_subject,
    status
) VALUES (
    $1,
    NULLIF(btrim($2::text), ''),
    NULLIF(btrim($3::text), ''),
    NULLIF(btrim($4::text), ''),
    'active'
)
RETURNING *;

-- name: AdminGetTechnicianForOrg :one
SELECT *
FROM technicians
WHERE
    id = $1
    AND TRUE;

-- name: AdminListTechniciansForOrg :many
SELECT *
FROM technicians
WHERE
    ($1::boolean IS FALSE OR id = $2::uuid)
    AND ($3::boolean IS FALSE OR status = $4::text)
    AND (
        $5::boolean IS FALSE
        OR display_name ILIKE ('%' || $6::text || '%')
        OR (
            email IS NOT NULL
            AND email::text ILIKE ('%' || $6::text || '%')
        )
    )
ORDER BY
    display_name ASC
LIMIT $7 OFFSET $8;

-- name: AdminCountTechniciansForOrg :one
SELECT count(*)::bigint AS cnt
FROM technicians
WHERE
    ($1::boolean IS FALSE OR id = $2::uuid)
    AND ($3::boolean IS FALSE OR status = $4::text)
    AND (
        $5::boolean IS FALSE
        OR display_name ILIKE ('%' || $6::text || '%')
        OR (
            email IS NOT NULL
            AND email::text ILIKE ('%' || $6::text || '%')
        )
    );

-- name: AdminUpdateTechnicianRow :one
UPDATE technicians
SET
    display_name = $1,
    email = NULLIF(btrim($2::text), ''),
    phone = NULLIF(btrim($3::text), ''),
    external_subject = NULLIF(btrim($4::text), ''),
    updated_at = now()
WHERE
    id = $5
    AND TRUE
RETURNING *;

-- name: AdminSetTechnicianStatus :one
UPDATE technicians
SET
    status = $1,
    updated_at = now()
WHERE
    id = $2
    AND TRUE
RETURNING *;

-- name: AdminGetTechnicianAssignmentForOrg :one
SELECT *
FROM technician_machine_assignments
WHERE
    id = $1
    AND TRUE;

-- name: AdminUpdateTechnicianAssignment :one
UPDATE technician_machine_assignments
SET
    role = $1,
    valid_to = $2,
    status = $3,
    updated_at = now()
WHERE
    id = $4
    AND TRUE
RETURNING *;

-- name: AdminReleaseTechnicianAssignment :one
UPDATE technician_machine_assignments
SET
    status = 'released',
    valid_to = COALESCE(valid_to, now()),
    updated_at = now()
WHERE
    id = $1
    AND TRUE
RETURNING *;

-- name: AdminReleaseTechnicianAssignmentForMachineUser :one
UPDATE technician_machine_assignments
SET
    status = 'released',
    valid_to = COALESCE(valid_to, now()),
    updated_at = now()
WHERE
    machine_id = $1
    AND technician_id = $2
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
    AND TRUE
    AND status = 'active';
