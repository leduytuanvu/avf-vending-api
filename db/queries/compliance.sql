-- name: InsertAuditLog :one
INSERT INTO audit_logs (
    actor_type,
    actor_id,
    action,
    resource_type,
    resource_id,
    payload,
    ip
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
RETURNING *;

-- name: ListAuditLogsForCompany :many
SELECT
    id,
    actor_type,
    actor_id,
    action,
    resource_type,
    resource_id,
    payload,
    ip,
    created_at
FROM audit_logs
ORDER BY
    created_at DESC
LIMIT $1;

-- name: ListAuditLogsForActorInCompany :many
SELECT
    id,
    actor_type,
    actor_id,
    action,
    resource_type,
    resource_id,
    payload,
    ip,
    created_at
FROM audit_logs
WHERE
    actor_type = $1
    AND actor_id = $2
ORDER BY
    created_at DESC
LIMIT $3;
