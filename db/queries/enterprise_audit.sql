-- name: EnterpriseAuditInsertEvent :one
INSERT INTO audit_events (
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
    COALESCE(sqlc.narg('occurred_at')::timestamptz, now())
)
RETURNING *;

-- name: EnterpriseAuditGetEvent :one
SELECT
    *
FROM
    audit_events
WHERE
    id = $1
    AND TRUE;

-- name: EnterpriseAuditCountEvents :one
SELECT
    count(*)::bigint
FROM
    audit_events
WHERE
    (
        $1::text IS NULL
        OR btrim($1::text) = ''
        OR action = $1
    )
    AND (
        $2::text IS NULL
        OR btrim($2::text) = ''
        OR actor_id = $2
    )
    AND (
        $3::text IS NULL
        OR btrim($3::text) = ''
        OR actor_type = $3
    )
    AND (
        $4::text IS NULL
        OR btrim($4::text) = ''
        OR outcome = $4
    )
    AND (
        $5::text IS NULL
        OR btrim($5::text) = ''
        OR resource_type = $5
    )
    AND (
        $6::text IS NULL
        OR btrim($6::text) = ''
        OR resource_id = $6
    )
    AND (
        $7::text IS NULL
        OR btrim($7::text) = ''
        OR created_at >= $7::timestamptz
    )
    AND (
        $8::text IS NULL
        OR btrim($8::text) = ''
        OR created_at <= $8::timestamptz
    )
    AND (
        $9::text IS NULL
        OR btrim($9::text) = ''
        OR machine_id::text = $9
    );

-- name: EnterpriseAuditListEvents :many
SELECT
    *
FROM
    audit_events
WHERE
    (
        $1::text IS NULL
        OR btrim($1::text) = ''
        OR action = $1
    )
    AND (
        $2::text IS NULL
        OR btrim($2::text) = ''
        OR actor_id = $2
    )
    AND (
        $3::text IS NULL
        OR btrim($3::text) = ''
        OR actor_type = $3
    )
    AND (
        $4::text IS NULL
        OR btrim($4::text) = ''
        OR outcome = $4
    )
    AND (
        $5::text IS NULL
        OR btrim($5::text) = ''
        OR resource_type = $5
    )
    AND (
        $6::text IS NULL
        OR btrim($6::text) = ''
        OR resource_id = $6
    )
    AND (
        $7::text IS NULL
        OR btrim($7::text) = ''
        OR created_at >= $7::timestamptz
    )
    AND (
        $8::text IS NULL
        OR btrim($8::text) = ''
        OR created_at <= $8::timestamptz
    )
    AND (
        $9::text IS NULL
        OR btrim($9::text) = ''
        OR machine_id::text = $9
    )
ORDER BY
    created_at DESC
LIMIT $10 OFFSET $11;
