// Package postgres implements MQTT command transport helpers (dispatch attempts, ack deadlines).
// Device command outcomes are accepted on MQTT topics commands/receipt or commands/ack (see mqtt.Dispatch).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/observability/mqttprom"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrMQTTMaxDispatchAttemptsExceeded is returned when machine_command_attempts rows for a command_id reach command_ledger.max_dispatch_attempts.
var ErrMQTTMaxDispatchAttemptsExceeded = errors.New("postgres: MQTT dispatch attempt limit reached for command")

// ListDeviceCommandReceiptsByMachine returns recent command receipts for a machine (newest first).
func (s *Store) ListDeviceCommandReceiptsByMachine(ctx context.Context, machineID uuid.UUID, limit int32) ([]db.DeviceCommandReceipt, error) {
	return db.New(s.pool).ListDeviceCommandReceiptsByMachine(ctx, db.ListDeviceCommandReceiptsByMachineParams{
		MachineID: machineID,
		Limit:     limit,
	})
}

// ApplyMQTTCommandAckTimeouts marks sent attempts whose ledger timeout or ack deadline passed.
// Ledger timeout (command_ledger.timeout_at) is applied before per-attempt ack_deadline so SLA-style
// expiry wins when both are in the past (otherwise rows would only become ack_timeout).
func (s *Store) ApplyMQTTCommandAckTimeouts(ctx context.Context, before time.Time) error {
	q := db.New(s.pool)
	nExp, err := q.ApplyMachineCommandLedgerExpired(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		return err
	}
	mqttprom.AddCommandAttemptsExpired(nExp)
	nAck, err := q.ApplyMachineCommandAckTimeouts(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		return err
	}
	mqttprom.AddMachineCommandAckDeadlinesExceeded(nAck)
	return nil
}

// GetCommandLedgerByMachineSequence loads a ledger row by machine and monotonic sequence.
func (s *Store) GetCommandLedgerByMachineSequence(ctx context.Context, machineID uuid.UUID, sequence int64) (db.CommandLedger, error) {
	return db.New(s.pool).GetCommandLedgerByMachineSequence(ctx, db.GetCommandLedgerByMachineSequenceParams{
		MachineID: machineID,
		Sequence:  sequence,
	})
}

// GetLatestMachineCommandAttemptByCommandID returns the newest attempt row for a command.
func (s *Store) GetLatestMachineCommandAttemptByCommandID(ctx context.Context, commandID uuid.UUID) (db.MachineCommandAttempt, error) {
	return db.New(s.pool).GetLatestMachineCommandAttemptByCommandID(ctx, commandID)
}

// InsertMQTTDispatchAttemptWithLedgerMeta inserts a pending attempt and bumps command_ledger transport metadata atomically.
// mqttRouteMeta, when non-empty, is stored in command_ledger.route_key (JSON: mqtt topic + payload sha256 hex for ops/debug).
func (s *Store) InsertMQTTDispatchAttemptWithLedgerMeta(ctx context.Context, commandID, machineID uuid.UUID, correlationID *uuid.UUID, requestWireJSON []byte, ledgerTimeoutAt time.Time, mqttRouteMeta string) (db.MachineCommandAttempt, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.MachineCommandAttempt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	ledger, err := q.GetCommandLedgerByID(ctx, commandID)
	if err != nil {
		return db.MachineCommandAttempt{}, err
	}
	nAttempts, err := q.CountMachineCommandAttemptsByCommandID(ctx, commandID)
	if err != nil {
		return db.MachineCommandAttempt{}, err
	}
	if nAttempts >= int64(ledger.MaxDispatchAttempts) {
		mqttprom.RecordMQTTDispatchRefused("max_dispatch_attempts")
		return db.MachineCommandAttempt{}, ErrMQTTMaxDispatchAttemptsExceeded
	}
	var corr pgtype.UUID
	if correlationID != nil {
		corr = pgtype.UUID{Bytes: *correlationID, Valid: true}
	}
	att, err := q.InsertMachineCommandAttempt(ctx, db.InsertMachineCommandAttemptParams{
		CommandID:          commandID,
		MachineID:          machineID,
		CorrelationID:      corr,
		RequestPayloadJson: requestWireJSON,
	})
	if err != nil {
		return db.MachineCommandAttempt{}, err
	}
	if err := q.UpdateCommandLedgerMQTTDispatchMeta(ctx, db.UpdateCommandLedgerMQTTDispatchMetaParams{
		ID:        commandID,
		TimeoutAt: pgtype.Timestamptz{Time: ledgerTimeoutAt, Valid: true},
		Column3:   mqttRouteMeta,
	}); err != nil {
		return db.MachineCommandAttempt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.MachineCommandAttempt{}, err
	}
	mqttprom.RecordCommandDispatchQueued()
	return att, nil
}

// MarkMQTTDispatchAttemptSent marks a pending attempt as sent and sets the device ack deadline.
func (s *Store) MarkMQTTDispatchAttemptSent(ctx context.Context, attemptID uuid.UUID, ackDeadline time.Time) error {
	if err := db.New(s.pool).UpdateMachineCommandAttemptSent(ctx, db.UpdateMachineCommandAttemptSentParams{
		ID:            attemptID,
		AckDeadlineAt: pgtype.Timestamptz{Time: ackDeadline, Valid: true},
	}); err != nil {
		return err
	}
	mqttprom.RecordCommandDispatchPublished()
	return nil
}

// MarkMQTTDispatchAttemptPublishFailed marks a pending attempt failed after a transport publish error.
func (s *Store) MarkMQTTDispatchAttemptPublishFailed(ctx context.Context, attemptID uuid.UUID, reason string) error {
	return db.New(s.pool).UpdateMachineCommandAttemptPublishFailed(ctx, db.UpdateMachineCommandAttemptPublishFailedParams{
		ID:            attemptID,
		TimeoutReason: pgtype.Text{String: reason, Valid: true},
	})
}

// HTTPCommandPollRow is a machine command still awaiting device-side handling (pending/sent attempt).
type HTTPCommandPollRow struct {
	CommandID      uuid.UUID
	Sequence       int64
	CommandType    string
	Payload        []byte
	CorrelationID  *uuid.UUID
	IdempotencyKey string
	AttemptStatus  string
}

// ListMachineCommandsForHTTPPoll returns open dispatch work for a machine (oldest sequence first).
func (s *Store) ListMachineCommandsForHTTPPoll(ctx context.Context, machineID uuid.UUID, limit int32) ([]HTTPCommandPollRow, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("postgres: nil store")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	const q = `
SELECT cl.id, cl.sequence, cl.command_type, cl.payload, cl.correlation_id, cl.idempotency_key, mca.status
FROM command_ledger cl
INNER JOIN machine_command_attempts mca ON mca.command_id = cl.id
    AND mca.attempt_no = (
        SELECT MAX(attempt_no) FROM machine_command_attempts m2 WHERE m2.command_id = cl.id
    )
WHERE cl.machine_id = $1
  AND mca.status IN ('pending', 'sent')
ORDER BY cl.sequence ASC
LIMIT $2`
	rows, err := s.pool.Query(ctx, q, machineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HTTPCommandPollRow
	for rows.Next() {
		var r HTTPCommandPollRow
		var corr pgtype.UUID
		var idem pgtype.Text
		if err := rows.Scan(&r.CommandID, &r.Sequence, &r.CommandType, &r.Payload, &corr, &idem, &r.AttemptStatus); err != nil {
			return nil, err
		}
		if corr.Valid {
			u := uuid.UUID(corr.Bytes)
			r.CorrelationID = &u
		}
		if idem.Valid {
			r.IdempotencyKey = idem.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
