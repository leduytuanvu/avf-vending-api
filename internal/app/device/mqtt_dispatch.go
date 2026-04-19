package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrMQTTCommandPublisherMissing means outbound MQTT is not wired for this API process.
var ErrMQTTCommandPublisherMissing = errors.New("device: MQTT command publisher is not configured")

// MQTTDispatchPublisher publishes JSON to a machine-specific MQTT dispatch topic.
type MQTTDispatchPublisher interface {
	PublishDeviceDispatch(ctx context.Context, machineID uuid.UUID, payload []byte) error
}

// MQTTCommandDispatcher persists command intent then publishes over MQTT (API process only).
type MQTTCommandDispatcher struct {
	wf                 domaindevice.CommandShadowWorkflow
	store              *postgres.Store
	pub                MQTTDispatchPublisher
	ackTimeout         time.Duration
	publishMaxAttempts int
	retryBackoff       time.Duration
}

// MQTTCommandDispatcherDeps wires the dispatcher.
type MQTTCommandDispatcherDeps struct {
	Workflow           domaindevice.CommandShadowWorkflow
	Store              *postgres.Store
	Publisher          MQTTDispatchPublisher
	AckTimeout         time.Duration
	PublishMaxAttempts int // total publish tries including the first; default 3 when <= 0
	RetryBackoff       time.Duration
}

// NewMQTTCommandDispatcher returns nil when store or workflow is nil. Publisher may be nil (Dispatch will error explicitly).
func NewMQTTCommandDispatcher(d MQTTCommandDispatcherDeps) *MQTTCommandDispatcher {
	if d.Store == nil || d.Workflow == nil {
		return nil
	}
	ack := d.AckTimeout
	if ack <= 0 {
		ack = 30 * time.Second
	}
	attempts := d.PublishMaxAttempts
	if attempts <= 0 {
		attempts = 3
	}
	backoff := d.RetryBackoff
	if backoff <= 0 {
		backoff = 150 * time.Millisecond
	}
	return &MQTTCommandDispatcher{
		wf:                 d.Workflow,
		store:              d.Store,
		pub:                d.Publisher,
		ackTimeout:         ack,
		publishMaxAttempts: attempts,
		retryBackoff:       backoff,
	}
}

// RemoteCommandDispatchInput is the application input for dispatching a remote command.
type RemoteCommandDispatchInput struct {
	Append domaindevice.AppendCommandInput
}

// RemoteCommandDispatchResult is returned after a dispatch attempt.
type RemoteCommandDispatchResult struct {
	CommandID        uuid.UUID `json:"command_id"`
	Sequence         int64     `json:"sequence"`
	AttemptID        uuid.UUID `json:"attempt_id"`
	Replay           bool      `json:"replay"`
	DispatchState    string    `json:"dispatch_state"`
	SkippedRepublish bool      `json:"skipped_republish,omitempty"`
}

type dispatchWire struct {
	Sequence       int64           `json:"sequence"`
	CommandType    string          `json:"command_type"`
	Payload        json.RawMessage `json:"payload"`
	CorrelationID  *uuid.UUID      `json:"correlation_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key"`
}

// MapAttemptTransportState maps machine_command_attempts.status to a stable API dispatch_state.
func MapAttemptTransportState(attemptStatus string) string {
	switch strings.TrimSpace(strings.ToLower(attemptStatus)) {
	case "pending":
		return "queued"
	case "sent":
		return "published"
	case "completed":
		return "acknowledged"
	case "failed", "nack":
		return "failed"
	case "ack_timeout":
		return "timed_out"
	case "duplicate", "late":
		return "superseded"
	default:
		return attemptStatus
	}
}

// DispatchRemoteMQTTCommand persists the command (unless replay), ensures a transport attempt row, then publishes.
func (d *MQTTCommandDispatcher) DispatchRemoteMQTTCommand(ctx context.Context, in RemoteCommandDispatchInput) (RemoteCommandDispatchResult, error) {
	if d == nil || d.store == nil || d.wf == nil {
		return RemoteCommandDispatchResult{}, errors.New("device: nil MQTT command dispatcher")
	}
	if d.pub == nil {
		return RemoteCommandDispatchResult{}, ErrMQTTCommandPublisherMissing
	}

	_ = d.store.ApplyMQTTCommandAckTimeouts(ctx, time.Now())

	appendIn := in.Append
	if len(appendIn.DesiredState) == 0 {
		appendIn.DesiredState = []byte("{}")
	}

	appendRes, err := d.wf.AppendCommandUpdateShadow(ctx, appendIn)
	if err != nil {
		return RemoteCommandDispatchResult{}, err
	}

	ledgerRow, err := d.store.GetCommandLedgerByMachineSequence(ctx, appendIn.MachineID, appendRes.Sequence)
	if err != nil {
		return RemoteCommandDispatchResult{}, err
	}

	payloadWire := ledgerRow.Payload
	if len(payloadWire) == 0 {
		payloadWire = []byte("{}")
	}

	wire := dispatchWire{
		Sequence:       appendRes.Sequence,
		CommandType:    ledgerRow.CommandType,
		Payload:        json.RawMessage(payloadWire),
		CorrelationID:  pgUUIDPtr(ledgerRow.CorrelationID),
		IdempotencyKey: pgTextString(ledgerRow.IdempotencyKey),
	}
	wireBytes, err := json.Marshal(wire)
	if err != nil {
		return RemoteCommandDispatchResult{}, fmt.Errorf("device: marshal dispatch wire: %w", err)
	}

	ledgerDeadline := time.Now().Add(d.ackTimeout)

	latest, err := d.store.GetLatestMachineCommandAttemptByCommandID(ctx, appendRes.CommandID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return RemoteCommandDispatchResult{}, err
	}

	var att db.MachineCommandAttempt
	skippedRepublish := false

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		att, err = d.store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, appendRes.CommandID, appendIn.MachineID, pgUUIDPtr(ledgerRow.CorrelationID), wireBytes, ledgerDeadline)
		if err != nil {
			return RemoteCommandDispatchResult{}, err
		}
	case latest.Status == "pending":
		att = latest
	case latest.Status == "sent":
		if ackAt := pgTimestamptzPtr(latest.AckDeadlineAt); ackAt != nil && time.Now().Before(*ackAt) {
			att = latest
			skippedRepublish = true
		} else {
			att, err = d.store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, appendRes.CommandID, appendIn.MachineID, pgUUIDPtr(ledgerRow.CorrelationID), wireBytes, ledgerDeadline)
			if err != nil {
				return RemoteCommandDispatchResult{}, err
			}
		}
	case latest.Status == "ack_timeout" && appendRes.Replay:
		att, err = d.store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, appendRes.CommandID, appendIn.MachineID, pgUUIDPtr(ledgerRow.CorrelationID), wireBytes, ledgerDeadline)
		if err != nil {
			return RemoteCommandDispatchResult{}, err
		}
	case latest.Status == "failed" && appendRes.Replay && isPublishFailure(latest):
		att, err = d.store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, appendRes.CommandID, appendIn.MachineID, pgUUIDPtr(ledgerRow.CorrelationID), wireBytes, ledgerDeadline)
		if err != nil {
			return RemoteCommandDispatchResult{}, err
		}
	case isTerminalAttemptStatus(latest.Status):
		att = latest
		skippedRepublish = true
	default:
		return RemoteCommandDispatchResult{}, fmt.Errorf("device: unsupported attempt status %q", latest.Status)
	}

	if skippedRepublish {
		return RemoteCommandDispatchResult{
			CommandID:        appendRes.CommandID,
			Sequence:         appendRes.Sequence,
			AttemptID:        att.ID,
			Replay:           appendRes.Replay,
			DispatchState:    MapAttemptTransportState(att.Status),
			SkippedRepublish: true,
		}, nil
	}

	var publishErr error
	for attempt := 0; attempt < d.publishMaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				publishErr = ctx.Err()
				break
			case <-time.After(d.retryBackoff):
			}
		}
		publishErr = d.pub.PublishDeviceDispatch(ctx, appendIn.MachineID, wireBytes)
		if publishErr == nil {
			break
		}
	}
	if publishErr != nil {
		_ = d.store.MarkMQTTDispatchAttemptPublishFailed(ctx, att.ID, "mqtt_publish: "+truncateErr(publishErr, 400))
		return RemoteCommandDispatchResult{}, fmt.Errorf("device: mqtt publish: %w", publishErr)
	}

	ackDeadline := time.Now().Add(d.ackTimeout)
	if err := d.store.MarkMQTTDispatchAttemptSent(ctx, att.ID, ackDeadline); err != nil {
		return RemoteCommandDispatchResult{}, err
	}

	return RemoteCommandDispatchResult{
		CommandID:     appendRes.CommandID,
		Sequence:      appendRes.Sequence,
		AttemptID:     att.ID,
		Replay:        appendRes.Replay,
		DispatchState: "published",
	}, nil
}

func isPublishFailure(a db.MachineCommandAttempt) bool {
	if !a.TimeoutReason.Valid {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(a.TimeoutReason.String)), "mqtt_publish")
}

func isTerminalAttemptStatus(s string) bool {
	switch s {
	case "completed", "nack", "failed", "ack_timeout", "duplicate", "late":
		return true
	default:
		return false
	}
}

func pgUUIDPtr(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	return &id
}

func pgTextString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func pgTextPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func pgTimestamptzPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	tt := ts.Time.UTC()
	return &tt
}

func truncateErr(err error, max int) string {
	s := err.Error()
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// RemoteCommandStatusView is a read model for GET status.
type RemoteCommandStatusView struct {
	MachineID     uuid.UUID `json:"machine_id"`
	CommandID     uuid.UUID `json:"command_id"`
	Sequence      int64     `json:"sequence"`
	CommandType   string    `json:"command_type"`
	DispatchState string    `json:"dispatch_state"`
	Attempt       *struct {
		ID            uuid.UUID  `json:"id"`
		AttemptNo     int32      `json:"attempt_no"`
		Status        string     `json:"status"`
		SentAt        time.Time  `json:"sent_at"`
		AckDeadlineAt *time.Time `json:"ack_deadline_at,omitempty"`
		ResultAt      *time.Time `json:"result_received_at,omitempty"`
		TimeoutReason *string    `json:"timeout_reason,omitempty"`
	} `json:"attempt,omitempty"`
}

// GetRemoteCommandStatus returns persisted command + latest attempt state.
func (d *MQTTCommandDispatcher) GetRemoteCommandStatus(ctx context.Context, machineID uuid.UUID, sequence int64) (RemoteCommandStatusView, error) {
	if d == nil || d.store == nil {
		return RemoteCommandStatusView{}, errors.New("device: nil MQTT command dispatcher")
	}
	ledger, err := d.store.GetCommandLedgerByMachineSequence(ctx, machineID, sequence)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RemoteCommandStatusView{}, ErrNotFound
		}
		return RemoteCommandStatusView{}, err
	}
	latest, err := d.store.GetLatestMachineCommandAttemptByCommandID(ctx, ledger.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return RemoteCommandStatusView{}, err
	}
	out := RemoteCommandStatusView{
		MachineID:     machineID,
		CommandID:     ledger.ID,
		Sequence:      ledger.Sequence,
		CommandType:   ledger.CommandType,
		DispatchState: "queued",
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	out.DispatchState = MapAttemptTransportState(latest.Status)
	out.Attempt = &struct {
		ID            uuid.UUID  `json:"id"`
		AttemptNo     int32      `json:"attempt_no"`
		Status        string     `json:"status"`
		SentAt        time.Time  `json:"sent_at"`
		AckDeadlineAt *time.Time `json:"ack_deadline_at,omitempty"`
		ResultAt      *time.Time `json:"result_received_at,omitempty"`
		TimeoutReason *string    `json:"timeout_reason,omitempty"`
	}{
		ID:            latest.ID,
		AttemptNo:     latest.AttemptNo,
		Status:        latest.Status,
		SentAt:        latest.SentAt,
		AckDeadlineAt: pgTimestamptzPtr(latest.AckDeadlineAt),
		ResultAt:      pgTimestamptzPtr(latest.ResultReceivedAt),
		TimeoutReason: pgTextPtr(latest.TimeoutReason),
	}
	return out, nil
}

// RemoteCommandReceiptView is a row in the receipts list API.
type RemoteCommandReceiptView struct {
	ID               int64      `json:"id"`
	Sequence         int64      `json:"sequence"`
	Status           string     `json:"status"`
	CorrelationID    *uuid.UUID `json:"correlation_id,omitempty"`
	DedupeKey        string     `json:"dedupe_key"`
	ReceivedAt       time.Time  `json:"received_at"`
	CommandAttemptID *uuid.UUID `json:"command_attempt_id,omitempty"`
}

// ListRecentCommandReceipts returns recent device command receipts for a machine.
func (d *MQTTCommandDispatcher) ListRecentCommandReceipts(ctx context.Context, machineID uuid.UUID, limit int32) ([]RemoteCommandReceiptView, error) {
	if d == nil || d.store == nil {
		return nil, errors.New("device: nil MQTT command dispatcher")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := d.store.ListDeviceCommandReceiptsByMachine(ctx, machineID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]RemoteCommandReceiptView, 0, len(rows))
	for _, r := range rows {
		out = append(out, RemoteCommandReceiptView{
			ID:               r.ID,
			Sequence:         r.Sequence,
			Status:           r.Status,
			CorrelationID:    pgUUIDPtr(r.CorrelationID),
			DedupeKey:        r.DedupeKey,
			ReceivedAt:       r.ReceivedAt,
			CommandAttemptID: pgUUIDPtr(r.CommandAttemptID),
		})
	}
	return out, nil
}
