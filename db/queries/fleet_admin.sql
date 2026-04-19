-- name: FleetAdminListMachines :many
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
WHERE
    m.organization_id = $1
    AND ($2::boolean IS FALSE OR m.site_id = $3::uuid)
    AND ($4::boolean IS FALSE OR m.id = $5::uuid)
    AND ($6::boolean IS FALSE OR m.status = $7::text)
    AND m.updated_at >= $8::timestamptz
    AND m.updated_at <= $9::timestamptz
ORDER BY
    m.updated_at DESC,
    m.name ASC
LIMIT $10 OFFSET $11;

-- name: FleetAdminCountMachines :one
SELECT
    count(*)::bigint AS cnt
FROM machines m
WHERE
    m.organization_id = $1
    AND ($2::boolean IS FALSE OR m.site_id = $3::uuid)
    AND ($4::boolean IS FALSE OR m.id = $5::uuid)
    AND ($6::boolean IS FALSE OR m.status = $7::text)
    AND m.updated_at >= $8::timestamptz
    AND m.updated_at <= $9::timestamptz;

-- name: FleetAdminListTechnicians :many
SELECT
    t.id,
    t.organization_id,
    t.display_name,
    t.email,
    t.phone,
    t.external_subject,
    t.created_at
FROM technicians t
WHERE
    t.organization_id = $1
    AND ($2::boolean IS FALSE OR t.id = $3::uuid)
    AND (
        $4::boolean IS FALSE
        OR t.display_name ILIKE ('%' || $5::text || '%')
        OR (
            t.email IS NOT NULL
            AND t.email::text ILIKE ('%' || $5::text || '%')
        )
    )
    AND t.created_at >= $6::timestamptz
    AND t.created_at <= $7::timestamptz
ORDER BY
    t.display_name ASC
LIMIT $8 OFFSET $9;

-- name: FleetAdminCountTechnicians :one
SELECT
    count(*)::bigint AS cnt
FROM technicians t
WHERE
    t.organization_id = $1
    AND ($2::boolean IS FALSE OR t.id = $3::uuid)
    AND (
        $4::boolean IS FALSE
        OR t.display_name ILIKE ('%' || $5::text || '%')
        OR (
            t.email IS NOT NULL
            AND t.email::text ILIKE ('%' || $5::text || '%')
        )
    )
    AND t.created_at >= $6::timestamptz
    AND t.created_at <= $7::timestamptz;

-- name: FleetAdminListAssignments :many
SELECT
    tma.id AS assignment_id,
    tma.technician_id,
    t.display_name AS technician_display_name,
    tma.machine_id,
    m.name AS machine_name,
    m.serial_number AS machine_serial_number,
    tma.role,
    tma.valid_from,
    tma.valid_to,
    tma.created_at
FROM technician_machine_assignments tma
INNER JOIN technicians t ON t.id = tma.technician_id
INNER JOIN machines m ON m.id = tma.machine_id
WHERE
    t.organization_id = $1
    AND ($2::boolean IS FALSE OR tma.technician_id = $3::uuid)
    AND ($4::boolean IS FALSE OR tma.machine_id = $5::uuid)
    AND tma.created_at >= $6::timestamptz
    AND tma.created_at <= $7::timestamptz
ORDER BY
    tma.valid_from DESC
LIMIT $8 OFFSET $9;

-- name: FleetAdminCountAssignments :one
SELECT
    count(*)::bigint AS cnt
FROM technician_machine_assignments tma
INNER JOIN technicians t ON t.id = tma.technician_id
WHERE
    t.organization_id = $1
    AND ($2::boolean IS FALSE OR tma.technician_id = $3::uuid)
    AND ($4::boolean IS FALSE OR tma.machine_id = $5::uuid)
    AND tma.created_at >= $6::timestamptz
    AND tma.created_at <= $7::timestamptz;

-- name: FleetAdminListCommands :many
SELECT
    cl.id AS command_id,
    cl.machine_id,
    m.organization_id,
    m.name AS machine_name,
    m.serial_number AS machine_serial_number,
    cl.sequence,
    cl.command_type,
    cl.created_at,
    cl.attempt_count,
    cl.correlation_id,
    la.status AS latest_attempt_status
FROM command_ledger cl
INNER JOIN machines m ON m.id = cl.machine_id
LEFT JOIN LATERAL (
    SELECT
        mca.status
    FROM machine_command_attempts mca
    WHERE
        mca.command_id = cl.id
    ORDER BY
        mca.attempt_no DESC
    LIMIT
        1
) la ON TRUE
WHERE
    m.organization_id = $1
    AND ($2::boolean IS FALSE OR cl.machine_id = $3::uuid)
    AND ($4::boolean IS FALSE OR COALESCE(la.status, 'pending') = $5::text)
    AND cl.created_at >= $6::timestamptz
    AND cl.created_at <= $7::timestamptz
ORDER BY
    cl.created_at DESC
LIMIT $8 OFFSET $9;

-- name: FleetAdminCountCommands :one
SELECT
    count(*)::bigint AS cnt
FROM command_ledger cl
INNER JOIN machines m ON m.id = cl.machine_id
LEFT JOIN LATERAL (
    SELECT
        mca.status
    FROM machine_command_attempts mca
    WHERE
        mca.command_id = cl.id
    ORDER BY
        mca.attempt_no DESC
    LIMIT
        1
) la ON TRUE
WHERE
    m.organization_id = $1
    AND ($2::boolean IS FALSE OR cl.machine_id = $3::uuid)
    AND ($4::boolean IS FALSE OR COALESCE(la.status, 'pending') = $5::text)
    AND cl.created_at >= $6::timestamptz
    AND cl.created_at <= $7::timestamptz;

-- name: FleetAdminListOTACampaigns :many
SELECT
    c.id AS campaign_id,
    c.organization_id,
    c.name AS campaign_name,
    c.strategy,
    c.status AS campaign_status,
    c.created_at,
    a.id AS artifact_id,
    a.semver AS artifact_semver,
    a.storage_key AS artifact_storage_key
FROM ota_campaigns c
INNER JOIN ota_artifacts a ON a.id = c.artifact_id
WHERE
    c.organization_id = $1
    AND ($2::boolean IS FALSE OR c.status = $3::text)
    AND c.created_at >= $4::timestamptz
    AND c.created_at <= $5::timestamptz
ORDER BY
    c.created_at DESC
LIMIT $6 OFFSET $7;

-- name: FleetAdminCountOTACampaigns :one
SELECT
    count(*)::bigint AS cnt
FROM ota_campaigns c
WHERE
    c.organization_id = $1
    AND ($2::boolean IS FALSE OR c.status = $3::text)
    AND c.created_at >= $4::timestamptz
    AND c.created_at <= $5::timestamptz;
