-- Financial ledger append-only entries for zero-loss reconciliation (GAP 7).

-- name: InsertFinancialLedgerEntry :one
INSERT INTO financial_ledger_entries (
    machine_id,
    site_id,
    order_id,
    payment_id,
    refund_id,
    entry_type,
    signed_amount_minor,
    currency,
    occurred_at,
    reference_type,
    reference_id,
    correlation_id,
    metadata
) VALUES (
    sqlc.narg('machine_id')::uuid,
    sqlc.narg('site_id')::uuid,
    sqlc.narg('order_id')::uuid,
    sqlc.narg('payment_id')::uuid,
    sqlc.narg('refund_id')::uuid,
    $1,
    $2,
    $3,
    $4,
    sqlc.narg('reference_type')::text,
    sqlc.narg('reference_id')::uuid,
    sqlc.narg('correlation_id')::uuid,
    COALESCE(NULLIF(sqlc.narg('metadata')::text, '')::jsonb, '{}'::jsonb)
)
RETURNING id, entry_type, signed_amount_minor, occurred_at;
