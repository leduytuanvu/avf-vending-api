-- name: InsertAuditLog :one
INSERT INTO audit_logs (
    organization_id,
    actor_type,
    actor_id,
    action,
    resource_type,
    resource_id,
    payload,
    ip
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListAuditLogsForOrganization :many
SELECT
    id,
    organization_id,
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
    organization_id = $1
ORDER BY
    created_at DESC
LIMIT $2;

-- name: ListAuditLogsForActorInOrganization :many
SELECT
    id,
    organization_id,
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
    organization_id = $1
    AND actor_type = $2
    AND actor_id = $3
ORDER BY
    created_at DESC
LIMIT $4;
