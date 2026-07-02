-- name: GetCurrentMachineRuntimeSession :one
SELECT
    ms.id,
    ms.machine_id,
    ms.credential_version,
    ms.status,
    ms.issued_at,
    ms.expires_at,
    ms.revoked_at,
    ms.last_used_at,
    ms.user_agent,
    ms.ip_address
FROM
    machine_sessions ms
WHERE
    ms.machine_id = $1
    AND ms.status = 'active'
    AND ms.revoked_at IS NULL
    AND ms.expires_at > now()
ORDER BY
    ms.issued_at DESC
LIMIT
    1;

-- name: ListMachineRuntimeSessionHistory :many
SELECT
    ms.id,
    ms.machine_id,
    ms.credential_version,
    ms.status,
    ms.issued_at,
    ms.expires_at,
    ms.revoked_at,
    ms.last_used_at,
    ms.user_agent,
    ms.ip_address
FROM
    machine_sessions ms
WHERE
    ms.machine_id = $1
ORDER BY
    ms.issued_at DESC
LIMIT $2 OFFSET $3;

-- name: CountActiveMachineSessionsForMachine :one
SELECT
    count(*)::bigint AS cnt
FROM
    machine_sessions
WHERE
    machine_id = $1
    AND status = 'active'
    AND revoked_at IS NULL
    AND expires_at > now();
