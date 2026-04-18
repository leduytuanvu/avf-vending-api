package db

import (
	"context"

	"github.com/google/uuid"
)

type InsertCommandLedgerEntryParams struct {
	MachineID         uuid.UUID
	Sequence          int64
	CommandType       string
	Payload           []byte
	CorrelationID     *uuid.UUID
	IdempotencyKey    *string
	OperatorSessionID *uuid.UUID
}

const insertCommandLedgerEntry = `-- name: InsertCommandLedgerEntry :one
INSERT INTO command_ledger (
    machine_id,
    sequence,
    command_type,
    payload,
    correlation_id,
    idempotency_key,
    operator_session_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
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
    operator_session_id
`

func (q *Queries) InsertCommandLedgerEntry(ctx context.Context, arg InsertCommandLedgerEntryParams) (CommandLedger, error) {
	row := q.db.QueryRow(ctx, insertCommandLedgerEntry,
		arg.MachineID,
		arg.Sequence,
		arg.CommandType,
		arg.Payload,
		arg.CorrelationID,
		arg.IdempotencyKey,
		arg.OperatorSessionID,
	)
	var c CommandLedger
	err := row.Scan(
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
	)
	return c, err
}

type UpsertMachineShadowDesiredParams struct {
	MachineID    uuid.UUID
	DesiredState []byte
}

const upsertMachineShadowDesired = `-- name: UpsertMachineShadowDesired :one
INSERT INTO machine_shadow (
    machine_id,
    desired_state,
    reported_state,
    version,
    updated_at
)
VALUES ($1, $2, '{}'::jsonb, 1, now())
ON CONFLICT (machine_id) DO UPDATE
SET
    desired_state = excluded.desired_state,
    version = machine_shadow.version + 1,
    updated_at = now()
RETURNING machine_id, desired_state, reported_state, version, updated_at
`

func (q *Queries) UpsertMachineShadowDesired(ctx context.Context, arg UpsertMachineShadowDesiredParams) (MachineShadow, error) {
	row := q.db.QueryRow(ctx, upsertMachineShadowDesired, arg.MachineID, arg.DesiredState)
	var m MachineShadow
	err := row.Scan(
		&m.MachineID,
		&m.DesiredState,
		&m.ReportedState,
		&m.Version,
		&m.UpdatedAt,
	)
	return m, err
}

const getMachineShadowByMachineID = `-- name: GetMachineShadowByMachineID :one
SELECT machine_id, desired_state, reported_state, version, updated_at
FROM machine_shadow
WHERE machine_id = $1
`

type GetCommandLedgerByMachineIdempotencyParams struct {
	MachineID      uuid.UUID
	IdempotencyKey string
}

const getCommandLedgerByMachineIdempotency = `-- name: GetCommandLedgerByMachineIdempotency :one
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
    machine_id = $1
    AND idempotency_key = $2
`

func (q *Queries) GetCommandLedgerByMachineIdempotency(ctx context.Context, arg GetCommandLedgerByMachineIdempotencyParams) (CommandLedger, error) {
	row := q.db.QueryRow(ctx, getCommandLedgerByMachineIdempotency, arg.MachineID, arg.IdempotencyKey)
	var c CommandLedger
	err := row.Scan(
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
	)
	return c, err
}

func (q *Queries) GetMachineShadowByMachineID(ctx context.Context, machineID uuid.UUID) (MachineShadow, error) {
	row := q.db.QueryRow(ctx, getMachineShadowByMachineID, machineID)
	var m MachineShadow
	err := row.Scan(
		&m.MachineID,
		&m.DesiredState,
		&m.ReportedState,
		&m.Version,
		&m.UpdatedAt,
	)
	return m, err
}

type UpsertMachineShadowReportedParams struct {
	MachineID     uuid.UUID
	ReportedState []byte
}

const upsertMachineShadowReported = `-- name: UpsertMachineShadowReported :one
INSERT INTO machine_shadow (
    machine_id,
    desired_state,
    reported_state,
    version,
    updated_at
)
VALUES ($1, '{}'::jsonb, $2, 1, now())
ON CONFLICT (machine_id) DO UPDATE
SET
    reported_state = excluded.reported_state,
    version = machine_shadow.version + 1,
    updated_at = now()
RETURNING machine_id, desired_state, reported_state, version, updated_at
`

func (q *Queries) UpsertMachineShadowReported(ctx context.Context, arg UpsertMachineShadowReportedParams) (MachineShadow, error) {
	row := q.db.QueryRow(ctx, upsertMachineShadowReported, arg.MachineID, arg.ReportedState)
	var m MachineShadow
	err := row.Scan(
		&m.MachineID,
		&m.DesiredState,
		&m.ReportedState,
		&m.Version,
		&m.UpdatedAt,
	)
	return m, err
}

const touchMachineConnectivity = `-- name: TouchMachineConnectivity :exec
UPDATE machines
SET
    updated_at = now(),
    status = CASE
        WHEN status = 'offline' THEN 'online'
        WHEN status = 'online' THEN 'online'
        ELSE status
    END
WHERE id = $1
`

func (q *Queries) TouchMachineConnectivity(ctx context.Context, machineID uuid.UUID) error {
	_, err := q.db.Exec(ctx, touchMachineConnectivity, machineID)
	return err
}

type InsertDeviceTelemetryEventParams struct {
	MachineID uuid.UUID
	EventType string
	Payload   []byte
	DedupeKey *string
}

const insertDeviceTelemetryEvent = `-- name: InsertDeviceTelemetryEvent :one
INSERT INTO device_telemetry_events (
    machine_id,
    event_type,
    payload,
    dedupe_key
)
VALUES ($1, $2, $3, $4)
RETURNING id, machine_id, event_type, payload, dedupe_key, received_at
`

func (q *Queries) InsertDeviceTelemetryEvent(ctx context.Context, arg InsertDeviceTelemetryEventParams) (DeviceTelemetryEvent, error) {
	row := q.db.QueryRow(ctx, insertDeviceTelemetryEvent,
		arg.MachineID,
		arg.EventType,
		arg.Payload,
		arg.DedupeKey,
	)
	var e DeviceTelemetryEvent
	err := row.Scan(
		&e.ID,
		&e.MachineID,
		&e.EventType,
		&e.Payload,
		&e.DedupeKey,
		&e.ReceivedAt,
	)
	return e, err
}

type InsertDeviceCommandReceiptParams struct {
	MachineID        uuid.UUID
	Sequence         int64
	Status           string
	CorrelationID    *uuid.UUID
	Payload          []byte
	DedupeKey        string
	CommandAttemptID *uuid.UUID
}

const insertDeviceCommandReceipt = `-- name: InsertDeviceCommandReceipt :one
INSERT INTO device_command_receipts (
    machine_id,
    sequence,
    status,
    correlation_id,
    payload,
    dedupe_key,
    command_attempt_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING
    id,
    machine_id,
    sequence,
    status,
    correlation_id,
    payload,
    dedupe_key,
    received_at,
    command_attempt_id
`

func (q *Queries) InsertDeviceCommandReceipt(ctx context.Context, arg InsertDeviceCommandReceiptParams) (DeviceCommandReceipt, error) {
	row := q.db.QueryRow(ctx, insertDeviceCommandReceipt,
		arg.MachineID,
		arg.Sequence,
		arg.Status,
		arg.CorrelationID,
		arg.Payload,
		arg.DedupeKey,
		arg.CommandAttemptID,
	)
	var r DeviceCommandReceipt
	err := row.Scan(
		&r.ID,
		&r.MachineID,
		&r.Sequence,
		&r.Status,
		&r.CorrelationID,
		&r.Payload,
		&r.DedupeKey,
		&r.ReceivedAt,
		&r.CommandAttemptID,
	)
	return r, err
}
