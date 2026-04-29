-- name: CommerceAdminListOrders :many
SELECT
    o.id,
    o.organization_id,
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
    o.organization_id = $1
    AND ($2::boolean IS FALSE OR o.status = $3::text)
    AND ($4::boolean IS FALSE OR o.machine_id = $5::uuid)
    AND o.created_at >= $6::timestamptz
    AND o.created_at <= $7::timestamptz
    AND (
        $8::boolean IS FALSE
        OR o.id::text ILIKE ('%' || $9::text || '%')
        OR (
            o.idempotency_key IS NOT NULL
            AND o.idempotency_key::text ILIKE ('%' || $9::text || '%')
        )
    )
ORDER BY
    o.created_at DESC
LIMIT $10 OFFSET $11;

-- name: CommerceAdminCountOrders :one
SELECT
    count(*)::bigint AS cnt
FROM orders o
WHERE
    o.organization_id = $1
    AND ($2::boolean IS FALSE OR o.status = $3::text)
    AND ($4::boolean IS FALSE OR o.machine_id = $5::uuid)
    AND o.created_at >= $6::timestamptz
    AND o.created_at <= $7::timestamptz
    AND (
        $8::boolean IS FALSE
        OR o.id::text ILIKE ('%' || $9::text || '%')
        OR (
            o.idempotency_key IS NOT NULL
            AND o.idempotency_key::text ILIKE ('%' || $9::text || '%')
        )
    );

-- name: CommerceAdminListPayments :many
SELECT
    p.id AS payment_id,
    p.order_id,
    o.organization_id,
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
    o.organization_id = $1
    AND ($2::boolean IS FALSE OR p.state = $3::text)
    AND ($4::boolean IS FALSE OR p.provider = $5::text)
    AND ($6::boolean IS FALSE OR o.machine_id = $7::uuid)
    AND p.created_at >= $8::timestamptz
    AND p.created_at <= $9::timestamptz
    AND (
        $10::boolean IS FALSE
        OR p.id::text ILIKE ('%' || $11::text || '%')
        OR o.id::text ILIKE ('%' || $11::text || '%')
        OR (
            p.idempotency_key IS NOT NULL
            AND p.idempotency_key::text ILIKE ('%' || $11::text || '%')
        )
    )
ORDER BY
    p.created_at DESC
LIMIT $12 OFFSET $13;

-- name: CommerceAdminCountPayments :one
SELECT
    count(*)::bigint AS cnt
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
WHERE
    o.organization_id = $1
    AND ($2::boolean IS FALSE OR p.state = $3::text)
    AND ($4::boolean IS FALSE OR p.provider = $5::text)
    AND ($6::boolean IS FALSE OR o.machine_id = $7::uuid)
    AND p.created_at >= $8::timestamptz
    AND p.created_at <= $9::timestamptz
    AND (
        $10::boolean IS FALSE
        OR p.id::text ILIKE ('%' || $11::text || '%')
        OR o.id::text ILIKE ('%' || $11::text || '%')
        OR (
            p.idempotency_key IS NOT NULL
            AND p.idempotency_key::text ILIKE ('%' || $11::text || '%')
        )
    );

-- name: CommerceAdminListReconciliationCases :many
SELECT
    id,
    organization_id,
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
    organization_id = $1
    AND ($2::boolean IS FALSE OR status = $3::text)
    AND ($4::boolean IS FALSE OR case_type = $5::text)
ORDER BY
    last_detected_at DESC
LIMIT $6 OFFSET $7;

-- name: CommerceAdminCountReconciliationCases :one
SELECT count(*)::bigint
FROM commerce_reconciliation_cases
WHERE
    organization_id = $1
    AND ($2::boolean IS FALSE OR status = $3::text)
    AND ($4::boolean IS FALSE OR case_type = $5::text);

-- name: CommerceAdminGetReconciliationCase :one
SELECT
    id,
    organization_id,
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
    organization_id = $1
    AND id = $2;

-- name: CommerceAdminResolveReconciliationCase :one
UPDATE commerce_reconciliation_cases
SET
    status = $3,
    resolved_at = now(),
    resolved_by = $4,
    resolution_note = $5
WHERE
    organization_id = $1
    AND id = $2
    AND status IN ('open', 'reviewing', 'escalated')
RETURNING *;
