-- name: GetMachineOrganizationID :one
SELECT
    organization_id
FROM
    machines
WHERE
    id = $1;

-- name: InsertMachineCheckIn :one
INSERT INTO machine_check_ins (
    organization_id,
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
    m.organization_id,
    m.id,
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
    $14,
    $15
FROM
    machines m
WHERE
    m.id = $1
RETURNING
    id,
    organization_id,
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
    last_check_in_at = $2,
    updated_at = now()
WHERE
    machine_id = $1;
