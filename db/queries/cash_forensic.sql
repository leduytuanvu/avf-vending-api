-- Cash forensic ledger: ingest and read model queries.

-- name: InsertCashPayoutEvent :one
INSERT INTO cash_payout_events (
    machine_id,
    order_id,
    withdrawal_id,
    note_sequence,
    event_type,
    device_event_id,
    denomination_minor,
    amount_minor,
    recycler_count_before,
    recycler_count_after,
    outcome_finality,
    currency,
    occurred_at_device,
    raw_metadata
) VALUES (
    $1,
    sqlc.narg('order_id')::uuid,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    sqlc.narg('recycler_count_before')::int,
    sqlc.narg('recycler_count_after')::int,
    $8,
    $9,
    $10,
    COALESCE(NULLIF(sqlc.narg('raw_metadata')::text, '')::jsonb, '{}'::jsonb)
)
ON CONFLICT (machine_id, device_event_id) DO UPDATE
SET device_event_id = EXCLUDED.device_event_id
RETURNING *;

-- name: InsertCashBillLifecycleEvent :one
INSERT INTO cash_bill_lifecycle_events (
    machine_id,
    order_id,
    device_event_id,
    lifecycle_type,
    denomination_minor,
    raw_record_hex,
    currency,
    occurred_at_device,
    raw_metadata
) VALUES (
    $1,
    sqlc.narg('order_id')::uuid,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    COALESCE(NULLIF(sqlc.narg('raw_metadata')::text, '')::jsonb, '{}'::jsonb)
)
ON CONFLICT (machine_id, device_event_id) DO UPDATE
SET device_event_id = EXCLUDED.device_event_id
RETURNING *;

-- name: InsertCashHardwareObservation :one
INSERT INTO cash_hardware_observations (
    machine_id,
    device_event_id,
    observed_at_device,
    recycler_denomination_minor,
    recycler_count,
    cashbox_count,
    source,
    currency,
    raw_metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    sqlc.narg('cashbox_count')::int,
    $6,
    $7,
    COALESCE(NULLIF(sqlc.narg('raw_metadata')::text, '')::jsonb, '{}'::jsonb)
)
ON CONFLICT (machine_id, device_event_id) DO UPDATE
SET device_event_id = EXCLUDED.device_event_id
RETURNING *;

-- name: InsertCashAdjustment :one
INSERT INTO cash_adjustments (
    machine_id,
    amount_minor,
    bucket,
    reason,
    operator_account_id,
    idempotency_key,
    currency,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    sqlc.narg('operator_account_id')::uuid,
    $5,
    $6,
    COALESCE(NULLIF(sqlc.narg('metadata')::text, '')::jsonb, '{}'::jsonb)
)
ON CONFLICT (idempotency_key) DO UPDATE
SET idempotency_key = EXCLUDED.idempotency_key
RETURNING *;

-- name: UpdateCashAcceptanceEventOrder :exec
UPDATE cash_acceptance_events
SET order_id = $3
WHERE
    machine_id = $1
    AND device_event_id = $2
    AND (order_id IS NULL OR order_id = $3);

-- name: GetLatestCashHardwareObservation :one
SELECT *
FROM cash_hardware_observations
WHERE
    machine_id = $1
    AND currency = $2
ORDER BY observed_at_device DESC, id DESC
LIMIT 1;

-- name: SumCashAcceptanceByMachineSince :one
SELECT
    coalesce(
        sum(
            CASE
                WHEN credit_source IN ('stacked_cashbox', 'unknown') THEN denomination_minor
                ELSE 0
            END
        ),
        0::bigint
    )::bigint AS cashbox_minor,
    coalesce(
        sum(
            CASE
                WHEN credit_source = 'stored_in_recycler' THEN denomination_minor
                ELSE 0
            END
        ),
        0::bigint
    )::bigint AS recycler_minor
FROM cash_acceptance_events
WHERE
    machine_id = $1
    AND currency = $2
    AND accepted_at > $3;

-- name: SumCashPayoutConfirmedByMachineSince :one
SELECT coalesce(sum(amount_minor), 0::bigint)::bigint AS total_minor
FROM cash_payout_events
WHERE
    machine_id = $1
    AND currency = $2
    AND occurred_at_device > $3
    AND outcome_finality = 'confirmed';

-- name: SumCashAdjustmentsByMachineSince :one
SELECT
    coalesce(
        sum(CASE WHEN bucket = 'cashbox' THEN amount_minor ELSE 0 END),
        0::bigint
    )::bigint AS cashbox_minor,
    coalesce(
        sum(CASE WHEN bucket = 'recycler' THEN amount_minor ELSE 0 END),
        0::bigint
    )::bigint AS recycler_minor,
    coalesce(
        sum(CASE WHEN bucket = 'unallocated_wallet' THEN amount_minor ELSE 0 END),
        0::bigint
    )::bigint AS wallet_minor
FROM cash_adjustments
WHERE
    machine_id = $1
    AND currency = $2
    AND created_at > $3;

-- name: SumCollectedCashByMachineSince :one
SELECT coalesce(sum(amount_minor), 0::bigint)::bigint AS total_minor
FROM cash_collections
WHERE
    machine_id = $1
    AND currency = $2
    AND lifecycle_status = 'closed'
    AND closed_at > $3;

-- name: ListCashLedgerAcceptanceEvents :many
SELECT
    id,
    machine_id,
    order_id,
    device_event_id,
    denomination_minor,
    credit_source,
    currency,
    accepted_at,
    created_at,
    'acceptance'::text AS movement_class,
    CASE
        WHEN credit_source = 'stored_in_recycler' THEN 'recycler'
        ELSE 'cashbox'
    END AS destination
FROM cash_acceptance_events
WHERE
    machine_id = sqlc.narg('machine_id')::uuid
    AND accepted_at >= sqlc.narg('from_time')::timestamptz
    AND accepted_at < sqlc.narg('to_time')::timestamptz
    AND (
        sqlc.narg('order_id')::uuid IS NULL
        OR order_id = sqlc.narg('order_id')::uuid
    )
    AND (
        sqlc.narg('after_time')::timestamptz IS NULL
        OR accepted_at < sqlc.narg('after_time')::timestamptz
        OR (
            accepted_at = sqlc.narg('after_time')::timestamptz
            AND id::text < sqlc.narg('after_id')::text
        )
    )
ORDER BY accepted_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: ListCashLedgerPayoutEvents :many
SELECT
    id,
    machine_id,
    order_id,
    withdrawal_id,
    note_sequence,
    event_type,
    device_event_id,
    denomination_minor,
    amount_minor,
    recycler_count_before,
    recycler_count_after,
    outcome_finality,
    currency,
    occurred_at_device,
    created_at,
    'payout'::text AS movement_class,
    'customer'::text AS destination
FROM cash_payout_events
WHERE
    machine_id = sqlc.narg('machine_id')::uuid
    AND occurred_at_device >= sqlc.narg('from_time')::timestamptz
    AND occurred_at_device < sqlc.narg('to_time')::timestamptz
    AND (
        sqlc.narg('order_id')::uuid IS NULL
        OR order_id = sqlc.narg('order_id')::uuid
    )
    AND (
        sqlc.narg('after_time')::timestamptz IS NULL
        OR occurred_at_device < sqlc.narg('after_time')::timestamptz
        OR (
            occurred_at_device = sqlc.narg('after_time')::timestamptz
            AND id::text < sqlc.narg('after_id')::text
        )
    )
ORDER BY occurred_at_device DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: GetCashPayoutEventByID :one
SELECT *
FROM cash_payout_events
WHERE id = $1;

-- name: GetCashAcceptanceEventByID :one
SELECT *
FROM cash_acceptance_events
WHERE id = $1;

-- name: ListCashAcceptanceEventsForMachineSince :many
SELECT *
FROM cash_acceptance_events
WHERE
    machine_id = $1
    AND currency = $2
    AND accepted_at > $3
ORDER BY accepted_at ASC;

-- name: ListCashPayoutEventsForMachineSince :many
SELECT *
FROM cash_payout_events
WHERE
    machine_id = $1
    AND currency = $2
    AND occurred_at_device > $3
ORDER BY occurred_at_device ASC;

-- name: ListUnresolvedCashPayoutAmbiguous :many
SELECT *
FROM cash_payout_events
WHERE
    machine_id = $1
    AND outcome_finality = 'ambiguous'
    AND occurred_at_device < $2
ORDER BY occurred_at_device ASC
LIMIT $3;
