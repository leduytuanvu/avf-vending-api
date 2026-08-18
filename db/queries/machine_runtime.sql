-- name: GetMachineShadowVersion :one
SELECT
    version
FROM
    machine_shadow
WHERE
    machine_id = $1;

-- name: InsertMachineCheckIn :one
INSERT INTO machine_check_ins (
    machine_id,
    android_id,
    sim_serial,
    package_name,
    version_name,
    version_code,
    android_release,
    sdk_int,
    manufacturer,
    model,
    timezone,
    network_state,
    boot_id,
    occurred_at,
    metadata
)
SELECT
    m.id,
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13,
    COALESCE(NULLIF(sqlc.arg('metadata')::text, '')::jsonb, '{}'::jsonb)
FROM
    machines m
WHERE
    m.id = sqlc.arg('id')
RETURNING
    id,
    machine_id,
    android_id,
    sim_serial,
    package_name,
    version_name,
    version_code,
    android_release,
    sdk_int,
    manufacturer,
    model,
    timezone,
    network_state,
    boot_id,
    occurred_at,
    recorded_at,
    metadata;

-- name: UpdateMachineCurrentSnapshotLastCheckIn :exec
UPDATE machine_current_snapshot
SET
    last_check_in_at = $1,
    updated_at = now()
WHERE
    machine_id = $2;

-- name: RuntimeGetMachineSellReadinessAcks :one
SELECT
    m.published_planogram_version_id,
    mcs.last_acknowledged_planogram_version_id,
    mcs.last_acknowledged_config_revision,
    COALESCE((
        SELECT mc.config_revision FROM machine_configs mc
        WHERE mc.machine_id = m.id
        ORDER BY mc.applied_at DESC
        LIMIT 1
    ), 0)::integer AS latest_config_revision
FROM machines m
LEFT JOIN machine_current_snapshot mcs ON mcs.machine_id = m.id
WHERE
    m.id = $1;
