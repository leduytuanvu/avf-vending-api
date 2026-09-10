package postgres

import (
	"context"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// insertInventoryEventsBatchJSON inserts inventory_events from a JSON array string.
// pgx encodes []byte as bytea; PostgreSQL jsonb columns require UTF-8 text JSON.
func insertInventoryEventsBatchJSON(ctx context.Context, tx pgx.Tx, payload []byte) ([]int64, error) {
	const q = `-- inventoryAdminInsertInventoryEventsBatch
INSERT INTO inventory_events (
    machine_id,
    machine_cabinet_id,
    cabinet_code,
    slot_code,
    product_id,
    event_type,
    reason_code,
    quantity_before,
    quantity_delta,
    quantity_after,
    unit_price_minor,
    currency,
    correlation_id,
    operator_session_id,
    technician_id,
    refill_session_id,
    inventory_count_session_id,
    occurred_at,
    recorded_at,
    metadata
)
SELECT
    (e->>'machine_id')::uuid AS machine_id,
    NULLIF (e->>'machine_cabinet_id', '')::uuid AS machine_cabinet_id,
    NULLIF (btrim(e->>'cabinet_code'), '') AS cabinet_code,
    NULLIF (e->>'slot_code', '') AS slot_code,
    NULLIF (e->>'product_id', '')::uuid AS product_id,
    e->>'event_type' AS event_type,
    NULLIF (btrim(e->>'reason_code'), '') AS reason_code,
    NULLIF (e->>'quantity_before', '')::int AS quantity_before,
    (e->>'quantity_delta')::int AS quantity_delta,
    NULLIF (e->>'quantity_after', '')::int AS quantity_after,
    coalesce((e->>'unit_price_minor')::bigint, 0) AS unit_price_minor,
    coalesce(
        NULLIF (btrim(e->>'currency'), ''),
        'USD'::text
    ) AS currency,
    NULLIF (e->>'correlation_id', '')::uuid AS correlation_id,
    NULLIF (e->>'operator_session_id', '')::uuid AS operator_session_id,
    NULLIF (e->>'technician_id', '')::uuid AS technician_id,
    NULLIF (e->>'refill_session_id', '')::uuid AS refill_session_id,
    NULLIF (e->>'inventory_count_session_id', '')::uuid AS inventory_count_session_id,
    coalesce((e->>'occurred_at')::timestamptz, now()) AS occurred_at,
    coalesce((e->>'recorded_at')::timestamptz, now()) AS recorded_at,
    coalesce(e->'metadata', '{}'::jsonb) AS metadata
FROM
    jsonb_array_elements(
        COALESCE(NULLIF($1::text, '')::jsonb, '[]'::jsonb)
    ) AS e
RETURNING
    id`

	rows, err := tx.Query(ctx, q, pgjson.RequiredString(payload))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func insertCommandLedgerEntryJSON(ctx context.Context, tx pgx.Tx, arg db.InsertCommandLedgerEntryParams) (db.CommandLedger, error) {
	const q = `
INSERT INTO command_ledger (
    machine_id,
    sequence,
    command_type,
    payload,
    correlation_id,
    idempotency_key,
    operator_session_id
)
VALUES (
    $1,
    $2,
    $3,
    COALESCE(NULLIF($4::text, '')::jsonb, '{}'::jsonb),
    $5,
    $6,
    $7
)
RETURNING
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
    operator_session_id,
    max_dispatch_attempts`
	var i db.CommandLedger
	err := tx.QueryRow(
		ctx,
		q,
		arg.MachineID,
		arg.Sequence,
		arg.CommandType,
		pgjson.RequiredString(arg.Payload),
		arg.CorrelationID,
		arg.IdempotencyKey,
		arg.OperatorSessionID,
	).Scan(
		&i.ID,
		&i.MachineID,
		&i.Sequence,
		&i.CommandType,
		&i.Payload,
		&i.CorrelationID,
		&i.IdempotencyKey,
		&i.CreatedAt,
		&i.ProtocolType,
		&i.DeadlineAt,
		&i.TimeoutAt,
		&i.AttemptCount,
		&i.LastAttemptAt,
		&i.RouteKey,
		&i.SourceSystem,
		&i.SourceEventID,
		&i.OperatorSessionID,
		&i.MaxDispatchAttempts,
	)
	return i, err
}

func upsertMachineShadowDesiredJSON(ctx context.Context, tx pgx.Tx, machineID uuid.UUID, desired []byte) error {
	const q = `
INSERT INTO machine_shadow (
    machine_id,
    desired_state,
    reported_state,
    version,
    updated_at
)
VALUES (
    $1,
    COALESCE(NULLIF($2::text, '')::jsonb, '{}'::jsonb),
    '{}'::jsonb,
    1,
    now()
)
ON CONFLICT (machine_id) DO UPDATE
SET
    desired_state = excluded.desired_state,
    version = machine_shadow.version + 1,
    updated_at = now()`
	_, err := tx.Exec(ctx, q, machineID, pgjson.RequiredString(desired))
	return err
}
