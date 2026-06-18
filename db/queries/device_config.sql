-- name: SnapshotUpdateEffectiveDeviceConfig :exec
UPDATE machine_current_snapshot
SET
    effective_device_config = $1,
    updated_at = now()
WHERE
    machine_id = $2;

-- name: SnapshotUpdateDeviceConfigFieldAck :exec
UPDATE machine_current_snapshot
SET
    device_config_field_ack = $1,
    updated_at = now()
WHERE
    machine_id = $2;
