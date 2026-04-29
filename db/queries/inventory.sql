-- Admin inventory reconcile marker (append-only ledger row).
-- name: AdminInventoryInsertReconcileMarker :one
INSERT INTO inventory_events (
    organization_id,
    machine_id,
    event_type,
    reason_code,
    quantity_delta,
    occurred_at,
    recorded_at,
    metadata
)
VALUES (
    $1,
    $2,
    'reconcile',
    $3,
    0,
    now (),
    now (),
    sqlc.arg('metadata')::jsonb
)
RETURNING id;
