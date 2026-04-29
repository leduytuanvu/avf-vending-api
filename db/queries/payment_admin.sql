-- Admin/finance: PSP webhooks, settlements, disputes, exports (org-scoped).

-- name: ListPaymentProviderEventsForOrgAdmin :many
SELECT
    e.id,
    e.payment_id,
    e.organization_id,
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
WHERE (e.organization_id = sqlc.arg('organization_id')::uuid OR o.organization_id = sqlc.arg('organization_id')::uuid)
ORDER BY e.received_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPaymentProviderEventsForOrgAdmin :one
SELECT
    count(*)::bigint
FROM payment_provider_events e
LEFT JOIN payments p ON p.id = e.payment_id
LEFT JOIN orders o ON o.id = p.order_id
WHERE (e.organization_id = sqlc.arg('organization_id')::uuid OR o.organization_id = sqlc.arg('organization_id')::uuid);

-- name: ListPaymentProviderSettlementsForOrg :many
SELECT
    id,
    organization_id,
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
WHERE organization_id = $1
ORDER BY settlement_date DESC,
    created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPaymentProviderSettlementsForOrg :one
SELECT count(*)::bigint
FROM payment_provider_settlements
WHERE organization_id = $1;

-- name: UpsertPaymentProviderSettlement :one
INSERT INTO payment_provider_settlements (
    organization_id,
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
    $10,
    $11
)
ON CONFLICT (organization_id, provider, provider_settlement_id) DO UPDATE SET
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
    organization_id,
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
    status = $3,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING
    id,
    organization_id,
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
        o.organization_id = $1
        AND lower(p.provider) = lower($2)
        AND pa.provider_reference = ANY ($3::text[])
    ORDER BY
        p.id
) AS sub;

-- name: ListPaymentDisputesForOrg :many
SELECT
    id,
    organization_id,
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
WHERE organization_id = $1
ORDER BY opened_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPaymentDisputesForOrg :one
SELECT count(*)::bigint
FROM payment_disputes
WHERE organization_id = $1;

-- name: GetPaymentDisputeForOrg :one
SELECT
    id,
    organization_id,
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
    AND organization_id = $2;

-- name: ResolvePaymentDisputeForOrg :one
UPDATE payment_disputes
SET
    status = $3,
    resolution_note = $4,
    resolved_at = now(),
    resolved_by = $5,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
    AND status NOT IN ('won', 'lost', 'closed')
RETURNING
    id,
    organization_id,
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
    organization_id,
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
    $3,
    sqlc.narg('payment_id')::uuid,
    sqlc.narg('order_id')::uuid,
    $4,
    $5,
    sqlc.narg('reason')::text,
    sqlc.narg('status')::text,
    $6
)
RETURNING
    id,
    organization_id,
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
    o.machine_id,
    o.organization_id
FROM payments p
JOIN orders o ON o.id = p.order_id
WHERE
    o.organization_id = $1
    AND p.created_at >= $2
    AND p.created_at <= $3
ORDER BY p.created_at ASC;
