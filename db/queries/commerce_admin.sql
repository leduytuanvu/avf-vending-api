-- name: CommerceAdminListOrders :many
SELECT
    o.id,
    o.machine_id,
    m.code AS machine_code,
    o.status,
    o.currency,
    o.subtotal_minor,
    o.tax_minor,
    o.total_minor,
    o.idempotency_key,
    o.created_at,
    o.updated_at,
    COALESCE(
        wp.provider,
        (
            SELECT p.provider
            FROM payments p
            WHERE p.order_id = o.id
            ORDER BY p.created_at DESC
            LIMIT 1
        ),
        ''
    ) AS payment_provider
FROM orders o
INNER JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments wp ON wp.id = o.winning_payment_id
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

-- name: CommerceAdminGetOrderDetail :one
SELECT
    o.id,
    o.machine_id,
    m.code AS machine_code,
    o.status,
    o.currency,
    o.subtotal_minor,
    o.tax_minor,
    o.total_minor,
    o.idempotency_key,
    o.created_at,
    o.updated_at,
    COALESCE(
        wp.provider,
        (
            SELECT p.provider
            FROM payments p
            WHERE p.order_id = o.id
            ORDER BY p.created_at DESC
            LIMIT 1
        ),
        ''
    ) AS payment_provider,
    COALESCE(
        wp.state,
        (
            SELECT p.state
            FROM payments p
            WHERE p.order_id = o.id
            ORDER BY p.created_at DESC
            LIMIT 1
        ),
        ''
    ) AS payment_state,
    o.machine_pricing_snapshot
FROM orders o
INNER JOIN machines m ON m.id = o.machine_id
LEFT JOIN payments wp ON wp.id = o.winning_payment_id
WHERE o.id = $1;

-- name: CommerceAdminListOrderLineItems :many
SELECT
    vs.id AS vend_session_id,
    vs.line_sequence,
    vs.slot_index,
    vs.product_id,
    pr.name AS product_name,
    vs.state AS vend_state,
    vs.failure_reason,
    COALESCE(cql_match.cabinet_code, '') AS cabinet_code,
    COALESCE(cql_match.slot_code, '') AS slot_code,
    1 AS quantity,
    COALESCE(cql_match.unit_price_minor, 0) AS unit_price_minor,
    COALESCE(cql_match.line_subtotal_minor, 0) AS line_subtotal_minor
FROM vend_sessions vs
INNER JOIN products pr ON pr.id = vs.product_id
LEFT JOIN orders o ON o.id = vs.order_id
LEFT JOIN LATERAL (
    SELECT
        cql.cabinet_code,
        cql.slot_code,
        cql.unit_price_minor,
        (
            CASE
                WHEN cql.quantity > 1 THEN cql.unit_price_minor
                ELSE cql.line_subtotal_minor
            END
        )::bigint AS line_subtotal_minor
    FROM checkout_quotes cq
    INNER JOIN checkout_quote_lines cql ON cql.quote_id = cq.id
    WHERE cq.machine_id = o.machine_id
      AND (
          (
              cq.idempotency_key IS NOT NULL
              AND o.idempotency_key IS NOT NULL
              AND cq.idempotency_key = o.idempotency_key
          )
          OR (
              cq.state = 'consumed'
              AND cq.created_at <= o.created_at + INTERVAL '5 seconds'
              AND cq.created_at >= o.created_at - INTERVAL '30 minutes'
          )
      )
      AND (
          (cql.product_id = vs.product_id AND cql.slot_index = vs.slot_index)
          OR cql.line_sequence = vs.line_sequence
      )
    ORDER BY
        CASE
            WHEN cq.idempotency_key IS NOT NULL
                AND o.idempotency_key IS NOT NULL
                AND cq.idempotency_key = o.idempotency_key
            THEN 0
            ELSE 1
        END,
        cq.created_at DESC
    LIMIT 1
) cql_match ON TRUE
WHERE vs.order_id = $1
ORDER BY vs.line_sequence ASC;

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
