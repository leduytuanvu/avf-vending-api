-- Order timeline + refund_requests (admin commerce P0.4).

-- name: CommerceAdminOrderOrganizationID :one
SELECT
    organization_id
FROM orders
WHERE
    id = $1;

-- name: CommerceAdminCountOrderTimeline :one
SELECT count(*)::bigint
FROM order_timelines
WHERE
    organization_id = $1
    AND order_id = $2;

-- name: InsertOrderTimelineEvent :exec
INSERT INTO order_timelines (
    organization_id,
    order_id,
    event_type,
    actor_type,
    actor_id,
    payload,
    occurred_at
) VALUES (
    sqlc.arg('organization_id'),
    sqlc.arg('order_id'),
    sqlc.arg('event_type'),
    sqlc.arg('actor_type'),
    sqlc.arg('actor_id'),
    COALESCE(sqlc.arg('payload')::jsonb, '{}'::jsonb),
    sqlc.arg('occurred_at')
);

-- name: CommerceAdminListOrderTimeline :many
SELECT
    id,
    organization_id,
    order_id,
    event_type,
    actor_type,
    actor_id,
    payload,
    occurred_at,
    created_at
FROM order_timelines
WHERE
    organization_id = $1
    AND order_id = $2
ORDER BY
    occurred_at DESC
LIMIT $3 OFFSET $4;

-- name: CommerceAdminInsertRefundRequest :one
INSERT INTO refund_requests (
    organization_id,
    order_id,
    payment_id,
    amount_minor,
    currency,
    reason,
    status,
    requested_by,
    idempotency_key
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
) RETURNING *;

-- name: CommerceAdminUpdateRefundRequestLinkedRefund :one
UPDATE refund_requests
SET
    refund_id = $3,
    status = $4,
    updated_at = now(),
    completed_at = CASE WHEN $4 IN ('succeeded', 'failed') THEN now() ELSE completed_at END
WHERE
    organization_id = $1
    AND id = $2
RETURNING *;

-- name: CommerceAdminGetRefundRequestByOrgIdempotency :one
SELECT *
FROM refund_requests
WHERE
    organization_id = $1
    AND idempotency_key = $2;

-- name: CommerceAdminListRefundRequests :many
SELECT *
FROM refund_requests
WHERE
    organization_id = $1
    AND ($2::boolean IS FALSE OR status = $3::text)
ORDER BY
    created_at DESC
LIMIT $4 OFFSET $5;

-- name: CommerceAdminCountRefundRequests :one
SELECT count(*)::bigint
FROM refund_requests
WHERE
    organization_id = $1
    AND ($2::boolean IS FALSE OR status = $3::text);

-- name: CommerceAdminGetRefundRequest :one
SELECT *
FROM refund_requests
WHERE
    organization_id = $1
    AND id = $2;
