-- name: InsertCommandLedgerEntry :one
INSERT INTO command_ledger (
    machine_id,
    organization_id,
    sequence,
    command_type,
    payload,
    correlation_id,
    idempotency_key,
    operator_session_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
    id,
    machine_id,
    organization_id,
    sequence,
    command_type,
    payload,
    correlation_id,
    idempotency_key,
    created_at,
    protocol_type,
    deadline_at,
    timeout_at,
    attempt_count,
    last_attempt_at,
    route_key,
    source_system,
    source_event_id,
    operator_session_id,
    max_dispatch_attempts;

-- name: UpsertMachineShadowDesired :one
INSERT INTO machine_shadow (
    machine_id,
    desired_state,
    reported_state,
    version,
    updated_at
)
VALUES ($1, $2, '{}'::jsonb, 1, now())
ON CONFLICT (machine_id) DO UPDATE
SET
    desired_state = excluded.desired_state,
    version = machine_shadow.version + 1,
    updated_at = now()
RETURNING *;

-- name: GetMachineShadowByMachineID :one
SELECT *
FROM machine_shadow
WHERE machine_id = $1;

-- name: GetCommandLedgerByMachineIdempotency :one
SELECT
    id,
    machine_id,
    organization_id,
    sequence,
    command_type,
    payload,
    correlation_id,
    idempotency_key,
    created_at,
    protocol_type,
    deadline_at,
    timeout_at,
    attempt_count,
    last_attempt_at,
    route_key,
    source_system,
    source_event_id,
    operator_session_id,
    max_dispatch_attempts
FROM command_ledger
WHERE
    machine_id = $1
    AND idempotency_key = $2;

-- name: UpsertMachineShadowReported :one
INSERT INTO machine_shadow (
    machine_id,
    desired_state,
    reported_state,
    version,
    updated_at
)
VALUES ($1, '{}'::jsonb, $2, 1, now())
ON CONFLICT (machine_id) DO UPDATE
SET
    reported_state = excluded.reported_state,
    version = machine_shadow.version + 1,
    updated_at = now()
RETURNING machine_id, desired_state, reported_state, version, updated_at;

-- name: TouchMachineConnectivity :exec
UPDATE machines
SET
    updated_at = now(),
    status = CASE
        WHEN status = 'offline' THEN 'online'
        WHEN status = 'online' THEN 'online'
        ELSE status
    END
WHERE id = $1;

-- name: InsertDeviceTelemetryEvent :one
INSERT INTO device_telemetry_events (
    machine_id,
    event_type,
    payload,
    dedupe_key
)
VALUES ($1, $2, $3, $4)
RETURNING id, machine_id, event_type, payload, dedupe_key, received_at;

-- name: InsertDeviceCommandReceipt :one
INSERT INTO device_command_receipts (
    machine_id,
    sequence,
    status,
    correlation_id,
    payload,
    dedupe_key,
    command_attempt_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    id,
    machine_id,
    sequence,
    status,
    correlation_id,
    payload,
    dedupe_key,
    received_at,
    command_attempt_id;
