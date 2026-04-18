-- name: GetOperatorSessionByID :one
SELECT
    id,
    organization_id,
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

-- name: GetOperatorSessionByIDForUpdate :one
SELECT
    id,
    organization_id,
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
    organization_id,
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
    organization_id,
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
    expires_at = COALESCE($6, expires_at),
    client_metadata = $7
WHERE
    machine_id = $1
    AND organization_id = $2
    AND status = 'ACTIVE'
    AND actor_type = $3
    AND technician_id IS NOT DISTINCT FROM $4
    AND user_principal IS NOT DISTINCT FROM $5
RETURNING
    id,
    organization_id,
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
    organization_id,
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
    $7,
    $8
)
RETURNING
    id,
    organization_id,
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
    status = $2,
    ended_at = $3,
    updated_at = $3,
    ended_reason = $4
WHERE
    id = $1
    AND status = 'ACTIVE'
RETURNING
    id,
    organization_id,
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
    organization_id,
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
    organization_id,
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
    $7
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
    $1,
    $2,
    $3,
    $4,
    $5,
    COALESCE($6::timestamptz, now()),
    $7,
    $8
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
    organization_id,
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
    organization_id,
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
    organization_id,
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
    organization_id = $1
    AND actor_type = 'USER'
    AND user_principal = $2
ORDER BY started_at DESC
LIMIT $3;

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
    AND s.organization_id = $2
ORDER BY
    a.occurred_at DESC
LIMIT $3;

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
    s.organization_id = $1
    AND s.actor_type = 'USER'
    AND s.user_principal = $2
ORDER BY
    a.occurred_at DESC
LIMIT $3;
