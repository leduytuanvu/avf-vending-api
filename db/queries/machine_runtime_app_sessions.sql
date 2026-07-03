-- name: StartMachineRuntimeAppSession :one
INSERT INTO machine_runtime_app_sessions (
    machine_id,
    device_attachment_id,
    machine_session_id,
    operator_session_id,
    previous_runtime_session_id,
    boot_id,
    app_start_id,
    app_instance_id,
    package_name,
    app_version,
    app_build_sha,
    start_reason,
    status,
    last_network_state,
    last_mqtt_state,
    storefront_state,
    sell_ready,
    blockers,
    hardware_status,
    catalog_status,
    outbox_status,
    recovery_status,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
)
RETURNING *;

-- name: GetCurrentMachineRuntimeAppSession :one
SELECT *
FROM machine_runtime_app_sessions
WHERE machine_id = $1
    AND ended_at IS NULL
    AND status IN ('STARTING', 'ONLINE', 'STALE', 'OFFLINE', 'BLOCKED', 'MAINTENANCE')
ORDER BY started_at DESC
LIMIT 1;

-- name: GetMachineRuntimeAppSessionByID :one
SELECT *
FROM machine_runtime_app_sessions
WHERE id = $1;

-- name: GetMachineRuntimeAppSessionByBootAndStart :one
SELECT *
FROM machine_runtime_app_sessions
WHERE machine_id = $1
    AND boot_id = $2
    AND app_start_id = $3
    AND ended_at IS NULL
ORDER BY started_at DESC
LIMIT 1;

-- name: ListMachineRuntimeAppSessionHistory :many
SELECT *
FROM machine_runtime_app_sessions
WHERE machine_id = $1
ORDER BY started_at DESC
LIMIT sqlc.arg (limit_val) OFFSET sqlc.arg (offset_val);

-- name: HeartbeatMachineRuntimeAppSession :one
UPDATE machine_runtime_app_sessions
SET
    last_heartbeat_at = $2,
    status = $3,
    last_network_state = $4,
    last_mqtt_state = $5,
    storefront_state = $6,
    sell_ready = $7,
    blockers = $8,
    hardware_status = $9,
    catalog_status = $10,
    outbox_status = $11,
    recovery_status = $12,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: EndMachineRuntimeAppSession :one
UPDATE machine_runtime_app_sessions
SET
    status = $2,
    end_reason = $3,
    ended_at = $4,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkMachineRuntimeAppSessionStale :one
UPDATE machine_runtime_app_sessions
SET
    status = 'STALE',
    updated_at = now()
WHERE id = $1 AND ended_at IS NULL
RETURNING *;

-- name: MarkMachineRuntimeAppSessionCrashed :one
UPDATE machine_runtime_app_sessions
SET
    status = 'CRASHED',
    end_reason = 'APP_CRASH_DETECTED',
    ended_at = now(),
    updated_at = now()
WHERE id = $1 AND ended_at IS NULL
RETURNING *;

-- name: CloseCurrentRuntimeAppSessionForMachine :many
UPDATE machine_runtime_app_sessions
SET
    status = $2,
    end_reason = $3,
    ended_at = now(),
    updated_at = now()
WHERE machine_id = $1
    AND ended_at IS NULL
    AND status IN ('STARTING', 'ONLINE', 'STALE', 'OFFLINE', 'BLOCKED', 'MAINTENANCE')
RETURNING *;

-- name: UpdateMachineCurrentRuntimeAppSession :exec
UPDATE machines
SET
    current_runtime_app_session_id = $2,
    updated_at = now()
WHERE id = $1;

-- name: UpdateMachineOnlineStatus :exec
UPDATE machines
SET
    online_status = $2,
    last_seen_at = COALESCE($3, last_seen_at),
    updated_at = now()
WHERE id = $1;

-- name: TouchRuntimeAppSessionMQTT :exec
UPDATE machine_runtime_app_sessions
SET
    last_mqtt_seen_at = $2,
    last_mqtt_state = $3,
    updated_at = now()
WHERE id = $1 AND ended_at IS NULL;

-- name: CountCurrentMachineRuntimeAppSessions :one
SELECT count(*)::bigint AS count
FROM machine_runtime_app_sessions
WHERE machine_id = $1
    AND ended_at IS NULL
    AND status IN ('STARTING', 'ONLINE', 'STALE', 'OFFLINE', 'BLOCKED', 'MAINTENANCE');

-- name: UpdateMachineCurrentSnapshotRuntime :exec
UPDATE machine_current_snapshot
SET
    current_device_attachment_id = $2,
    current_runtime_app_session_id = $3,
    online_status = $4,
    runtime_session_status = $5,
    runtime_start_reason = $6,
    runtime_started_at = $7,
    runtime_last_heartbeat_at = $8,
    last_mqtt_state = $9,
    storefront_state = $10,
    sell_ready = $11,
    blockers = $12,
    updated_at = now()
WHERE machine_id = $1;
