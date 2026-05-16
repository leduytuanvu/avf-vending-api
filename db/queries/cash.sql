-- name: InsertCashCollection :one
INSERT INTO cash_collections (
    machine_id,
    collected_at,
    opened_at,
    closed_at,
    lifecycle_status,
    amount_minor,
    expected_amount_minor,
    variance_amount_minor,
    requires_review,
    close_request_hash,
    currency,
    metadata,
    operator_session_id
)
VALUES (
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
    $11,
    $12,
    $13
)
RETURNING
    id,
    machine_id,
    collected_at,
    opened_at,
    closed_at,
    lifecycle_status,
    amount_minor,
    expected_amount_minor,
    variance_amount_minor,
    requires_review,
    close_request_hash,
    currency,
    metadata,
    reconciliation_status,
    reconciled_by,
    reconciled_at,
    created_at,
    operator_session_id;

-- name: GetCashCollectionByIDForOrgMachine :one
SELECT
    id,
    machine_id,
    collected_at,
    opened_at,
    closed_at,
    lifecycle_status,
    amount_minor,
    expected_amount_minor,
    variance_amount_minor,
    requires_review,
    close_request_hash,
    currency,
    metadata,
    reconciliation_status,
    reconciled_by,
    reconciled_at,
    created_at,
    operator_session_id
FROM cash_collections
WHERE
    id = $1
    AND TRUE
    AND machine_id = $2;

-- name: GetOpenCashCollectionByMachine :one
SELECT
    id,
    machine_id,
    collected_at,
    opened_at,
    closed_at,
    lifecycle_status,
    amount_minor,
    expected_amount_minor,
    variance_amount_minor,
    requires_review,
    close_request_hash,
    currency,
    metadata,
    reconciliation_status,
    reconciled_by,
    reconciled_at,
    created_at,
    operator_session_id
FROM cash_collections
WHERE
    machine_id = $1
    AND TRUE
    AND lifecycle_status = 'open';

-- name: FindCashCollectionOpenByStartIdempotencyKey :one
SELECT
    id,
    machine_id,
    collected_at,
    opened_at,
    closed_at,
    lifecycle_status,
    amount_minor,
    expected_amount_minor,
    variance_amount_minor,
    requires_review,
    close_request_hash,
    currency,
    metadata,
    reconciliation_status,
    reconciled_by,
    reconciled_at,
    created_at,
    operator_session_id
FROM cash_collections
WHERE
    metadata ->> 'start_idempotency_key' = $1::text
    AND machine_id = $2
    AND TRUE
    AND lifecycle_status = 'open';

-- name: CashSettlementNetExpectedMinor :one
WITH lc AS (
    SELECT
        max(closed_at) AS t
    FROM cash_collections
    WHERE
        machine_id = $1
        AND TRUE
        AND lifecycle_status = 'closed'
        AND closed_at IS NOT NULL
)
SELECT
    (
        COALESCE(
            (
                SELECT
                    sum(p.amount_minor)
                FROM payments p
                INNER JOIN orders o ON o.id = p.order_id
                WHERE
                    o.machine_id = $1
                    AND TRUE
                    AND p.provider = 'cash'
                    AND p.state = 'captured'
                    AND p.currency = $2
                    AND p.updated_at > COALESCE((SELECT t FROM lc), '-infinity'::timestamptz)
            ),
            0::bigint
        ) - COALESCE(
            (
                SELECT
                    sum(r.amount_minor)
                FROM refunds r
                INNER JOIN orders o ON o.id = r.order_id
                WHERE
                    o.machine_id = $1
                    AND TRUE
                    AND o.currency = $2
                    AND r.state = 'completed'
                    AND r.created_at > COALESCE((SELECT t FROM lc), '-infinity'::timestamptz)
            ),
            0::bigint
        )
    )::bigint AS net_minor;

-- name: CashSettlementLastClosedAt :one
SELECT
    coalesce(max(closed_at), 'epoch'::timestamptz)::timestamptz AS last_closed_at
FROM cash_collections
WHERE
    machine_id = $1
    AND TRUE
    AND lifecycle_status = 'closed';

-- name: CloseCashCollection :one
UPDATE cash_collections
SET
    lifecycle_status = 'closed',
    closed_at = now(),
    amount_minor = $1,
    expected_amount_minor = $2,
    variance_amount_minor = $3,
    requires_review = $4,
    close_request_hash = $5,
    reconciliation_status = $6,
    metadata = $7
WHERE
    id = $8
    AND TRUE
    AND machine_id = $9
    AND lifecycle_status = 'open'
RETURNING
    id,
    machine_id,
    collected_at,
    opened_at,
    closed_at,
    lifecycle_status,
    amount_minor,
    expected_amount_minor,
    variance_amount_minor,
    requires_review,
    close_request_hash,
    currency,
    metadata,
    reconciliation_status,
    reconciled_by,
    reconciled_at,
    created_at,
    operator_session_id;

-- name: ListCashCollectionsForMachine :many
SELECT
    id,
    machine_id,
    collected_at,
    opened_at,
    closed_at,
    lifecycle_status,
    amount_minor,
    expected_amount_minor,
    variance_amount_minor,
    requires_review,
    close_request_hash,
    currency,
    metadata,
    reconciliation_status,
    reconciled_by,
    reconciled_at,
    created_at,
    operator_session_id
FROM cash_collections
WHERE
    machine_id = $1
    AND TRUE
ORDER BY collected_at DESC
LIMIT $2 OFFSET $3;

-- name: InsertCashReconciliation :one
INSERT INTO cash_reconciliations (
    machine_id,
    cash_collection_id,
    expected_amount_minor,
    counted_amount_minor,
    variance_amount_minor,
    reconciled_at,
    status,
    metadata
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    now(),
    $6,
    $7
)
RETURNING
    id,
    machine_id,
    cash_session_id,
    cash_collection_id,
    expected_amount_minor,
    counted_amount_minor,
    variance_amount_minor,
    reconciled_at,
    status,
    metadata;
