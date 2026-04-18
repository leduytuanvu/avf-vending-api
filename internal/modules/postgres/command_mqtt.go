package postgres

import (
	"context"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ListDeviceCommandReceiptsByMachine returns recent command receipts for a machine (newest first).
func (s *Store) ListDeviceCommandReceiptsByMachine(ctx context.Context, machineID uuid.UUID, limit int32) ([]db.DeviceCommandReceipt, error) {
	return db.New(s.pool).ListDeviceCommandReceiptsByMachine(ctx, db.ListDeviceCommandReceiptsByMachineParams{
		MachineID: machineID,
		Limit:     limit,
	})
}

// ApplyMQTTCommandAckTimeouts marks sent attempts whose ack deadline passed as ack_timeout.
func (s *Store) ApplyMQTTCommandAckTimeouts(ctx context.Context, before time.Time) error {
	return db.New(s.pool).ApplyMachineCommandAckTimeouts(ctx, before)
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
func (s *Store) InsertMQTTDispatchAttemptWithLedgerMeta(ctx context.Context, commandID, machineID uuid.UUID, correlationID *uuid.UUID, requestWireJSON []byte, ledgerTimeoutAt time.Time) (db.MachineCommandAttempt, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.MachineCommandAttempt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	att, err := q.InsertMachineCommandAttempt(ctx, db.InsertMachineCommandAttemptParams{
		CommandID:          commandID,
		MachineID:          machineID,
		CorrelationID:      correlationID,
		RequestPayloadJSON: requestWireJSON,
	})
	if err != nil {
		return db.MachineCommandAttempt{}, err
	}
	if err := q.UpdateCommandLedgerMQTTDispatchMeta(ctx, db.UpdateCommandLedgerMQTTDispatchMetaParams{
		ID:        commandID,
		TimeoutAt: ledgerTimeoutAt,
	}); err != nil {
		return db.MachineCommandAttempt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.MachineCommandAttempt{}, err
	}
	return att, nil
}

// MarkMQTTDispatchAttemptSent marks a pending attempt as sent and sets the device ack deadline.
func (s *Store) MarkMQTTDispatchAttemptSent(ctx context.Context, attemptID uuid.UUID, ackDeadline time.Time) error {
	return db.New(s.pool).UpdateMachineCommandAttemptSent(ctx, db.UpdateMachineCommandAttemptSentParams{
		ID:            attemptID,
		AckDeadlineAt: ackDeadline,
	})
}

// MarkMQTTDispatchAttemptPublishFailed marks a pending attempt failed after a transport publish error.
func (s *Store) MarkMQTTDispatchAttemptPublishFailed(ctx context.Context, attemptID uuid.UUID, reason string) error {
	return db.New(s.pool).UpdateMachineCommandAttemptPublishFailed(ctx, db.UpdateMachineCommandAttemptPublishFailedParams{
		ID:            attemptID,
		TimeoutReason: reason,
	})
}
