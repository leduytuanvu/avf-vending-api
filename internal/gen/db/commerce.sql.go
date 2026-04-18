package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type InsertOrderParams struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	Status         string
	Currency       string
	SubtotalMinor  int64
	TaxMinor       int64
	TotalMinor     int64
	IdempotencyKey *string
}

const insertOrder = `-- name: InsertOrder :one
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
RETURNING id, organization_id, machine_id, status, currency, subtotal_minor, tax_minor, total_minor, idempotency_key, created_at, updated_at
`

func (q *Queries) InsertOrder(ctx context.Context, arg InsertOrderParams) (Order, error) {
	row := q.db.QueryRow(ctx, insertOrder,
		arg.OrganizationID,
		arg.MachineID,
		arg.Status,
		arg.Currency,
		arg.SubtotalMinor,
		arg.TaxMinor,
		arg.TotalMinor,
		arg.IdempotencyKey,
	)
	var o Order
	err := row.Scan(
		&o.ID,
		&o.OrganizationID,
		&o.MachineID,
		&o.Status,
		&o.Currency,
		&o.SubtotalMinor,
		&o.TaxMinor,
		&o.TotalMinor,
		&o.IdempotencyKey,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	return o, err
}

type InsertVendSessionParams struct {
	OrderID   uuid.UUID
	MachineID uuid.UUID
	SlotIndex int32
	ProductID uuid.UUID
	State     string
}

const insertVendSession = `-- name: InsertVendSession :one
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
    created_at
`

func (q *Queries) InsertVendSession(ctx context.Context, arg InsertVendSessionParams) (VendSession, error) {
	row := q.db.QueryRow(ctx, insertVendSession,
		arg.OrderID,
		arg.MachineID,
		arg.SlotIndex,
		arg.ProductID,
		arg.State,
	)
	var v VendSession
	err := row.Scan(
		&v.ID,
		&v.OrderID,
		&v.MachineID,
		&v.SlotIndex,
		&v.ProductID,
		&v.State,
		&v.FailureReason,
		&v.CorrelationID,
		&v.StartedAt,
		&v.CompletedAt,
		&v.FinalCommandAttemptID,
		&v.CreatedAt,
	)
	return v, err
}

type InsertPaymentParams struct {
	OrderID        uuid.UUID
	Provider       string
	State          string
	AmountMinor    int64
	Currency       string
	IdempotencyKey *string
}

const insertPayment = `-- name: InsertPayment :one
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
    settlement_batch_id
`

func (q *Queries) InsertPayment(ctx context.Context, arg InsertPaymentParams) (Payment, error) {
	row := q.db.QueryRow(ctx, insertPayment,
		arg.OrderID,
		arg.Provider,
		arg.State,
		arg.AmountMinor,
		arg.Currency,
		arg.IdempotencyKey,
	)
	var p Payment
	err := row.Scan(
		&p.ID,
		&p.OrderID,
		&p.Provider,
		&p.State,
		&p.AmountMinor,
		&p.Currency,
		&p.IdempotencyKey,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.ReconciliationStatus,
		&p.SettlementStatus,
		&p.SettlementBatchID,
	)
	return p, err
}

type GetPaymentByOrderAndIdempotencyKeyParams struct {
	OrderID        uuid.UUID
	IdempotencyKey string
}

const getPaymentByOrderAndIdempotencyKey = `-- name: GetPaymentByOrderAndIdempotencyKey :one
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
    AND idempotency_key = $2
`

type GetOrderByOrgIdempotencyParams struct {
	OrganizationID uuid.UUID
	IdempotencyKey string
}

const getOrderByOrgIdempotency = `-- name: GetOrderByOrgIdempotency :one
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
    AND idempotency_key = $2
`

func (q *Queries) GetOrderByOrgIdempotency(ctx context.Context, arg GetOrderByOrgIdempotencyParams) (Order, error) {
	row := q.db.QueryRow(ctx, getOrderByOrgIdempotency, arg.OrganizationID, arg.IdempotencyKey)
	var o Order
	err := row.Scan(
		&o.ID,
		&o.OrganizationID,
		&o.MachineID,
		&o.Status,
		&o.Currency,
		&o.SubtotalMinor,
		&o.TaxMinor,
		&o.TotalMinor,
		&o.IdempotencyKey,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	return o, err
}

const getOrderByID = `-- name: GetOrderByID :one
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
`

func (q *Queries) GetOrderByID(ctx context.Context, id uuid.UUID) (Order, error) {
	row := q.db.QueryRow(ctx, getOrderByID, id)
	var o Order
	err := row.Scan(
		&o.ID,
		&o.OrganizationID,
		&o.MachineID,
		&o.Status,
		&o.Currency,
		&o.SubtotalMinor,
		&o.TaxMinor,
		&o.TotalMinor,
		&o.IdempotencyKey,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	return o, err
}

type GetVendSessionByOrderAndSlotParams struct {
	OrderID   uuid.UUID
	SlotIndex int32
}

const getVendSessionByOrderAndSlot = `-- name: GetVendSessionByOrderAndSlot :one
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
`

func (q *Queries) GetVendSessionByOrderAndSlot(ctx context.Context, arg GetVendSessionByOrderAndSlotParams) (VendSession, error) {
	row := q.db.QueryRow(ctx, getVendSessionByOrderAndSlot, arg.OrderID, arg.SlotIndex)
	var v VendSession
	err := row.Scan(
		&v.ID,
		&v.OrderID,
		&v.MachineID,
		&v.SlotIndex,
		&v.ProductID,
		&v.State,
		&v.FailureReason,
		&v.CorrelationID,
		&v.StartedAt,
		&v.CompletedAt,
		&v.FinalCommandAttemptID,
		&v.CreatedAt,
	)
	return v, err
}

func (q *Queries) GetPaymentByOrderAndIdempotencyKey(ctx context.Context, arg GetPaymentByOrderAndIdempotencyKeyParams) (Payment, error) {
	row := q.db.QueryRow(ctx, getPaymentByOrderAndIdempotencyKey, arg.OrderID, arg.IdempotencyKey)
	var p Payment
	err := row.Scan(
		&p.ID,
		&p.OrderID,
		&p.Provider,
		&p.State,
		&p.AmountMinor,
		&p.Currency,
		&p.IdempotencyKey,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.ReconciliationStatus,
		&p.SettlementStatus,
		&p.SettlementBatchID,
	)
	return p, err
}

const listPaymentsPendingTimeout = `-- name: ListPaymentsPendingTimeout :many
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
LIMIT $2
`

func (q *Queries) ListPaymentsPendingTimeout(ctx context.Context, before time.Time, limit int32) ([]Payment, error) {
	rows, err := q.db.Query(ctx, listPaymentsPendingTimeout, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(
			&p.ID,
			&p.OrderID,
			&p.Provider,
			&p.State,
			&p.AmountMinor,
			&p.Currency,
			&p.IdempotencyKey,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.ReconciliationStatus,
			&p.SettlementStatus,
			&p.SettlementBatchID,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const listOrdersWithUnresolvedPayment = `-- name: ListOrdersWithUnresolvedPayment :many
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
LIMIT $2
`

func (q *Queries) ListOrdersWithUnresolvedPayment(ctx context.Context, before time.Time, limit int32) ([]Order, error) {
	rows, err := q.db.Query(ctx, listOrdersWithUnresolvedPayment, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(
			&o.ID,
			&o.OrganizationID,
			&o.MachineID,
			&o.Status,
			&o.Currency,
			&o.SubtotalMinor,
			&o.TaxMinor,
			&o.TotalMinor,
			&o.IdempotencyKey,
			&o.CreatedAt,
			&o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

const listVendSessionsStuckForReconciliation = `-- name: ListVendSessionsStuckForReconciliation :many
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
    o.status AS order_status
FROM vend_sessions v
INNER JOIN orders o ON o.id = v.order_id
WHERE
    v.state IN ('pending', 'in_progress')
    AND v.created_at < $1
    AND o.status IN ('paid', 'vending', 'created')
ORDER BY
    v.created_at ASC
LIMIT $2
`

type ListVendSessionsStuckForReconciliationRow struct {
	ID                    uuid.UUID  `json:"id"`
	OrderID               uuid.UUID  `json:"order_id"`
	MachineID             uuid.UUID  `json:"machine_id"`
	SlotIndex             int32      `json:"slot_index"`
	ProductID             uuid.UUID  `json:"product_id"`
	State                 string     `json:"state"`
	FailureReason         *string    `json:"failure_reason"`
	CorrelationID         *uuid.UUID `json:"correlation_id"`
	StartedAt             *time.Time `json:"started_at"`
	CompletedAt           *time.Time `json:"completed_at"`
	FinalCommandAttemptID *uuid.UUID `json:"final_command_attempt_id"`
	CreatedAt             time.Time  `json:"created_at"`
	OrderStatus           string     `json:"order_status"`
}

func (q *Queries) ListVendSessionsStuckForReconciliation(ctx context.Context, before time.Time, limit int32) ([]ListVendSessionsStuckForReconciliationRow, error) {
	rows, err := q.db.Query(ctx, listVendSessionsStuckForReconciliation, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ListVendSessionsStuckForReconciliationRow
	for rows.Next() {
		var i ListVendSessionsStuckForReconciliationRow
		if err := rows.Scan(
			&i.ID,
			&i.OrderID,
			&i.MachineID,
			&i.SlotIndex,
			&i.ProductID,
			&i.State,
			&i.FailureReason,
			&i.CorrelationID,
			&i.StartedAt,
			&i.CompletedAt,
			&i.FinalCommandAttemptID,
			&i.CreatedAt,
			&i.OrderStatus,
		); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

const listPotentialDuplicatePayments = `-- name: ListPotentialDuplicatePayments :many
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
LIMIT $2
`

func (q *Queries) ListPotentialDuplicatePayments(ctx context.Context, before time.Time, limit int32) ([]Payment, error) {
	rows, err := q.db.Query(ctx, listPotentialDuplicatePayments, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(
			&p.ID,
			&p.OrderID,
			&p.Provider,
			&p.State,
			&p.AmountMinor,
			&p.Currency,
			&p.IdempotencyKey,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.ReconciliationStatus,
			&p.SettlementStatus,
			&p.SettlementBatchID,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const listPaymentsForRefundReview = `-- name: ListPaymentsForRefundReview :many
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
LIMIT $2
`

func (q *Queries) ListPaymentsForRefundReview(ctx context.Context, before time.Time, limit int32) ([]Payment, error) {
	rows, err := q.db.Query(ctx, listPaymentsForRefundReview, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(
			&p.ID,
			&p.OrderID,
			&p.Provider,
			&p.State,
			&p.AmountMinor,
			&p.Currency,
			&p.IdempotencyKey,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.ReconciliationStatus,
			&p.SettlementStatus,
			&p.SettlementBatchID,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const listStaleCommandLedgerEntries = `-- name: ListStaleCommandLedgerEntries :many
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
LIMIT $2
`

func (q *Queries) ListStaleCommandLedgerEntries(ctx context.Context, before time.Time, limit int32) ([]CommandLedger, error) {
	rows, err := q.db.Query(ctx, listStaleCommandLedgerEntries, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CommandLedger
	for rows.Next() {
		var c CommandLedger
		if err := rows.Scan(
			&c.ID,
			&c.MachineID,
			&c.Sequence,
			&c.CommandType,
			&c.Payload,
			&c.CorrelationID,
			&c.IdempotencyKey,
			&c.CreatedAt,
			&c.ProtocolType,
			&c.DeadlineAt,
			&c.TimeoutAt,
			&c.AttemptCount,
			&c.LastAttemptAt,
			&c.RouteKey,
			&c.SourceSystem,
			&c.SourceEventID,
			&c.OperatorSessionID,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const markOutboxEventPublished = `-- name: MarkOutboxEventPublished :one
UPDATE outbox_events
SET
    published_at = now()
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
    dead_lettered_at
`

func (q *Queries) MarkOutboxEventPublished(ctx context.Context, id int64) (OutboxEvent, error) {
	row := q.db.QueryRow(ctx, markOutboxEventPublished, id)
	var e OutboxEvent
	err := row.Scan(
		&e.ID,
		&e.OrganizationID,
		&e.Topic,
		&e.EventType,
		&e.Payload,
		&e.AggregateType,
		&e.AggregateID,
		&e.IdempotencyKey,
		&e.CreatedAt,
		&e.PublishedAt,
		&e.PublishAttemptCount,
		&e.LastPublishError,
		&e.LastPublishAttemptAt,
		&e.NextPublishAfter,
		&e.DeadLetteredAt,
	)
	return e, err
}

type UpdateOrderStatusByOrgParams struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Status         string
}

const updateOrderStatusByOrg = `-- name: UpdateOrderStatusByOrg :one
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
    updated_at
`

func (q *Queries) UpdateOrderStatusByOrg(ctx context.Context, arg UpdateOrderStatusByOrgParams) (Order, error) {
	row := q.db.QueryRow(ctx, updateOrderStatusByOrg, arg.ID, arg.OrganizationID, arg.Status)
	var o Order
	err := row.Scan(
		&o.ID,
		&o.OrganizationID,
		&o.MachineID,
		&o.Status,
		&o.Currency,
		&o.SubtotalMinor,
		&o.TaxMinor,
		&o.TotalMinor,
		&o.IdempotencyKey,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	return o, err
}

type UpdateVendSessionStateByOrderSlotParams struct {
	OrderID       uuid.UUID
	SlotIndex     int32
	State         string
	FailureReason *string
}

const updateVendSessionStateByOrderSlot = `-- name: UpdateVendSessionStateByOrderSlot :one
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
    created_at
`

func (q *Queries) UpdateVendSessionStateByOrderSlot(ctx context.Context, arg UpdateVendSessionStateByOrderSlotParams) (VendSession, error) {
	row := q.db.QueryRow(ctx, updateVendSessionStateByOrderSlot,
		arg.OrderID,
		arg.SlotIndex,
		arg.State,
		arg.FailureReason,
	)
	var v VendSession
	err := row.Scan(
		&v.ID,
		&v.OrderID,
		&v.MachineID,
		&v.SlotIndex,
		&v.ProductID,
		&v.State,
		&v.FailureReason,
		&v.CorrelationID,
		&v.StartedAt,
		&v.CompletedAt,
		&v.FinalCommandAttemptID,
		&v.CreatedAt,
	)
	return v, err
}

const getLatestPaymentForOrder = `-- name: GetLatestPaymentForOrder :one
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
LIMIT 1
`

func (q *Queries) GetLatestPaymentForOrder(ctx context.Context, orderID uuid.UUID) (Payment, error) {
	row := q.db.QueryRow(ctx, getLatestPaymentForOrder, orderID)
	var p Payment
	err := row.Scan(
		&p.ID,
		&p.OrderID,
		&p.Provider,
		&p.State,
		&p.AmountMinor,
		&p.Currency,
		&p.IdempotencyKey,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.ReconciliationStatus,
		&p.SettlementStatus,
		&p.SettlementBatchID,
	)
	return p, err
}

const getPaymentByID = `-- name: GetPaymentByID :one
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
    id = $1
`

func (q *Queries) GetPaymentByID(ctx context.Context, id uuid.UUID) (Payment, error) {
	row := q.db.QueryRow(ctx, getPaymentByID, id)
	var p Payment
	err := row.Scan(
		&p.ID,
		&p.OrderID,
		&p.Provider,
		&p.State,
		&p.AmountMinor,
		&p.Currency,
		&p.IdempotencyKey,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.ReconciliationStatus,
		&p.SettlementStatus,
		&p.SettlementBatchID,
	)
	return p, err
}

type InsertPaymentAttemptParams struct {
	PaymentID         uuid.UUID
	ProviderReference *string
	State             string
	Payload           []byte
}

const insertPaymentAttempt = `-- name: InsertPaymentAttempt :one
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
    created_at
`

func (q *Queries) InsertPaymentAttempt(ctx context.Context, arg InsertPaymentAttemptParams) (PaymentAttempt, error) {
	row := q.db.QueryRow(ctx, insertPaymentAttempt,
		arg.PaymentID,
		arg.ProviderReference,
		arg.State,
		arg.Payload,
	)
	var i PaymentAttempt
	err := row.Scan(
		&i.ID,
		&i.PaymentID,
		&i.ProviderReference,
		&i.State,
		&i.Payload,
		&i.CreatedAt,
	)
	return i, err
}

type UpdatePaymentStateForReconciliationParams struct {
	ID    uuid.UUID
	State string
}

const updatePaymentStateForReconciliation = `-- name: UpdatePaymentStateForReconciliation :one
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
    settlement_batch_id
`

func (q *Queries) UpdatePaymentStateForReconciliation(ctx context.Context, arg UpdatePaymentStateForReconciliationParams) (Payment, error) {
	row := q.db.QueryRow(ctx, updatePaymentStateForReconciliation, arg.ID, arg.State)
	var p Payment
	err := row.Scan(
		&p.ID,
		&p.OrderID,
		&p.Provider,
		&p.State,
		&p.AmountMinor,
		&p.Currency,
		&p.IdempotencyKey,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.ReconciliationStatus,
		&p.SettlementStatus,
		&p.SettlementBatchID,
	)
	return p, err
}

type UpdatePaymentStateParams struct {
	ID    uuid.UUID
	State string
}

const updatePaymentState = `-- name: UpdatePaymentState :one
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
    settlement_batch_id
`

func (q *Queries) UpdatePaymentState(ctx context.Context, arg UpdatePaymentStateParams) (Payment, error) {
	row := q.db.QueryRow(ctx, updatePaymentState, arg.ID, arg.State)
	var p Payment
	err := row.Scan(
		&p.ID,
		&p.OrderID,
		&p.Provider,
		&p.State,
		&p.AmountMinor,
		&p.Currency,
		&p.IdempotencyKey,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.ReconciliationStatus,
		&p.SettlementStatus,
		&p.SettlementBatchID,
	)
	return p, err
}

type GetPaymentProviderEventByProviderRefParams struct {
	Provider    string
	ProviderRef string
}

const getPaymentProviderEventByProviderRef = `-- name: GetPaymentProviderEventByProviderRef :one
SELECT
    id,
    payment_id,
    provider,
    provider_ref,
    provider_amount_minor,
    currency,
    event_type,
    payload,
    received_at
FROM payment_provider_events
WHERE
    provider = $1
    AND provider_ref = $2
`

func (q *Queries) GetPaymentProviderEventByProviderRef(ctx context.Context, arg GetPaymentProviderEventByProviderRefParams) (PaymentProviderEvent, error) {
	row := q.db.QueryRow(ctx, getPaymentProviderEventByProviderRef, arg.Provider, arg.ProviderRef)
	var i PaymentProviderEvent
	err := row.Scan(
		&i.ID,
		&i.PaymentID,
		&i.Provider,
		&i.ProviderRef,
		&i.ProviderAmountMinor,
		&i.Currency,
		&i.EventType,
		&i.Payload,
		&i.ReceivedAt,
	)
	return i, err
}

type InsertPaymentProviderEventParams struct {
	PaymentID           uuid.UUID
	Provider            string
	ProviderRef         string
	ProviderAmountMinor *int64
	Currency            *string
	EventType           string
	Payload             []byte
}

const insertPaymentProviderEvent = `-- name: InsertPaymentProviderEvent :one
INSERT INTO payment_provider_events (
    payment_id,
    provider,
    provider_ref,
    provider_amount_minor,
    currency,
    event_type,
    payload
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING
    id,
    payment_id,
    provider,
    provider_ref,
    provider_amount_minor,
    currency,
    event_type,
    payload,
    received_at
`

func (q *Queries) InsertPaymentProviderEvent(ctx context.Context, arg InsertPaymentProviderEventParams) (PaymentProviderEvent, error) {
	ref := arg.ProviderRef
	row := q.db.QueryRow(ctx, insertPaymentProviderEvent,
		arg.PaymentID,
		arg.Provider,
		ref,
		arg.ProviderAmountMinor,
		arg.Currency,
		arg.EventType,
		arg.Payload,
	)
	var i PaymentProviderEvent
	err := row.Scan(
		&i.ID,
		&i.PaymentID,
		&i.Provider,
		&i.ProviderRef,
		&i.ProviderAmountMinor,
		&i.Currency,
		&i.EventType,
		&i.Payload,
		&i.ReceivedAt,
	)
	return i, err
}
