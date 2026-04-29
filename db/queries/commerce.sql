-- name: InsertOrder :one
INSERT INTO orders (
    organization_id,
    machine_id,
    status,
    currency,
    subtotal_minor,
    tax_minor,
    total_minor,
    idempotency_key
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: InsertVendSession :one
INSERT INTO vend_sessions (
    order_id,
    machine_id,
    slot_index,
    product_id,
    state
)
VALUES ($1, $2, $3, $4, $5)
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
    created_at;

-- name: InsertPayment :one
INSERT INTO payments (
    order_id,
    provider,
    state,
    amount_minor,
    currency,
    idempotency_key
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING
    id,
    order_id,
    provider,
    state,
    amount_minor,
    currency,
    idempotency_key,
    created_at,
    updated_at,
    reconciliation_status,
    settlement_status,
    settlement_batch_id;

-- name: GetPaymentByOrderAndIdempotencyKey :one
SELECT
    id,
    order_id,
    provider,
    state,
    amount_minor,
    currency,
    idempotency_key,
    created_at,
    updated_at,
    reconciliation_status,
    settlement_status,
    settlement_batch_id
FROM payments
WHERE
    order_id = $1
    AND idempotency_key = $2;

-- name: GetOrderByOrgIdempotency :one
SELECT
    id,
    organization_id,
    machine_id,
    status,
    currency,
    subtotal_minor,
    tax_minor,
    total_minor,
    idempotency_key,
    created_at,
    updated_at
FROM orders
WHERE
    organization_id = $1
    AND idempotency_key = $2;

-- name: GetOrderByID :one
SELECT
    id,
    organization_id,
    machine_id,
    status,
    currency,
    subtotal_minor,
    tax_minor,
    total_minor,
    idempotency_key,
    created_at,
    updated_at
FROM orders
WHERE
    id = $1;

-- name: LockOrderByIDAndOrgForUpdate :one
SELECT
    id,
    organization_id,
    machine_id,
    status,
    currency,
    subtotal_minor,
    tax_minor,
    total_minor,
    idempotency_key,
    created_at,
    updated_at
FROM orders
WHERE
    id = $1
    AND organization_id = $2
FOR UPDATE;

-- name: LockVendSessionByOrderAndSlotForUpdate :one
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
    created_at
FROM vend_sessions
WHERE
    order_id = $1
    AND slot_index = $2
FOR UPDATE;

-- name: GetVendSessionByOrderAndSlot :one
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
    created_at
FROM vend_sessions
WHERE
    order_id = $1
    AND slot_index = $2;

-- name: GetFirstVendSessionByOrder :one
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
    created_at
FROM vend_sessions
WHERE
    order_id = $1
ORDER BY
    created_at ASC
LIMIT 1;

-- name: ListPaymentsPendingTimeout :many
SELECT
    id,
    order_id,
    provider,
    state,
    amount_minor,
    currency,
    idempotency_key,
    created_at,
    updated_at,
    reconciliation_status,
    settlement_status,
    settlement_batch_id
FROM payments
WHERE
    state IN ('created', 'authorized')
    AND created_at < $1
ORDER BY
    created_at ASC
LIMIT $2;

-- name: ListOrdersWithUnresolvedPayment :many
SELECT DISTINCT
    ON (o.id) o.id,
    o.organization_id,
    o.machine_id,
    o.status,
    o.currency,
    o.subtotal_minor,
    o.tax_minor,
    o.total_minor,
    o.idempotency_key,
    o.created_at,
    o.updated_at
FROM orders o
INNER JOIN payments p ON p.order_id = o.id
WHERE
    p.state IN ('created', 'authorized')
    AND p.created_at < $1
ORDER BY
    o.id,
    o.updated_at ASC
LIMIT $2;

-- name: ListVendSessionsStuckForReconciliation :many
SELECT
    v.id,
    v.order_id,
    v.machine_id,
    v.slot_index,
    v.product_id,
    v.state,
    v.failure_reason,
    v.correlation_id,
    v.started_at,
    v.completed_at,
    v.final_command_attempt_id,
    v.created_at,
    o.organization_id,
    o.status AS order_status
FROM vend_sessions v
INNER JOIN orders o ON o.id = v.order_id
WHERE
    v.state IN ('pending', 'in_progress')
    AND v.created_at < $1
    AND o.status IN ('paid', 'vending', 'created')
ORDER BY
    v.created_at ASC
LIMIT $2;

-- name: ListPotentialDuplicatePayments :many
SELECT
    p.id,
    p.order_id,
    p.provider,
    p.state,
    p.amount_minor,
    p.currency,
    p.idempotency_key,
    p.created_at,
    p.updated_at,
    p.reconciliation_status,
    p.settlement_status,
    p.settlement_batch_id
FROM payments p
WHERE
    EXISTS (
        SELECT 1
        FROM payments p2
        WHERE
            p2.order_id = p.order_id
            AND p2.id <> p.id
            AND p2.amount_minor = p.amount_minor
            AND p2.currency = p.currency
    )
    AND p.created_at < $1
ORDER BY
    p.order_id,
    p.created_at ASC
LIMIT $2;

-- name: ListPaymentsForRefundReview :many
SELECT
    p.id,
    p.order_id,
    p.provider,
    p.state,
    p.amount_minor,
    p.currency,
    p.idempotency_key,
    p.created_at,
    p.updated_at,
    p.reconciliation_status,
    p.settlement_status,
    p.settlement_batch_id
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
WHERE
    p.state = 'captured'
    AND o.status = 'failed'
    AND p.created_at < $1
ORDER BY
    p.created_at ASC
LIMIT $2;

-- name: ListPaidOrdersWithoutVendStart :many
SELECT
    o.id AS order_id,
    o.organization_id,
    o.machine_id,
    p.id AS payment_id,
    p.provider,
    p.state AS payment_state,
    v.id AS vend_session_id,
    v.state AS vend_state,
    o.updated_at
FROM orders o
INNER JOIN payments p ON p.order_id = o.id
INNER JOIN vend_sessions v ON v.order_id = o.id
WHERE
    p.state IN ('captured', 'partially_refunded')
    AND o.status = 'paid'
    AND v.state = 'pending'
    AND o.updated_at < $1
ORDER BY
    o.updated_at ASC
LIMIT $2;

-- name: ListPaidVendFailuresForReview :many
SELECT
    o.id AS order_id,
    o.organization_id,
    o.machine_id,
    p.id AS payment_id,
    p.provider,
    p.state AS payment_state,
    v.id AS vend_session_id,
    v.state AS vend_state,
    v.completed_at
FROM payments p
INNER JOIN orders o ON o.id = p.order_id
INNER JOIN vend_sessions v ON v.order_id = o.id
WHERE
    p.state IN ('captured', 'partially_refunded')
    AND o.status = 'failed'
    AND v.state = 'failed'
    AND v.completed_at < $1
ORDER BY
    v.completed_at ASC
LIMIT $2;

-- name: ListRefundsPendingTooLong :many
SELECT
    r.id AS refund_id,
    r.payment_id,
    r.order_id,
    o.organization_id,
    p.provider,
    r.state AS refund_state,
    r.amount_minor,
    r.currency,
    r.created_at
FROM refunds r
INNER JOIN orders o ON o.id = r.order_id
INNER JOIN payments p ON p.id = r.payment_id
WHERE
    r.state IN ('requested', 'processing')
    AND r.created_at < $1
ORDER BY
    r.created_at ASC
LIMIT $2;

-- name: ListStaleCommandLedgerEntries :many
SELECT
    id,
    machine_id,
    sequence,
    command_type,
    payload,
    correlation_id,
    idempotency_key,
    created_at,
    protocol_type,
    deadline_at,
    timeout_at,
    attempt_count,
    last_attempt_at,
    route_key,
    source_system,
    source_event_id,
    operator_session_id
FROM command_ledger
WHERE
    created_at < $1
ORDER BY
    created_at ASC
LIMIT $2;

-- name: UpdateOrderStatusByOrg :one
UPDATE orders
SET
    status = $3,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
RETURNING
    id,
    organization_id,
    machine_id,
    status,
    currency,
    subtotal_minor,
    tax_minor,
    total_minor,
    idempotency_key,
    created_at,
    updated_at;

-- name: UpdateVendSessionStateByOrderSlot :one
UPDATE vend_sessions
SET
    state = $3,
    failure_reason = $4,
    completed_at = CASE
        WHEN $3 IN ('success', 'failed') THEN now()
        ELSE completed_at
    END,
    started_at = CASE
        WHEN $3 = 'in_progress'
        AND started_at IS NULL THEN now()
        ELSE started_at
    END
WHERE
    order_id = $1
    AND slot_index = $2
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
    created_at;

-- name: GetLatestPaymentForOrder :one
SELECT
    id,
    order_id,
    provider,
    state,
    amount_minor,
    currency,
    idempotency_key,
    created_at,
    updated_at,
    reconciliation_status,
    settlement_status,
    settlement_batch_id
FROM payments
WHERE
    order_id = $1
ORDER BY
    created_at DESC
LIMIT 1;

-- name: GetPaymentByID :one
SELECT
    id,
    order_id,
    provider,
    state,
    amount_minor,
    currency,
    idempotency_key,
    created_at,
    updated_at,
    reconciliation_status,
    settlement_status,
    settlement_batch_id
FROM payments
WHERE
    id = $1;

-- name: InsertPaymentAttempt :one
INSERT INTO payment_attempts (
    payment_id,
    provider_reference,
    state,
    payload
) VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING
    id,
    payment_id,
    provider_reference,
    state,
    payload,
    created_at;

-- name: UpdatePaymentStateForReconciliation :one
UPDATE payments
SET
    state = $2,
    updated_at = now()
WHERE
    id = $1
    AND state IN ('created', 'authorized')
RETURNING
    id,
    order_id,
    provider,
    state,
    amount_minor,
    currency,
    idempotency_key,
    created_at,
    updated_at,
    reconciliation_status,
    settlement_status,
    settlement_batch_id;

-- name: UpdatePaymentState :one
UPDATE payments
SET
    state = $2,
    updated_at = now()
WHERE
    id = $1
RETURNING
    id,
    order_id,
    provider,
    state,
    amount_minor,
    currency,
    idempotency_key,
    created_at,
    updated_at,
    reconciliation_status,
    settlement_status,
    settlement_batch_id;

-- name: GetPaymentProviderEventByProviderRef :one
SELECT
    id,
    payment_id,
    organization_id,
    provider,
    provider_ref,
    webhook_event_id,
    provider_amount_minor,
    currency,
    event_type,
    payload,
    received_at,
    validation_status,
    provider_metadata,
    legal_hold,
    signature_valid,
    applied_at,
    ingress_status,
    ingress_error
FROM payment_provider_events
WHERE
    provider = $1
    AND provider_ref = $2;

-- name: GetPaymentProviderEventByWebhookEventID :one
SELECT
    id,
    payment_id,
    organization_id,
    provider,
    provider_ref,
    webhook_event_id,
    provider_amount_minor,
    currency,
    event_type,
    payload,
    received_at,
    validation_status,
    provider_metadata,
    legal_hold,
    signature_valid,
    applied_at,
    ingress_status,
    ingress_error
FROM payment_provider_events
WHERE
    provider = $1
    AND webhook_event_id = $2;

-- name: InsertPaymentProviderEvent :one
INSERT INTO payment_provider_events (
    payment_id,
    organization_id,
    provider,
    provider_ref,
    webhook_event_id,
    provider_amount_minor,
    currency,
    event_type,
    payload,
    validation_status,
    provider_metadata,
    signature_valid,
    applied_at,
    ingress_status,
    ingress_error
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
    $11,
    $12,
    $13,
    $14,
    $15
)
RETURNING
    id,
    payment_id,
    organization_id,
    provider,
    provider_ref,
    webhook_event_id,
    provider_amount_minor,
    currency,
    event_type,
    payload,
    received_at,
    validation_status,
    provider_metadata,
    legal_hold,
    signature_valid,
    applied_at,
    ingress_status,
    ingress_error;

-- name: MarkOutboxEventPublished :one
UPDATE outbox_events
SET
    published_at = now(),
    status = 'published',
    locked_by = NULL,
    locked_until = NULL,
    updated_at = now()
WHERE
    id = $1
    AND published_at IS NULL
RETURNING
    id,
    organization_id,
    topic,
    event_type,
    payload,
    aggregate_type,
    aggregate_id,
    idempotency_key,
    created_at,
    published_at,
    publish_attempt_count,
    last_publish_error,
    last_publish_attempt_at,
    next_publish_after,
    dead_lettered_at,
    status,
    locked_by,
    locked_until,
    updated_at,
    max_publish_attempts;

-- name: CommerceIsProductInMachinePublishedAssortment :one
SELECT
    EXISTS (
        SELECT
            1
        FROM
            machines m
            INNER JOIN machine_assortment_bindings b ON b.machine_id = m.id
            AND b.organization_id = m.organization_id
            AND b.is_primary
            AND b.valid_to IS NULL
            INNER JOIN assortments a ON a.id = b.assortment_id
            AND a.organization_id = m.organization_id
            AND a.status = 'published'
            INNER JOIN assortment_items ai ON ai.assortment_id = a.id
            AND ai.organization_id = m.organization_id
            AND ai.product_id = $3
        WHERE
            m.id = $1
            AND m.organization_id = $2
    ) AS ok;

-- name: InsertRefundRow :one
INSERT INTO refunds (
    payment_id,
    order_id,
    amount_minor,
    currency,
    state,
    reason,
    idempotency_key,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING
    id,
    payment_id,
    order_id,
    amount_minor,
    currency,
    state,
    reason,
    idempotency_key,
    metadata,
    created_at;

-- name: ListRefundsForOrder :many
SELECT
    id,
    payment_id,
    order_id,
    amount_minor,
    currency,
    state,
    reason,
    idempotency_key,
    metadata,
    created_at
FROM
    refunds
WHERE
    order_id = $1
ORDER BY
    created_at ASC;

-- name: GetRefundByIDForOrder :one
SELECT
    id,
    payment_id,
    order_id,
    amount_minor,
    currency,
    state,
    reason,
    idempotency_key,
    metadata,
    created_at
FROM
    refunds
WHERE
    id = $1
    AND order_id = $2;

-- name: GetRefundByOrderIdempotency :one
SELECT
    id,
    payment_id,
    order_id,
    amount_minor,
    currency,
    state,
    reason,
    idempotency_key,
    metadata,
    created_at
FROM
    refunds
WHERE
    order_id = $1
    AND idempotency_key = $2;

-- name: SumNonFailedRefundAmountForPayment :one
SELECT
    COALESCE(SUM(amount_minor), 0)::bigint AS refunded_minor
FROM
    refunds
WHERE
    payment_id = $1
    AND state <> 'failed';
