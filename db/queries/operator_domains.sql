-- name: InsertRefillSession :one
INSERT INTO refill_sessions (
    machine_id,
    started_at,
    ended_at,
    operator_session_id,
    metadata
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING
    id,
    machine_id,
    started_at,
    ended_at,
    operator_session_id,
    metadata,
    created_at;

-- name: InsertMachineConfigApplication :one
INSERT INTO machine_configs (
    machine_id,
    applied_at,
    config_revision,
    config_payload,
    operator_session_id,
    metadata
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING
    id,
    machine_id,
    applied_at,
    config_revision,
    config_payload,
    operator_session_id,
    metadata,
    created_at;

-- name: InsertIncident :one
INSERT INTO incidents (
    machine_id,
    status,
    title,
    opened_at,
    updated_at,
    operator_session_id,
    metadata
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING
    id,
    machine_id,
    status,
    title,
    opened_at,
    updated_at,
    operator_session_id,
    metadata;

-- name: UpdateIncidentFromOperator :one
UPDATE incidents
SET
    status = $1,
    title = $2,
    metadata = $3,
    operator_session_id = $4,
    updated_at = now()
WHERE
    id = $5
    AND TRUE
RETURNING
    id,
    machine_id,
    status,
    title,
    opened_at,
    updated_at,
    operator_session_id,
    metadata;

-- name: InsertInventoryCountSession :one
INSERT INTO inventory_count_sessions (
    machine_id,
    operator_session_id,
    status,
    started_at,
    metadata
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING
    id,
    machine_id,
    operator_session_id,
    status,
    started_at,
    ended_at,
    metadata,
    created_at;

-- name: UpdateInventoryCountSessionClose :one
UPDATE inventory_count_sessions
SET
    status = $1,
    ended_at = now(),
    metadata = metadata || $2::jsonb
WHERE
    id = $3
    AND TRUE
RETURNING
    id,
    machine_id,
    operator_session_id,
    status,
    started_at,
    ended_at,
    metadata,
    created_at;
