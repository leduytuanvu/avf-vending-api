-- name: InsertCheckoutQuote :one
INSERT INTO checkout_quotes (
    machine_id,
    currency,
    payment_method,
    subtotal_minor,
    discount_minor,
    payable_minor,
    state,
    idempotency_key,
    expires_at
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
    $9
)
RETURNING *;

-- name: GetCheckoutQuoteByID :one
SELECT
    id,
    machine_id,
    currency,
    payment_method,
    subtotal_minor,
    discount_minor,
    payable_minor,
    state,
    idempotency_key,
    expires_at,
    created_at
FROM checkout_quotes
WHERE
    id = $1;

-- name: GetCheckoutQuoteByMachineAndIdempotencyKey :one
SELECT
    id,
    machine_id,
    currency,
    payment_method,
    subtotal_minor,
    discount_minor,
    payable_minor,
    state,
    idempotency_key,
    expires_at,
    created_at
FROM checkout_quotes
WHERE
    machine_id = $1
    AND idempotency_key = $2;

-- name: MarkCheckoutQuoteConsumed :one
UPDATE checkout_quotes
SET
    state = 'consumed'
WHERE
    id = $1
    AND state = 'active'
RETURNING
    id,
    machine_id,
    currency,
    payment_method,
    subtotal_minor,
    discount_minor,
    payable_minor,
    state,
    idempotency_key,
    expires_at,
    created_at;

-- name: InsertCheckoutQuoteLine :one
INSERT INTO checkout_quote_lines (
    quote_id,
    line_sequence,
    product_id,
    slot_config_id,
    cabinet_code,
    slot_code,
    slot_index,
    quantity,
    unit_price_minor,
    line_subtotal_minor,
    pricing_fingerprint
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
    $11
)
RETURNING *;

-- name: ListCheckoutQuoteLines :many
SELECT
    id,
    quote_id,
    line_sequence,
    product_id,
    slot_config_id,
    cabinet_code,
    slot_code,
    slot_index,
    quantity,
    unit_price_minor,
    line_subtotal_minor,
    pricing_fingerprint,
    created_at
FROM checkout_quote_lines
WHERE
    quote_id = $1
ORDER BY
    line_sequence ASC;

-- name: InsertVendSessionWithLineSequence :one
INSERT INTO vend_sessions (
    order_id,
    machine_id,
    slot_index,
    product_id,
    state,
    simulated,
    simulation_run_id,
    simulation_scenario,
    line_sequence
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
    $9
)
RETURNING
    id,
    order_id,
    machine_id,
    slot_index,
    product_id,
    state,
    failure_reason,
    correlation_id,
    started_at,
    completed_at,
    final_command_attempt_id,
    simulated,
    simulation_run_id,
    simulation_scenario,
    simulation_metadata,
    created_at,
    line_sequence;

-- name: GetVendSessionByOrderAndLineSequence :one
SELECT
    id,
    order_id,
    machine_id,
    slot_index,
    product_id,
    state,
    failure_reason,
    correlation_id,
    started_at,
    completed_at,
    final_command_attempt_id,
    simulated,
    simulation_run_id,
    simulation_scenario,
    simulation_metadata,
    created_at,
    line_sequence
FROM vend_sessions
WHERE
    order_id = $1
    AND line_sequence = $2;

-- name: LockVendSessionByOrderAndLineSequenceForUpdate :one
SELECT
    id,
    order_id,
    machine_id,
    slot_index,
    product_id,
    state,
    failure_reason,
    correlation_id,
    started_at,
    completed_at,
    final_command_attempt_id,
    simulated,
    simulation_run_id,
    simulation_scenario,
    simulation_metadata,
    created_at,
    line_sequence
FROM vend_sessions
WHERE
    order_id = $1
    AND line_sequence = $2
FOR UPDATE;

-- name: UpdateVendSessionStateByOrderLineSequence :one
UPDATE vend_sessions
SET
    state = $1,
    failure_reason = $2,
    completed_at = CASE
        WHEN $1 IN ('success', 'failed') THEN now()
        ELSE completed_at
    END,
    started_at = CASE
        WHEN $1 = 'in_progress'
        AND started_at IS NULL THEN now()
        ELSE started_at
    END
WHERE
    order_id = $3
    AND line_sequence = $4
RETURNING
    id,
    order_id,
    machine_id,
    slot_index,
    product_id,
    state,
    failure_reason,
    correlation_id,
    started_at,
    completed_at,
    final_command_attempt_id,
    simulated,
    simulation_run_id,
    simulation_scenario,
    simulation_metadata,
    created_at,
    line_sequence;

-- name: ListVendSessionsByOrder :many
SELECT
    id,
    order_id,
    machine_id,
    slot_index,
    product_id,
    state,
    failure_reason,
    correlation_id,
    started_at,
    completed_at,
    final_command_attempt_id,
    simulated,
    simulation_run_id,
    simulation_scenario,
    simulation_metadata,
    created_at,
    line_sequence
FROM vend_sessions
WHERE
    order_id = $1
ORDER BY
    line_sequence ASC;
