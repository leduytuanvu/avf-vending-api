-- Financial correctness: winner arbitration, cash events, money view.

-- name: ClaimWinningPayment :one
UPDATE orders
SET
    winning_payment_id = $1,
    winning_claimed_at = now(),
    status = CASE
        WHEN status IN ('created', 'quoted') THEN 'paid'
        ELSE status
    END,
    updated_at = now()
WHERE
    id = $2
    AND winning_payment_id IS NULL
RETURNING
    id,
    machine_id,
    status,
    currency,
    subtotal_minor,
    tax_minor,
    total_minor,
    idempotency_key,
    simulated,
    simulation_run_id,
    simulation_scenario,
    fake_bill,
    fake_board,
    simulation_metadata,
    winning_payment_id,
    winning_claimed_at,
    created_at,
    updated_at;

-- name: GetOrderWinningPaymentID :one
SELECT winning_payment_id
FROM orders
WHERE id = $1;

-- name: UpdatePaymentOutcome :one
UPDATE payments
SET
    outcome = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CancelPaymentByID :one
UPDATE payments
SET
    state = 'canceled',
    outcome = CASE
        WHEN outcome = 'winner' THEN outcome
        WHEN outcome = 'pending' THEN 'superseded'
        ELSE outcome
    END,
    updated_at = now()
WHERE
    id = $1
    AND state IN ('created', 'authorized')
RETURNING *;

-- name: GetLatestNonCapturedPaymentForOrder :one
SELECT *
FROM payments
WHERE
    order_id = $1
    AND state IN ('created', 'authorized')
ORDER BY created_at DESC
LIMIT 1;

-- name: ListPaymentsForOrder :many
SELECT *
FROM payments
WHERE order_id = $1
ORDER BY created_at ASC;

-- name: InsertCashAcceptanceEvent :one
INSERT INTO cash_acceptance_events (
    machine_id,
    order_id,
    device_event_id,
    denomination_minor,
    credit_source,
    currency,
    accepted_at,
    raw_metadata
) VALUES (
    $1,
    sqlc.narg('order_id')::uuid,
    $2,
    $3,
    $4,
    $5,
    $6,
    COALESCE(NULLIF(sqlc.narg('raw_metadata')::text, '')::jsonb, '{}'::jsonb)
)
ON CONFLICT (machine_id, device_event_id) DO UPDATE
SET device_event_id = EXCLUDED.device_event_id
RETURNING *;

-- name: InsertCashAllocation :one
INSERT INTO cash_allocations (
    order_id,
    payment_id,
    machine_id,
    amount_minor,
    pre_order_credit_minor,
    post_order_inserted_minor,
    consent_source,
    consented_at,
    currency,
    idempotency_key
) VALUES (
    $1,
    sqlc.narg('payment_id')::uuid,
    $2,
    $3,
    $4,
    $5,
    $6,
    sqlc.narg('consented_at')::timestamptz,
    $7,
    $8
)
ON CONFLICT (order_id, idempotency_key) DO UPDATE
SET idempotency_key = EXCLUDED.idempotency_key
RETURNING *;

-- name: InsertCashChangeEvent :one
INSERT INTO cash_change_events (
    order_id,
    payment_id,
    machine_id,
    change_due_minor,
    change_dispensed_minor,
    outcome,
    liability_minor,
    currency,
    idempotency_key
) VALUES (
    $1,
    sqlc.narg('payment_id')::uuid,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
ON CONFLICT (order_id, idempotency_key) DO UPDATE
SET idempotency_key = EXCLUDED.idempotency_key
RETURNING *;

-- name: GetCashAllocationForOrder :one
SELECT *
FROM cash_allocations
WHERE order_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetCashChangeEventForOrder :one
SELECT *
FROM cash_change_events
WHERE order_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListCashAcceptanceEventsForOrder :many
SELECT *
FROM cash_acceptance_events
WHERE order_id = $1
ORDER BY accepted_at ASC;

-- name: ListCapturedPaymentsWithoutWinner :many
SELECT p.*
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
WHERE
    p.state = 'captured'
    AND o.winning_payment_id IS NULL
    AND o.status NOT IN ('cancelled', 'failed')
    AND p.created_at < $1
ORDER BY p.created_at ASC
LIMIT $2;

-- name: ListOrdersWithCashGrossMismatch :many
SELECT o.id AS order_id
FROM orders o
LEFT JOIN cash_allocations ca ON ca.order_id = o.id
LEFT JOIN cash_change_events cc ON cc.order_id = o.id
WHERE
    o.winning_payment_id IS NOT NULL
    AND ca.id IS NOT NULL
    AND (
        ca.amount_minor + COALESCE(cc.change_dispensed_minor, 0) + COALESCE(cc.liability_minor, 0)
        < ca.pre_order_credit_minor + ca.post_order_inserted_minor
    )
    AND o.updated_at < $1
LIMIT $2;

-- name: ListUnresolvedChangeLiability :many
SELECT cc.*
FROM cash_change_events cc
INNER JOIN orders o ON o.id = cc.order_id
WHERE
    cc.liability_minor > 0
    AND cc.outcome IN ('not_delivered', 'ambiguous')
    AND o.status NOT IN ('cancelled', 'failed')
    AND cc.created_at < $1
LIMIT $2;
