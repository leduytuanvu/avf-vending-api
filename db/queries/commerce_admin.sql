-- name: CommerceAdminListOrders :many
SELECT
    o.id,
    o.machine_id,
    o.status,
    o.currency,
    o.subtotal_minor,
    o.tax_minor,
    o.total_minor,
    o.idempotency_key,
    o.created_at,
    o.updated_at
FROM orders o
WHERE
    ($1::boolean IS FALSE OR o.status = $2::text)
    AND ($3::boolean IS FALSE OR o.machine_id = $4::uuid)
    AND o.created_at >= $5::timestamptz
    AND o.created_at <= $6::timestamptz
    AND (
        $7::boolean IS FALSE
        OR o.id::text ILIKE ('%' || $8::text || '%')
        OR (
            o.idempotency_key IS NOT NULL
            AND o.idempotency_key::text ILIKE ('%' || $8::text || '%')
        )
    )
ORDER BY
    o.created_at DESC
LIMIT $9 OFFSET $10;

-- name: CommerceAdminCountOrders :one
SELECT
    count(*)::bigint AS cnt
FROM orders o
WHERE
    ($1::boolean IS FALSE OR o.status = $2::text)
    AND ($3::boolean IS FALSE OR o.machine_id = $4::uuid)
    AND o.created_at >= $5::timestamptz
    AND o.created_at <= $6::timestamptz
    AND (
        $7::boolean IS FALSE
        OR o.id::text ILIKE ('%' || $8::text || '%')
        OR (
            o.idempotency_key IS NOT NULL
            AND o.idempotency_key::text ILIKE ('%' || $8::text || '%')
        )
    );

-- name: CommerceAdminListPayments :many
SELECT
    p.id AS payment_id,
    p.order_id,
    o.machine_id,
    p.provider,
    p.state AS payment_state,
    p.amount_minor,
    p.currency,
    p.reconciliation_status,
    p.settlement_status,
    p.created_at,
    p.updated_at,
    o.status AS order_status
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
WHERE
    ($1::boolean IS FALSE OR p.state = $2::text)
    AND ($3::boolean IS FALSE OR p.provider = $4::text)
    AND ($5::boolean IS FALSE OR o.machine_id = $6::uuid)
    AND p.created_at >= $7::timestamptz
    AND p.created_at <= $8::timestamptz
    AND (
        $9::boolean IS FALSE
        OR p.id::text ILIKE ('%' || $10::text || '%')
        OR o.id::text ILIKE ('%' || $10::text || '%')
        OR (
            p.idempotency_key IS NOT NULL
            AND p.idempotency_key::text ILIKE ('%' || $10::text || '%')
        )
    )
ORDER BY
    p.created_at DESC
LIMIT $11 OFFSET $12;

-- name: CommerceAdminCountPayments :one
SELECT
    count(*)::bigint AS cnt
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
WHERE
    ($1::boolean IS FALSE OR p.state = $2::text)
    AND ($3::boolean IS FALSE OR p.provider = $4::text)
    AND ($5::boolean IS FALSE OR o.machine_id = $6::uuid)
    AND p.created_at >= $7::timestamptz
    AND p.created_at <= $8::timestamptz
    AND (
        $9::boolean IS FALSE
        OR p.id::text ILIKE ('%' || $10::text || '%')
        OR o.id::text ILIKE ('%' || $10::text || '%')
        OR (
            p.idempotency_key IS NOT NULL
            AND p.idempotency_key::text ILIKE ('%' || $10::text || '%')
        )
    );

-- name: CommerceAdminListReconciliationCases :many
SELECT
    id,
    case_type,
    status,
    severity,
    order_id,
    payment_id,
    vend_session_id,
    refund_id,
    machine_id,
    provider,
    provider_event_id,
    correlation_key,
    reason,
    metadata,
    first_detected_at,
    last_detected_at,
    resolved_at,
    resolved_by,
    resolution_note
FROM commerce_reconciliation_cases
WHERE
    ($1::boolean IS FALSE OR status = $2::text)
    AND ($3::boolean IS FALSE OR case_type = $4::text)
ORDER BY
    last_detected_at DESC
LIMIT $5 OFFSET $6;

-- name: CommerceAdminCountReconciliationCases :one
SELECT count(*)::bigint
FROM commerce_reconciliation_cases
WHERE
    ($1::boolean IS FALSE OR status = $2::text)
    AND ($3::boolean IS FALSE OR case_type = $4::text);

-- name: CommerceAdminGetReconciliationCase :one
SELECT
    id,
    case_type,
    status,
    severity,
    order_id,
    payment_id,
    vend_session_id,
    refund_id,
    machine_id,
    provider,
    provider_event_id,
    correlation_key,
    reason,
    metadata,
    first_detected_at,
    last_detected_at,
    resolved_at,
    resolved_by,
    resolution_note
FROM commerce_reconciliation_cases
WHERE
    id = $1;

-- name: CommerceAdminResolveReconciliationCase :one
UPDATE commerce_reconciliation_cases
SET
    status = $1,
    resolved_at = now(),
    resolved_by = $2,
    resolution_note = $3
WHERE
    id = $4
    AND status IN ('open', 'reviewing', 'escalated')
RETURNING *;
