-- name: GetOperatorSessionByID :one
SELECT
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    id = $1;

-- name: GetActiveOperatorSessionForMachine :one
SELECT
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    machine_id = $1
    AND status = 'ACTIVE'
ORDER BY
    started_at DESC
LIMIT
    1;

-- name: GetOperatorSessionByIDForUpdate :one
SELECT
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    id = $1
FOR UPDATE;

-- name: GetActiveOperatorSessionByMachineID :one
SELECT
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    machine_id = $1
    AND status = 'ACTIVE'
LIMIT
    1;

-- name: GetActiveOperatorSessionByMachineIDForUpdate :one
SELECT
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    machine_id = $1
    AND status = 'ACTIVE'
LIMIT
    1
FOR UPDATE;

-- name: ResumeActiveOperatorSessionForActor :one
UPDATE machine_operator_sessions
SET
    updated_at = now(),
    last_activity_at = now(),
    expires_at = COALESCE(sqlc.arg('expires_at'), expires_at),
    client_metadata = COALESCE(NULLIF(sqlc.arg('client_metadata')::text, '')::jsonb, '{}'::jsonb)
WHERE
    machine_id = sqlc.arg('machine_id')
    AND TRUE
    AND status = 'ACTIVE'
    AND actor_type = sqlc.arg('actor_type')
    AND technician_id IS NOT DISTINCT FROM sqlc.arg('technician_id')
    AND user_principal IS NOT DISTINCT FROM sqlc.arg('user_principal')
RETURNING
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at;

-- name: InsertMachineOperatorSession :one
INSERT INTO machine_operator_sessions (
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    expires_at,
    client_metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    COALESCE(NULLIF(sqlc.arg('client_metadata')::text, '')::jsonb, '{}'::jsonb)
)
RETURNING
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at;

-- name: EndMachineOperatorSession :one
UPDATE machine_operator_sessions
SET
    status = $1,
    ended_at = $2,
    updated_at = $2,
    ended_reason = $3
WHERE
    id = $4
    AND status = 'ACTIVE'
RETURNING
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at;

-- name: TouchMachineOperatorSessionActivity :one
UPDATE machine_operator_sessions
SET
    updated_at = now(),
    last_activity_at = now()
WHERE
    id = $1
    AND status = 'ACTIVE'
RETURNING
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at;

-- name: TimeoutMachineOperatorSessionIfExpired :one
UPDATE machine_operator_sessions
SET
    status = 'EXPIRED',
    ended_at = now(),
    updated_at = now(),
    ended_reason = 'session_expired'
WHERE
    id = $1
    AND status = 'ACTIVE'
    AND expires_at IS NOT NULL
    AND expires_at <= now()
RETURNING
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at;

-- name: InsertMachineOperatorAuthEvent :one
INSERT INTO machine_operator_auth_events (
    operator_session_id,
    machine_id,
    event_type,
    auth_method,
    occurred_at,
    correlation_id,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    COALESCE($5::timestamptz, now()),
    $6,
    COALESCE(NULLIF(sqlc.arg('metadata')::text, '')::jsonb, '{}'::jsonb)
)
RETURNING
    id,
    operator_session_id,
    machine_id,
    event_type,
    auth_method,
    occurred_at,
    correlation_id,
    metadata;

-- name: InsertMachineActionAttribution :one
INSERT INTO machine_action_attributions (
    operator_session_id,
    machine_id,
    action_origin_type,
    resource_type,
    resource_id,
    occurred_at,
    metadata,
    correlation_id
) VALUES (
    sqlc.narg('operator_session_id'),
    sqlc.arg('machine_id'),
    sqlc.arg('action_origin_type'),
    sqlc.arg('resource_type'),
    sqlc.arg('resource_id'),
    COALESCE(sqlc.arg('occurred_at')::timestamptz, now()),
    COALESCE(NULLIF(sqlc.arg('metadata')::text, '')::jsonb, '{}'::jsonb),
    sqlc.narg('correlation_id')
)
RETURNING
    id,
    operator_session_id,
    machine_id,
    action_origin_type,
    resource_type,
    resource_id,
    occurred_at,
    metadata,
    correlation_id;

-- name: ListOperatorSessionsByMachineID :many
SELECT
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    machine_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: ListOperatorSessionsByTechnicianID :many
SELECT
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    technician_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: ListOperatorSessionsByUserPrincipal :many
SELECT
    id,
    machine_id,
    actor_type,
    technician_id,
    user_principal,
    status,
    started_at,
    ended_at,
    expires_at,
    client_metadata,
    last_activity_at,
    ended_reason,
    created_at,
    updated_at
FROM machine_operator_sessions
WHERE
    actor_type = 'USER'
    AND user_principal = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: ListMachineOperatorAuthEventsByMachineID :many
SELECT
    id,
    operator_session_id,
    machine_id,
    event_type,
    auth_method,
    occurred_at,
    correlation_id,
    metadata
FROM machine_operator_auth_events
WHERE
    machine_id = $1
ORDER BY occurred_at DESC
LIMIT $2;

-- name: ListMachineActionAttributionsByMachineID :many
SELECT
    id,
    operator_session_id,
    machine_id,
    action_origin_type,
    resource_type,
    resource_id,
    occurred_at,
    metadata,
    correlation_id
FROM machine_action_attributions
WHERE
    machine_id = $1
ORDER BY occurred_at DESC
LIMIT $2;

-- name: ListMachineActionAttributionsByMachineAndResource :many
SELECT
    id,
    operator_session_id,
    machine_id,
    action_origin_type,
    resource_type,
    resource_id,
    occurred_at,
    metadata,
    correlation_id
FROM machine_action_attributions
WHERE
    machine_id = $1
    AND resource_type = $2
    AND resource_id = $3
ORDER BY occurred_at DESC
LIMIT $4;

-- name: ListMachineActionAttributionsForTechnician :many
SELECT
    a.id,
    a.operator_session_id,
    a.machine_id,
    a.action_origin_type,
    a.resource_type,
    a.resource_id,
    a.occurred_at,
    a.metadata,
    a.correlation_id
FROM machine_action_attributions a
INNER JOIN machine_operator_sessions s ON s.id = a.operator_session_id
WHERE
    s.technician_id = $1
    AND TRUE
ORDER BY
    a.occurred_at DESC
LIMIT $2;

-- name: ListMachineActionAttributionsForUserPrincipal :many
SELECT
    a.id,
    a.operator_session_id,
    a.machine_id,
    a.action_origin_type,
    a.resource_type,
    a.resource_id,
    a.occurred_at,
    a.metadata,
    a.correlation_id
FROM machine_action_attributions a
INNER JOIN machine_operator_sessions s ON s.id = a.operator_session_id
WHERE
    s.actor_type = 'USER'
    AND s.user_principal = $1
ORDER BY
    a.occurred_at DESC
LIMIT $2;
