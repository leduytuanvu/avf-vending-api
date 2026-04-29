-- name: EnterpriseAuditInsertEvent :one
INSERT INTO audit_events (
    organization_id,
    actor_type,
    actor_id,
    action,
    resource_type,
    resource_id,
    machine_id,
    site_id,
    request_id,
    trace_id,
    ip_address,
    user_agent,
    before_json,
    after_json,
    metadata,
    outcome,
    occurred_at
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13,
    $14,
    $15,
    $16,
    COALESCE(sqlc.narg('occurred_at')::timestamptz, now())
)
RETURNING *;

-- name: EnterpriseAuditGetEventForOrg :one
SELECT
    *
FROM
    audit_events
WHERE
    id = $1
    AND organization_id = $2;

-- name: EnterpriseAuditCountEvents :one
SELECT
    count(*)::bigint
FROM
    audit_events
WHERE
    organization_id = $1
    AND (
        $2::text IS NULL
        OR btrim($2::text) = ''
        OR action = $2
    )
    AND (
        $3::text IS NULL
        OR btrim($3::text) = ''
        OR actor_id = $3
    )
    AND (
        $4::text IS NULL
        OR btrim($4::text) = ''
        OR actor_type = $4
    )
    AND (
        $5::text IS NULL
        OR btrim($5::text) = ''
        OR outcome = $5
    )
    AND (
        $6::text IS NULL
        OR btrim($6::text) = ''
        OR resource_type = $6
    )
    AND (
        $7::text IS NULL
        OR btrim($7::text) = ''
        OR resource_id = $7
    )
    AND (
        $8::text IS NULL
        OR btrim($8::text) = ''
        OR created_at >= $8::timestamptz
    )
    AND (
        $9::text IS NULL
        OR btrim($9::text) = ''
        OR created_at <= $9::timestamptz
    )
    AND (
        $10::text IS NULL
        OR btrim($10::text) = ''
        OR machine_id::text = $10
    );

-- name: EnterpriseAuditListEvents :many
SELECT
    *
FROM
    audit_events
WHERE
    organization_id = $1
    AND (
        $2::text IS NULL
        OR btrim($2::text) = ''
        OR action = $2
    )
    AND (
        $3::text IS NULL
        OR btrim($3::text) = ''
        OR actor_id = $3
    )
    AND (
        $4::text IS NULL
        OR btrim($4::text) = ''
        OR actor_type = $4
    )
    AND (
        $5::text IS NULL
        OR btrim($5::text) = ''
        OR outcome = $5
    )
    AND (
        $6::text IS NULL
        OR btrim($6::text) = ''
        OR resource_type = $6
    )
    AND (
        $7::text IS NULL
        OR btrim($7::text) = ''
        OR resource_id = $7
    )
    AND (
        $8::text IS NULL
        OR btrim($8::text) = ''
        OR created_at >= $8::timestamptz
    )
    AND (
        $9::text IS NULL
        OR btrim($9::text) = ''
        OR created_at <= $9::timestamptz
    )
    AND (
        $10::text IS NULL
        OR btrim($10::text) = ''
        OR machine_id::text = $10
    )
ORDER BY
    created_at DESC
LIMIT $11 OFFSET $12;
