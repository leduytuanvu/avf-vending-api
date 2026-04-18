-- name: InsertCashCollection :one
INSERT INTO cash_collections (
    organization_id,
    machine_id,
    collected_at,
    amount_minor,
    currency,
    metadata,
    operator_session_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    id,
    organization_id,
    machine_id,
    collected_at,
    amount_minor,
    currency,
    metadata,
    reconciliation_status,
    reconciled_by,
    reconciled_at,
    created_at,
    operator_session_id;
