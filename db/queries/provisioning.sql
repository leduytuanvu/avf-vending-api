-- name: InsertProvisioningBatch :one
INSERT INTO machine_provisioning_batches (
    organization_id,
    site_id,
    hardware_profile_id,
    cabinet_type,
    status,
    machine_count,
    metadata,
    created_by
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING *;

-- name: InsertProvisioningBatchMachine :one
INSERT INTO machine_provisioning_batch_machines (
    batch_id,
    organization_id,
    machine_id,
    serial_number,
    activation_code_id,
    row_no
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: UpdateProvisioningBatchStatus :one
UPDATE machine_provisioning_batches
SET
    status = $3,
    machine_count = $4,
    updated_at = now ()
WHERE
    id = $1
    AND organization_id = $2
RETURNING *;

-- name: GetProvisioningBatchByID :one
SELECT *
FROM machine_provisioning_batches
WHERE
    id = $1
    AND organization_id = $2;

-- name: ListProvisioningBatchMachines :many
SELECT
    b.id,
    b.batch_id,
    b.organization_id,
    b.machine_id,
    b.serial_number,
    b.activation_code_id,
    b.row_no,
    b.created_at,
    ac.status AS activation_code_status,
    ac.expires_at AS activation_expires_at,
    ac.uses AS activation_uses,
    ac.max_uses AS activation_max_uses
FROM machine_provisioning_batch_machines b
LEFT JOIN machine_activation_codes ac ON ac.id = b.activation_code_id
WHERE
    b.batch_id = $1
    AND b.organization_id = $2
ORDER BY
    b.row_no ASC,
    b.created_at ASC;
