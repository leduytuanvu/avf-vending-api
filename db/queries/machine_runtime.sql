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
    $14
FROM
    machines m
WHERE
    m.id = $15
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
