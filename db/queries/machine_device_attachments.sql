-- name: InsertMachineDeviceAttachment :one
INSERT INTO machine_device_attachments (
    machine_id,
    previous_attachment_id,
    status,
    reason,
    attached_by_account_id,
    operator_session_id,
    correlation_id,
    android_id,
    android_serial,
    board_serial,
    device_serial,
    sim_serial,
    sim_iccid,
    sim_operator,
    sim_country_iso,
    manufacturer,
    brand,
    model,
    device_model,
    hardware,
    product,
    android_release,
    sdk_int,
    package_name,
    version_name,
    version_code,
    app_build_sha,
    boot_id,
    network_type,
    network_state,
    ip_address,
    user_agent,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32,
    COALESCE(NULLIF(sqlc.arg('metadata')::text, '')::jsonb, '{}'::jsonb)
)
RETURNING *;

-- name: GetActiveMachineDeviceAttachment :one
SELECT *
FROM machine_device_attachments
WHERE machine_id = $1 AND status = 'active'
ORDER BY attached_at DESC
LIMIT 1;

-- name: GetMachineDeviceAttachmentByID :one
SELECT *
FROM machine_device_attachments
WHERE id = $1;

-- name: ListMachineDeviceAttachments :many
SELECT *
FROM machine_device_attachments
WHERE machine_id = $1
ORDER BY attached_at DESC
LIMIT sqlc.arg (limit_val) OFFSET sqlc.arg (offset_val);

-- name: MarkMachineDeviceAttachmentReplaced :one
UPDATE machine_device_attachments
SET
    status = 'replaced',
    detached_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: MarkMachineDeviceAttachmentRevoked :one
UPDATE machine_device_attachments
SET
    status = 'revoked',
    detached_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: CountActiveMachineDeviceAttachments :one
SELECT count(*)::bigint AS count
FROM machine_device_attachments
WHERE machine_id = $1 AND status = 'active';

-- name: UpdateMachineCurrentDeviceAttachment :exec
UPDATE machines
SET
    current_device_attachment_id = $2,
    updated_at = now()
WHERE id = $1;
