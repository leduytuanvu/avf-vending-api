-- name: InsertRefillSession :one
INSERT INTO refill_sessions (
    organization_id,
    machine_id,
    started_at,
    ended_at,
    operator_session_id,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING
    id,
    organization_id,
    machine_id,
    started_at,
    ended_at,
    operator_session_id,
    metadata,
    created_at;

-- name: InsertMachineConfigApplication :one
INSERT INTO machine_configs (
    organization_id,
    machine_id,
    applied_at,
    config_revision,
    config_payload,
    operator_session_id,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    id,
    organization_id,
    machine_id,
    applied_at,
    config_revision,
    config_payload,
    operator_session_id,
    metadata,
    created_at;

-- name: InsertIncident :one
INSERT INTO incidents (
    organization_id,
    machine_id,
    status,
    title,
    opened_at,
    updated_at,
    operator_session_id,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
    id,
    organization_id,
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
    status = $2,
    title = $3,
    metadata = $4,
    operator_session_id = $5,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $6
RETURNING
    id,
    organization_id,
    machine_id,
    status,
    title,
    opened_at,
    updated_at,
    operator_session_id,
    metadata;
