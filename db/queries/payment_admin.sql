-- Admin/finance: PSP webhooks, settlements, disputes, exports (role-scoped).

-- name: ListPaymentProviderEventsForOrgAdmin :many
SELECT
    e.id,
    e.payment_id,
    e.provider,
    e.provider_ref,
    e.webhook_event_id,
    e.provider_amount_minor,
    e.currency,
    e.event_type,
    e.payload,
    e.received_at,
    e.validation_status,
    e.provider_metadata,
    e.legal_hold,
    e.signature_valid,
    e.applied_at,
    e.ingress_status,
    e.ingress_error
FROM payment_provider_events e
LEFT JOIN payments p ON p.id = e.payment_id
LEFT JOIN orders o ON o.id = p.order_id
WHERE TRUE
ORDER BY e.received_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPaymentProviderEventsForOrgAdmin :one
SELECT
    count(*)::bigint
FROM payment_provider_events e
LEFT JOIN payments p ON p.id = e.payment_id
LEFT JOIN orders o ON o.id = p.order_id
WHERE TRUE;

-- name: ListPaymentProviderSettlementsForOrg :many
SELECT
    id,
    provider,
    provider_settlement_id,
    gross_amount_minor,
    fee_amount_minor,
    net_amount_minor,
    currency,
    settlement_date,
    transaction_refs,
    status,
    metadata,
    created_at,
    updated_at
FROM payment_provider_settlements
WHERE TRUE
ORDER BY settlement_date DESC,
    created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPaymentProviderSettlementsForOrg :one
SELECT count(*)::bigint
FROM payment_provider_settlements
WHERE TRUE;

-- name: UpsertPaymentProviderSettlement :one
INSERT INTO payment_provider_settlements (
    provider,
    provider_settlement_id,
    gross_amount_minor,
    fee_amount_minor,
    net_amount_minor,
    currency,
    settlement_date,
    transaction_refs,
    status,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10
)
ON CONFLICT (provider, provider_settlement_id) DO UPDATE SET
    gross_amount_minor = EXCLUDED.gross_amount_minor,
    fee_amount_minor = EXCLUDED.fee_amount_minor,
    net_amount_minor = EXCLUDED.net_amount_minor,
    currency = EXCLUDED.currency,
    settlement_date = EXCLUDED.settlement_date,
    transaction_refs = EXCLUDED.transaction_refs,
    metadata = payment_provider_settlements.metadata || EXCLUDED.metadata,
    updated_at = now()
RETURNING
    id,
    provider,
    provider_settlement_id,
    gross_amount_minor,
    fee_amount_minor,
    net_amount_minor,
    currency,
    settlement_date,
    transaction_refs,
    status,
    metadata,
    created_at,
    updated_at;

-- name: UpdatePaymentProviderSettlementStatusForOrg :one
UPDATE payment_provider_settlements
SET
    status = $1,
    updated_at = now()
WHERE
    id = $2
    AND TRUE
RETURNING
    id,
    provider,
    provider_settlement_id,
    gross_amount_minor,
    fee_amount_minor,
    net_amount_minor,
    currency,
    settlement_date,
    transaction_refs,
    status,
    metadata,
    created_at,
    updated_at;

-- name: SettlementReferencedPaymentsTotalForOrg :one
SELECT
    COALESCE(sum(sub.amount_minor), 0)::bigint AS referenced_total_minor,
    count(*)::bigint AS referenced_payment_count
FROM (
    SELECT DISTINCT ON (p.id)
        p.id,
        p.amount_minor
    FROM payments p
    INNER JOIN orders o ON o.id = p.order_id
    INNER JOIN payment_attempts pa ON pa.payment_id = p.id
    WHERE
        lower(p.provider) = lower($1)
        AND pa.provider_reference = ANY ($2::text[])
    ORDER BY
        p.id
) AS sub;

-- name: ListPaymentDisputesForOrg :many
SELECT
    id,
    provider,
    provider_dispute_id,
    payment_id,
    order_id,
    amount_minor,
    currency,
    reason,
    status,
    opened_at,
    resolved_at,
    resolved_by,
    resolution_note,
    metadata,
    created_at,
    updated_at
FROM payment_disputes
WHERE TRUE
ORDER BY opened_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPaymentDisputesForOrg :one
SELECT count(*)::bigint
FROM payment_disputes
WHERE TRUE;

-- name: GetPaymentDisputeForOrg :one
SELECT
    id,
    provider,
    provider_dispute_id,
    payment_id,
    order_id,
    amount_minor,
    currency,
    reason,
    status,
    opened_at,
    resolved_at,
    resolved_by,
    resolution_note,
    metadata,
    created_at,
    updated_at
FROM payment_disputes
WHERE id = $1
    AND TRUE;

-- name: ResolvePaymentDisputeForOrg :one
UPDATE payment_disputes
SET
    status = $1,
    resolution_note = $2,
    resolved_at = now(),
    resolved_by = $3,
    updated_at = now()
WHERE
    id = $4
    AND TRUE
    AND status NOT IN ('won', 'lost', 'closed')
RETURNING
    id,
    provider,
    provider_dispute_id,
    payment_id,
    order_id,
    amount_minor,
    currency,
    reason,
    status,
    opened_at,
    resolved_at,
    resolved_by,
    resolution_note,
    metadata,
    created_at,
    updated_at;

-- name: InsertPaymentDispute :one
INSERT INTO payment_disputes (
    provider,
    provider_dispute_id,
    payment_id,
    order_id,
    amount_minor,
    currency,
    reason,
    status,
    metadata
) VALUES (
    $1,
    $2,
    sqlc.narg('payment_id')::uuid,
    sqlc.narg('order_id')::uuid,
    $3,
    $4,
    sqlc.narg('reason')::text,
    sqlc.narg('status')::text,
    $5
)
RETURNING
    id,
    provider,
    provider_dispute_id,
    payment_id,
    order_id,
    amount_minor,
    currency,
    reason,
    status,
    opened_at,
    resolved_at,
    resolved_by,
    resolution_note,
    metadata,
    created_at,
    updated_at;

-- name: ListPaymentsFinanceExportForOrg :many
SELECT
    p.id,
    p.order_id,
    p.provider,
    p.state,
    p.amount_minor,
    p.currency,
    p.reconciliation_status,
    p.settlement_status,
    p.created_at,
    p.updated_at,
    o.machine_id
FROM payments p
JOIN orders o ON o.id = p.order_id
WHERE
    p.created_at >= $1
    AND p.created_at <= $2
ORDER BY p.created_at ASC;
