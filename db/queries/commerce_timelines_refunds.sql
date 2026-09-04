-- Order timeline + refund_requests (admin commerce P0.4).

-- name: CommerceAdminOrderLookup :one
SELECT
    id
FROM orders
WHERE
    id = $1;

-- name: CommerceAdminCountOrderTimeline :one
SELECT count(*)::bigint
FROM order_timelines
WHERE
    order_id = $1;

-- name: InsertOrderTimelineEvent :exec
INSERT INTO order_timelines (
    order_id,
    event_type,
    actor_type,
    actor_id,
    payload,
    occurred_at
) VALUES (
    sqlc.arg('order_id'),
    sqlc.arg('event_type'),
    sqlc.arg('actor_type'),
    sqlc.arg('actor_id'),
    COALESCE(NULLIF(sqlc.arg('payload')::text, '')::jsonb, '{}'::jsonb),
    sqlc.arg('occurred_at')
);

-- name: CommerceAdminListOrderTimeline :many
SELECT
    id,
    order_id,
    event_type,
    actor_type,
    actor_id,
    payload,
    occurred_at,
    created_at
FROM order_timelines
WHERE
    order_id = $1
ORDER BY
    occurred_at DESC
LIMIT $2 OFFSET $3;

-- name: CommerceAdminInsertRefundRequest :one
INSERT INTO refund_requests (
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
    $8
) RETURNING *;

-- name: CommerceAdminUpdateRefundRequestLinkedRefund :one
UPDATE refund_requests
SET
    refund_id = $1,
    status = $2,
    updated_at = now(),
    completed_at = CASE WHEN $2 IN ('succeeded', 'failed') THEN now() ELSE completed_at END
WHERE
    id = $3
RETURNING *;

-- name: CommerceAdminGetRefundRequestByIdempotencyKey :one
SELECT *
FROM refund_requests
WHERE
    idempotency_key = $1;

-- name: CommerceAdminListRefundRequests :many
SELECT *
FROM refund_requests
WHERE
    ($1::boolean IS FALSE OR status = $2::text)
ORDER BY
    created_at DESC
LIMIT $3 OFFSET $4;

-- name: CommerceAdminCountRefundRequests :one
SELECT count(*)::bigint
FROM refund_requests
WHERE
    ($1::boolean IS FALSE OR status = $2::text);

-- name: CommerceAdminGetRefundRequest :one
SELECT *
FROM refund_requests
WHERE
    id = $1;
