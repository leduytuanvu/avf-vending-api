package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type GetCommandLedgerByMachineSequenceParams struct {
	MachineID uuid.UUID
	Sequence  int64
}

const getCommandLedgerByMachineSequence = `-- name: GetCommandLedgerByMachineSequence :one
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
    AND sequence = $2
`

func (q *Queries) GetCommandLedgerByMachineSequence(ctx context.Context, arg GetCommandLedgerByMachineSequenceParams) (CommandLedger, error) {
	row := q.db.QueryRow(ctx, getCommandLedgerByMachineSequence, arg.MachineID, arg.Sequence)
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

const getLatestMachineCommandAttemptByCommandID = `-- name: GetLatestMachineCommandAttemptByCommandID :one
SELECT
    id,
    command_id,
    machine_id,
    transport_session_id,
    attempt_no,
    sent_at,
    ack_deadline_at,
    acked_at,
    result_received_at,
    status,
    timeout_reason,
    protocol_pack_no,
    sequence_no,
    correlation_id,
    request_payload_json,
    raw_request,
    raw_response,
    latency_ms
FROM machine_command_attempts
WHERE command_id = $1
ORDER BY attempt_no DESC
LIMIT 1
`

func (q *Queries) GetLatestMachineCommandAttemptByCommandID(ctx context.Context, commandID uuid.UUID) (MachineCommandAttempt, error) {
	row := q.db.QueryRow(ctx, getLatestMachineCommandAttemptByCommandID, commandID)
	return scanMachineCommandAttempt(row)
}

const getLatestOpenMachineCommandAttemptForCommand = `-- name: GetLatestOpenMachineCommandAttemptForCommand :one
SELECT
    id,
    command_id,
    machine_id,
    transport_session_id,
    attempt_no,
    sent_at,
    ack_deadline_at,
    acked_at,
    result_received_at,
    status,
    timeout_reason,
    protocol_pack_no,
    sequence_no,
    correlation_id,
    request_payload_json,
    raw_request,
    raw_response,
    latency_ms
FROM machine_command_attempts
WHERE
    command_id = $1
    AND status IN ('pending', 'sent')
ORDER BY attempt_no DESC
LIMIT 1
`

func (q *Queries) GetLatestOpenMachineCommandAttemptForCommand(ctx context.Context, commandID uuid.UUID) (MachineCommandAttempt, error) {
	row := q.db.QueryRow(ctx, getLatestOpenMachineCommandAttemptForCommand, commandID)
	return scanMachineCommandAttempt(row)
}

type InsertMachineCommandAttemptParams struct {
	CommandID          uuid.UUID
	MachineID          uuid.UUID
	CorrelationID      *uuid.UUID
	RequestPayloadJSON []byte
}

const insertMachineCommandAttempt = `-- name: InsertMachineCommandAttempt :one
INSERT INTO machine_command_attempts (
    command_id,
    machine_id,
    attempt_no,
    sent_at,
    status,
    correlation_id,
    request_payload_json
)
VALUES (
    $1,
    $2,
    (
        SELECT COALESCE(MAX(attempt_no), 0) + 1
        FROM machine_command_attempts mc
        WHERE
            mc.command_id = $1
    ),
    now(),
    'pending',
    $3,
    $4
)
RETURNING
    id,
    command_id,
    machine_id,
    transport_session_id,
    attempt_no,
    sent_at,
    ack_deadline_at,
    acked_at,
    result_received_at,
    status,
    timeout_reason,
    protocol_pack_no,
    sequence_no,
    correlation_id,
    request_payload_json,
    raw_request,
    raw_response,
    latency_ms
`

func (q *Queries) InsertMachineCommandAttempt(ctx context.Context, arg InsertMachineCommandAttemptParams) (MachineCommandAttempt, error) {
	row := q.db.QueryRow(ctx, insertMachineCommandAttempt,
		arg.CommandID,
		arg.MachineID,
		arg.CorrelationID,
		arg.RequestPayloadJSON,
	)
	return scanMachineCommandAttempt(row)
}

type UpdateCommandLedgerMQTTDispatchMetaParams struct {
	ID        uuid.UUID
	TimeoutAt time.Time
}

const updateCommandLedgerMQTTDispatchMeta = `-- name: UpdateCommandLedgerMQTTDispatchMeta :exec
UPDATE command_ledger
SET
    protocol_type = COALESCE(protocol_type, 'mqtt'),
    timeout_at = $2,
    attempt_count = attempt_count + 1,
    last_attempt_at = now()
WHERE
    id = $1
`

func (q *Queries) UpdateCommandLedgerMQTTDispatchMeta(ctx context.Context, arg UpdateCommandLedgerMQTTDispatchMetaParams) error {
	_, err := q.db.Exec(ctx, updateCommandLedgerMQTTDispatchMeta, arg.ID, arg.TimeoutAt)
	return err
}

type UpdateMachineCommandAttemptAfterDeviceReceiptParams struct {
	ID     uuid.UUID
	Status string
}

const updateMachineCommandAttemptAfterDeviceReceipt = `-- name: UpdateMachineCommandAttemptAfterDeviceReceipt :exec
UPDATE machine_command_attempts
SET
    status = $2,
    result_received_at = now(),
    acked_at = CASE
        WHEN $2 = 'completed' THEN now()
        ELSE acked_at
    END
WHERE
    id = $1
    AND status IN ('pending', 'sent')
`

func (q *Queries) UpdateMachineCommandAttemptAfterDeviceReceipt(ctx context.Context, arg UpdateMachineCommandAttemptAfterDeviceReceiptParams) error {
	_, err := q.db.Exec(ctx, updateMachineCommandAttemptAfterDeviceReceipt, arg.ID, arg.Status)
	return err
}

type UpdateMachineCommandAttemptSentParams struct {
	ID            uuid.UUID
	AckDeadlineAt time.Time
}

const updateMachineCommandAttemptSent = `-- name: UpdateMachineCommandAttemptSent :exec
UPDATE machine_command_attempts
SET
    status = 'sent',
    ack_deadline_at = $2
WHERE
    id = $1
    AND status = 'pending'
`

func (q *Queries) UpdateMachineCommandAttemptSent(ctx context.Context, arg UpdateMachineCommandAttemptSentParams) error {
	_, err := q.db.Exec(ctx, updateMachineCommandAttemptSent, arg.ID, arg.AckDeadlineAt)
	return err
}

type UpdateMachineCommandAttemptPublishFailedParams struct {
	ID            uuid.UUID
	TimeoutReason string
}

const updateMachineCommandAttemptPublishFailed = `-- name: UpdateMachineCommandAttemptPublishFailed :exec
UPDATE machine_command_attempts
SET
    status = 'failed',
    result_received_at = now(),
    timeout_reason = $2
WHERE
    id = $1
    AND status = 'pending'
`

func (q *Queries) UpdateMachineCommandAttemptPublishFailed(ctx context.Context, arg UpdateMachineCommandAttemptPublishFailedParams) error {
	_, err := q.db.Exec(ctx, updateMachineCommandAttemptPublishFailed, arg.ID, arg.TimeoutReason)
	return err
}

const applyMachineCommandAckTimeouts = `-- name: ApplyMachineCommandAckTimeouts :exec
UPDATE machine_command_attempts
SET
    status = 'ack_timeout',
    result_received_at = now(),
    timeout_reason = 'ack_deadline_exceeded'
WHERE
    status = 'sent'
    AND ack_deadline_at IS NOT NULL
    AND ack_deadline_at < $1
`

func (q *Queries) ApplyMachineCommandAckTimeouts(ctx context.Context, before time.Time) error {
	_, err := q.db.Exec(ctx, applyMachineCommandAckTimeouts, before)
	return err
}

type ListDeviceCommandReceiptsByMachineParams struct {
	MachineID uuid.UUID
	Limit     int32
}

const listDeviceCommandReceiptsByMachine = `-- name: ListDeviceCommandReceiptsByMachine :many
SELECT
    id,
    machine_id,
    sequence,
    status,
    correlation_id,
    payload,
    dedupe_key,
    received_at,
    command_attempt_id
FROM device_command_receipts
WHERE
    machine_id = $1
ORDER BY received_at DESC
LIMIT $2
`

func (q *Queries) ListDeviceCommandReceiptsByMachine(ctx context.Context, arg ListDeviceCommandReceiptsByMachineParams) ([]DeviceCommandReceipt, error) {
	rows, err := q.db.Query(ctx, listDeviceCommandReceiptsByMachine, arg.MachineID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DeviceCommandReceipt
	for rows.Next() {
		var r DeviceCommandReceipt
		if err := rows.Scan(
			&r.ID,
			&r.MachineID,
			&r.Sequence,
			&r.Status,
			&r.CorrelationID,
			&r.Payload,
			&r.DedupeKey,
			&r.ReceivedAt,
			&r.CommandAttemptID,
		); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}

type machineCommandAttemptScanner interface {
	Scan(dest ...any) error
}

func scanMachineCommandAttempt(row machineCommandAttemptScanner) (MachineCommandAttempt, error) {
	var m MachineCommandAttempt
	err := row.Scan(
		&m.ID,
		&m.CommandID,
		&m.MachineID,
		&m.TransportSessionID,
		&m.AttemptNo,
		&m.SentAt,
		&m.AckDeadlineAt,
		&m.AckedAt,
		&m.ResultReceivedAt,
		&m.Status,
		&m.TimeoutReason,
		&m.ProtocolPackNo,
		&m.SequenceNo,
		&m.CorrelationID,
		&m.RequestPayloadJSON,
		&m.RawRequest,
		&m.RawResponse,
		&m.LatencyMs,
	)
	return m, err
}
