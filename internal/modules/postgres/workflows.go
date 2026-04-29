package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/observability/mqttprom"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CommandWorkflowAudit is optional persistence-side audit metadata (callers supply org + actor context).
type CommandWorkflowAudit struct {
	OrganizationID uuid.UUID
	ActorType      string
	ActorID        string
	Action         string
	ResourceType   string
	Payload        []byte
	IP             *string
}

// AppendCommandWithOutboxInput binds command ledger + desired shadow + durable outbox emission in one transaction.
type AppendCommandWithOutboxInput struct {
	Command device.AppendCommandInput

	OrganizationID       uuid.UUID
	OutboxTopic          string
	OutboxEventType      string
	OutboxPayload        []byte
	OutboxAggregateType  string
	OutboxAggregateID    uuid.UUID
	OutboxIdempotencyKey string

	Audit *CommandWorkflowAudit
}

// AppendCommandWithOutboxResult is the transactional outcome for AppendCommandUpdateShadowAndOutbox.
type AppendCommandWithOutboxResult struct {
	CommandReplay bool
	Sequence      int64
	Outbox        commerce.OutboxEvent
	OutboxReplay  bool
}

// CommandReceiptTransitionParams applies a command receipt row with optional reported shadow and connectivity bump.
type CommandReceiptTransitionParams struct {
	MachineID          uuid.UUID
	Sequence           int64
	Status             string
	CorrelationID      *uuid.UUID
	Payload            []byte
	DedupeKey          string
	CommandID          uuid.UUID
	OccurredAt         time.Time
	CommandAttemptID   *uuid.UUID
	ReportedShadowJSON []byte

	Audit *CommandWorkflowAudit
}

// CommandReceiptTransitionResult indicates whether the receipt insert was skipped due to dedupe.
type CommandReceiptTransitionResult struct {
	ReceiptReplay   bool
	IgnoredConflict bool
}

// CreateOrderWithVendSession inserts an order and its first vend session in one transaction.
// It is idempotent on (organization_id, idempotency_key) for orders and will return the existing pair when replayed.
func (s *Store) CreateOrderWithVendSession(ctx context.Context, in commerce.CreateOrderVendInput) (commerce.CreateOrderVendResult, error) {
	if in.IdempotencyKey == "" {
		return commerce.CreateOrderVendResult{}, errors.New("postgres: idempotency_key is required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return commerce.CreateOrderVendResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	var orderRow db.Order
	var orderInserted bool
	existingOrder, err := q.GetOrderByOrgIdempotency(ctx, db.GetOrderByOrgIdempotencyParams{
		OrganizationID: in.OrganizationID,
		IdempotencyKey: optionalStringToPgText(in.IdempotencyKey),
	})
	switch {
	case err == nil:
		orderRow = existingOrder
		orderInserted = false
	case isNoRows(err):
		orderRow, err = q.InsertOrder(ctx, db.InsertOrderParams{
			OrganizationID: in.OrganizationID,
			MachineID:      in.MachineID,
			Status:         in.OrderStatus,
			Currency:       in.Currency,
			SubtotalMinor:  in.SubtotalMinor,
			TaxMinor:       in.TaxMinor,
			TotalMinor:     in.TotalMinor,
			IdempotencyKey: optionalStringToPgText(in.IdempotencyKey),
		})
		if err != nil {
			if isUniqueViolation(err) {
				return commerce.CreateOrderVendResult{}, errors.New("postgres: duplicate order idempotency key race")
			}
			return commerce.CreateOrderVendResult{}, err
		}
		orderInserted = true
	default:
		return commerce.CreateOrderVendResult{}, err
	}

	if !orderInserted {
		firstVend, fvErr := q.GetFirstVendSessionByOrder(ctx, orderRow.ID)
		if fvErr == nil {
			if err := tx.Commit(ctx); err != nil {
				return commerce.CreateOrderVendResult{}, err
			}
			return commerce.CreateOrderVendResult{
				Order:  mapOrder(orderRow),
				Vend:   mapVend(firstVend),
				Replay: true,
			}, nil
		}
		if !isNoRows(fvErr) {
			return commerce.CreateOrderVendResult{}, fvErr
		}
	}

	vendRow, vErr := q.GetVendSessionByOrderAndSlot(ctx, db.GetVendSessionByOrderAndSlotParams{
		OrderID:   orderRow.ID,
		SlotIndex: in.SlotIndex,
	})
	if vErr == nil {
		if err := tx.Commit(ctx); err != nil {
			return commerce.CreateOrderVendResult{}, err
		}
		return commerce.CreateOrderVendResult{
			Order:  mapOrder(orderRow),
			Vend:   mapVend(vendRow),
			Replay: true,
		}, nil
	}
	if !isNoRows(vErr) {
		return commerce.CreateOrderVendResult{}, vErr
	}

	vRow, err := q.InsertVendSession(ctx, db.InsertVendSessionParams{
		OrderID:   orderRow.ID,
		MachineID: in.MachineID,
		SlotIndex: in.SlotIndex,
		ProductID: in.ProductID,
		State:     in.VendState,
	})
	if err != nil {
		return commerce.CreateOrderVendResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return commerce.CreateOrderVendResult{}, err
	}

	return commerce.CreateOrderVendResult{
		Order:  mapOrder(orderRow),
		Vend:   mapVend(vRow),
		Replay: false,
	}, nil
}

// TryReplayCreateOrderWithVend implements commerce.OrderVendWorkflow.
func (s *Store) TryReplayCreateOrderWithVend(ctx context.Context, organizationID uuid.UUID, idempotencyKey string) (commerce.CreateOrderVendResult, bool, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return commerce.CreateOrderVendResult{}, false, nil
	}
	q := db.New(s.pool)
	orderRow, err := q.GetOrderByOrgIdempotency(ctx, db.GetOrderByOrgIdempotencyParams{
		OrganizationID: organizationID,
		IdempotencyKey: optionalStringToPgText(key),
	})
	if err != nil {
		if isNoRows(err) {
			return commerce.CreateOrderVendResult{}, false, nil
		}
		return commerce.CreateOrderVendResult{}, false, err
	}
	vendRow, err := q.GetFirstVendSessionByOrder(ctx, orderRow.ID)
	if err != nil {
		if isNoRows(err) {
			return commerce.CreateOrderVendResult{}, false, errors.New("postgres: idempotent order exists without vend session")
		}
		return commerce.CreateOrderVendResult{}, false, err
	}
	return commerce.CreateOrderVendResult{
		Order:  mapOrder(orderRow),
		Vend:   mapVend(vendRow),
		Replay: true,
	}, true, nil
}

// CreatePaymentWithOutbox inserts a payment and an outbox event in one transaction.
// It is idempotent on payment (order_id, idempotency_key) and outbox (topic, idempotency_key).
func (s *Store) CreatePaymentWithOutbox(ctx context.Context, in commerce.PaymentOutboxInput) (commerce.PaymentOutboxResult, error) {
	if in.IdempotencyKey == "" || in.OutboxIdempotencyKey == "" {
		return commerce.PaymentOutboxResult{}, errors.New("postgres: idempotency keys are required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return commerce.PaymentOutboxResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	existingPay, err := q.GetPaymentByOrderAndIdempotencyKey(ctx, db.GetPaymentByOrderAndIdempotencyKeyParams{
		OrderID:        in.OrderID,
		IdempotencyKey: optionalStringToPgText(in.IdempotencyKey),
	})
	if err == nil {
		ob, oErr := q.GetOutboxByTopicAndIdempotency(ctx, db.GetOutboxByTopicAndIdempotencyParams{
			Topic:          in.OutboxTopic,
			IdempotencyKey: optionalStringToPgText(in.OutboxIdempotencyKey),
		})
		if oErr == nil {
			if err := tx.Commit(ctx); err != nil {
				return commerce.PaymentOutboxResult{}, err
			}
			return commerce.PaymentOutboxResult{
				Payment: mapPayment(existingPay),
				Outbox:  mapOutbox(ob),
				Replay:  true,
			}, nil
		}
		if !isNoRows(oErr) {
			return commerce.PaymentOutboxResult{}, oErr
		}

		obRow, insErr := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
			OrganizationID: uuidToPg(in.OrganizationID),
			Topic:          in.OutboxTopic,
			EventType:      in.OutboxEventType,
			Payload:        in.OutboxPayload,
			AggregateType:  in.OutboxAggregateType,
			AggregateID:    in.OutboxAggregateID,
			IdempotencyKey: optionalStringToPgText(in.OutboxIdempotencyKey),
		})
		if insErr != nil {
			return commerce.PaymentOutboxResult{}, insErr
		}
		if err := tx.Commit(ctx); err != nil {
			return commerce.PaymentOutboxResult{}, err
		}
		return commerce.PaymentOutboxResult{
			Payment: mapPayment(existingPay),
			Outbox:  mapOutbox(obRow),
			Replay:  true,
		}, nil
	}
	if !isNoRows(err) {
		return commerce.PaymentOutboxResult{}, err
	}

	pRow, err := q.InsertPayment(ctx, db.InsertPaymentParams{
		OrderID:        in.OrderID,
		Provider:       in.Provider,
		State:          in.PaymentState,
		AmountMinor:    in.AmountMinor,
		Currency:       in.Currency,
		IdempotencyKey: optionalStringToPgText(in.IdempotencyKey),
	})
	if err != nil {
		return commerce.PaymentOutboxResult{}, err
	}

	obRow, err := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		OrganizationID: uuidToPg(in.OrganizationID),
		Topic:          in.OutboxTopic,
		EventType:      in.OutboxEventType,
		Payload:        in.OutboxPayload,
		AggregateType:  in.OutboxAggregateType,
		AggregateID:    in.OutboxAggregateID,
		IdempotencyKey: optionalStringToPgText(in.OutboxIdempotencyKey),
	})
	if err != nil {
		return commerce.PaymentOutboxResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return commerce.PaymentOutboxResult{}, err
	}

	return commerce.PaymentOutboxResult{
		Payment: mapPayment(pRow),
		Outbox:  mapOutbox(obRow),
		Replay:  false,
	}, nil
}

func mapDeviceReceiptToAttemptStatus(receiptStatus string) string {
	switch strings.ToLower(strings.TrimSpace(receiptStatus)) {
	case "acked":
		return "completed"
	case "nacked":
		return "nack"
	case "failed":
		return "failed"
	case "timeout":
		return "ack_timeout"
	default:
		return ""
	}
}

func isTerminalMachineAttemptStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "nack", "failed", "ack_timeout", "expired", "duplicate", "late":
		return true
	default:
		return false
	}
}

func auditPayloadBytes(p []byte) []byte {
	if len(p) == 0 {
		return []byte("{}")
	}
	return p
}

func enrichCommandReceiptPayload(raw []byte, occurredAt time.Time) []byte {
	if occurredAt.IsZero() {
		if len(raw) == 0 {
			return []byte("{}")
		}
		return raw
	}
	var m map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	m["occurred_at"] = occurredAt.UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(m)
	if err != nil {
		if len(raw) == 0 {
			return []byte("{}")
		}
		return raw
	}
	return b
}

// maybeInsertAuditLog appends an audit row when audit metadata is present (best-effort correlation for workflows).
func maybeInsertAuditLog(ctx context.Context, q *db.Queries, audit *CommandWorkflowAudit, resourceID uuid.UUID) error {
	if audit == nil || audit.OrganizationID == uuid.Nil || audit.Action == "" {
		return nil
	}
	_, err := q.InsertAuditLog(ctx, db.InsertAuditLogParams{
		OrganizationID: audit.OrganizationID,
		ActorType:      audit.ActorType,
		ActorID:        audit.ActorID,
		Action:         audit.Action,
		ResourceType:   audit.ResourceType,
		ResourceID:     optionalUUIDToPg(&resourceID),
		Payload:        auditPayloadBytes(audit.Payload),
		Ip:             optionalStringPtrToPgText(audit.IP),
	})
	return err
}

// AppendCommandUpdateShadow bumps command_sequence, inserts command_ledger, and upserts machine_shadow.
// It is idempotent on (machine_id, idempotency_key) for the ledger.
// The machine row is locked for update so concurrent writers serialize per device; command replays still refresh desired shadow.
func (s *Store) AppendCommandUpdateShadow(ctx context.Context, in device.AppendCommandInput) (device.AppendCommandResult, error) {
	if in.IdempotencyKey == "" {
		return device.AppendCommandResult{}, errors.New("postgres: idempotency_key is required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return device.AppendCommandResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	m, err := q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		return device.AppendCommandResult{}, err
	}

	idem := in.IdempotencyKey
	existing, err := q.GetCommandLedgerByMachineIdempotency(ctx, db.GetCommandLedgerByMachineIdempotencyParams{
		MachineID:      in.MachineID,
		IdempotencyKey: optionalStringToPgText(idem),
	})
	if err == nil {
		if _, err := q.UpsertMachineShadowDesired(ctx, db.UpsertMachineShadowDesiredParams{
			MachineID:    in.MachineID,
			DesiredState: in.DesiredState,
		}); err != nil {
			return device.AppendCommandResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return device.AppendCommandResult{}, err
		}
		return device.AppendCommandResult{CommandID: existing.ID, Sequence: existing.Sequence, Replay: true}, nil
	}
	if !isNoRows(err) {
		return device.AppendCommandResult{}, err
	}

	seq, err := q.BumpMachineCommandSequence(ctx, in.MachineID)
	if err != nil {
		return device.AppendCommandResult{}, err
	}

	cmdRow, err := q.InsertCommandLedgerEntry(ctx, db.InsertCommandLedgerEntryParams{
		MachineID:         in.MachineID,
		OrganizationID:    m.OrganizationID,
		Sequence:          seq,
		CommandType:       in.CommandType,
		Payload:           in.Payload,
		CorrelationID:     optionalUUIDToPg(in.CorrelationID),
		IdempotencyKey:    optionalStringToPgText(idem),
		OperatorSessionID: optionalUUIDToPg(in.OperatorSessionID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return device.AppendCommandResult{}, errors.New("postgres: duplicate command idempotency race")
		}
		return device.AppendCommandResult{}, err
	}

	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         in.MachineID,
		OrganizationID:    m.OrganizationID,
		OperatorSessionID: in.OperatorSessionID,
		ActionDomain:      "commands",
		ActionType:        in.CommandType,
		ResourceTable:     "command_ledger",
		ResourceID:        cmdRow.ID.String(),
		CorrelationID:     in.CorrelationID,
		OccurredAt:        &cmdRow.CreatedAt,
	}); err != nil {
		return device.AppendCommandResult{}, err
	}

	if _, err := q.UpsertMachineShadowDesired(ctx, db.UpsertMachineShadowDesiredParams{
		MachineID:    in.MachineID,
		DesiredState: in.DesiredState,
	}); err != nil {
		return device.AppendCommandResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return device.AppendCommandResult{}, err
	}
	productionmetrics.RecordCommandCreated()
	return device.AppendCommandResult{CommandID: cmdRow.ID, Sequence: seq, Replay: false}, nil
}

// AppendCommandUpdateShadowAndOutbox performs AppendCommandUpdateShadow work plus a durable outbox row in the same transaction.
// It repairs missing outbox rows on command replay (idempotent command + outbox fan-out).
func (s *Store) AppendCommandUpdateShadowAndOutbox(ctx context.Context, in AppendCommandWithOutboxInput) (AppendCommandWithOutboxResult, error) {
	if in.Command.IdempotencyKey == "" {
		return AppendCommandWithOutboxResult{}, errors.New("postgres: command idempotency_key is required")
	}
	if in.OutboxTopic == "" || in.OutboxIdempotencyKey == "" {
		return AppendCommandWithOutboxResult{}, errors.New("postgres: outbox topic and idempotency_key are required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AppendCommandWithOutboxResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	m, err := q.GetMachineByIDForUpdate(ctx, in.Command.MachineID)
	if err != nil {
		return AppendCommandWithOutboxResult{}, err
	}

	idem := in.Command.IdempotencyKey
	existing, err := q.GetCommandLedgerByMachineIdempotency(ctx, db.GetCommandLedgerByMachineIdempotencyParams{
		MachineID:      in.Command.MachineID,
		IdempotencyKey: optionalStringToPgText(idem),
	})
	if err == nil {
		if _, err := q.UpsertMachineShadowDesired(ctx, db.UpsertMachineShadowDesiredParams{
			MachineID:    in.Command.MachineID,
			DesiredState: in.Command.DesiredState,
		}); err != nil {
			return AppendCommandWithOutboxResult{}, err
		}

		ob, oErr := q.GetOutboxByTopicAndIdempotency(ctx, db.GetOutboxByTopicAndIdempotencyParams{
			Topic:          in.OutboxTopic,
			IdempotencyKey: optionalStringToPgText(in.OutboxIdempotencyKey),
		})
		switch {
		case oErr == nil:
			if err := maybeInsertAuditLog(ctx, q, in.Audit, in.Command.MachineID); err != nil {
				return AppendCommandWithOutboxResult{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return AppendCommandWithOutboxResult{}, err
			}
			return AppendCommandWithOutboxResult{
				CommandReplay: true,
				Sequence:      existing.Sequence,
				Outbox:        mapOutbox(ob),
				OutboxReplay:  true,
			}, nil
		case isNoRows(oErr):
			obRow, insErr := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
				OrganizationID: uuidToPg(in.OrganizationID),
				Topic:          in.OutboxTopic,
				EventType:      in.OutboxEventType,
				Payload:        in.OutboxPayload,
				AggregateType:  in.OutboxAggregateType,
				AggregateID:    in.OutboxAggregateID,
				IdempotencyKey: optionalStringToPgText(in.OutboxIdempotencyKey),
			})
			if insErr != nil {
				return AppendCommandWithOutboxResult{}, insErr
			}
			if err := maybeInsertAuditLog(ctx, q, in.Audit, in.Command.MachineID); err != nil {
				return AppendCommandWithOutboxResult{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return AppendCommandWithOutboxResult{}, err
			}
			return AppendCommandWithOutboxResult{
				CommandReplay: true,
				Sequence:      existing.Sequence,
				Outbox:        mapOutbox(obRow),
				OutboxReplay:  false,
			}, nil
		default:
			return AppendCommandWithOutboxResult{}, oErr
		}
	}
	if !isNoRows(err) {
		return AppendCommandWithOutboxResult{}, err
	}

	seq, err := q.BumpMachineCommandSequence(ctx, in.Command.MachineID)
	if err != nil {
		return AppendCommandWithOutboxResult{}, err
	}

	cmdRow, err := q.InsertCommandLedgerEntry(ctx, db.InsertCommandLedgerEntryParams{
		MachineID:         in.Command.MachineID,
		OrganizationID:    m.OrganizationID,
		Sequence:          seq,
		CommandType:       in.Command.CommandType,
		Payload:           in.Command.Payload,
		CorrelationID:     optionalUUIDToPg(in.Command.CorrelationID),
		IdempotencyKey:    optionalStringToPgText(idem),
		OperatorSessionID: optionalUUIDToPg(in.Command.OperatorSessionID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return AppendCommandWithOutboxResult{}, errors.New("postgres: duplicate command idempotency race")
		}
		return AppendCommandWithOutboxResult{}, err
	}

	if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
		MachineID:         in.Command.MachineID,
		OrganizationID:    m.OrganizationID,
		OperatorSessionID: in.Command.OperatorSessionID,
		ActionDomain:      "commands",
		ActionType:        in.Command.CommandType,
		ResourceTable:     "command_ledger",
		ResourceID:        cmdRow.ID.String(),
		CorrelationID:     in.Command.CorrelationID,
		OccurredAt:        &cmdRow.CreatedAt,
	}); err != nil {
		return AppendCommandWithOutboxResult{}, err
	}

	if _, err := q.UpsertMachineShadowDesired(ctx, db.UpsertMachineShadowDesiredParams{
		MachineID:    in.Command.MachineID,
		DesiredState: in.Command.DesiredState,
	}); err != nil {
		return AppendCommandWithOutboxResult{}, err
	}

	obRow, err := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		OrganizationID: uuidToPg(in.OrganizationID),
		Topic:          in.OutboxTopic,
		EventType:      in.OutboxEventType,
		Payload:        in.OutboxPayload,
		AggregateType:  in.OutboxAggregateType,
		AggregateID:    in.OutboxAggregateID,
		IdempotencyKey: optionalStringToPgText(in.OutboxIdempotencyKey),
	})
	if err != nil {
		return AppendCommandWithOutboxResult{}, err
	}

	if err := maybeInsertAuditLog(ctx, q, in.Audit, in.Command.MachineID); err != nil {
		return AppendCommandWithOutboxResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AppendCommandWithOutboxResult{}, err
	}

	productionmetrics.RecordCommandCreated()
	return AppendCommandWithOutboxResult{
		CommandReplay: false,
		Sequence:      seq,
		Outbox:        mapOutbox(obRow),
		OutboxReplay:  false,
	}, nil
}

// ApplyCommandReceiptTransition inserts a command receipt, optionally updates reported shadow, bumps connectivity, and audits in one transaction.
// Duplicate dedupe_key rolls back and returns ReceiptReplay=true without mutating shadow (callers must use a fresh dedupe to attach new shadow data).
func (s *Store) ApplyCommandReceiptTransition(ctx context.Context, p CommandReceiptTransitionParams) (CommandReceiptTransitionResult, error) {
	if p.MachineID == uuid.Nil {
		return CommandReceiptTransitionResult{}, errors.New("postgres: machine_id is required")
	}
	if p.DedupeKey == "" {
		return CommandReceiptTransitionResult{}, errors.New("postgres: dedupe_key is required")
	}
	st, ok := platformmqtt.NormalizeReceiptStatus(p.Status)
	if !ok {
		return CommandReceiptTransitionResult{}, fmt.Errorf("postgres: invalid command receipt status %q", p.Status)
	}
	p.Status = st

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CommandReceiptTransitionResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	m, err := q.GetMachineByIDForUpdate(ctx, p.MachineID)
	if err != nil {
		return CommandReceiptTransitionResult{}, err
	}

	if _, err := q.GetDeviceCommandReceiptIDByDedupeKey(ctx, p.DedupeKey); err == nil {
		mqttprom.RecordCommandAckDuplicate()
		return CommandReceiptTransitionResult{ReceiptReplay: true}, nil
	} else if !isNoRows(err) {
		return CommandReceiptTransitionResult{}, err
	}

	cmdRow, err := q.GetCommandLedgerByMachineSequence(ctx, db.GetCommandLedgerByMachineSequenceParams{
		MachineID: p.MachineID,
		Sequence:  p.Sequence,
	})
	if err != nil {
		if isNoRows(err) {
			mqttprom.RecordCommandAckRejected("unknown_sequence")
			return CommandReceiptTransitionResult{}, fmt.Errorf("postgres: command receipt rejected: unknown sequence for machine")
		}
		return CommandReceiptTransitionResult{}, err
	}

	if p.CommandID == uuid.Nil {
		mqttprom.RecordCommandAckRejected("missing_command_id")
		return CommandReceiptTransitionResult{}, fmt.Errorf("postgres: command_id is required")
	}
	if p.CommandID != cmdRow.ID {
		mdMismatch, _ := json.Marshal(map[string]any{
			"machine_id":          p.MachineID.String(),
			"sequence":            p.Sequence,
			"incoming_command_id": p.CommandID.String(),
			"ledger_command_id":   cmdRow.ID.String(),
		})
		machinePG := pgtype.UUID{Bytes: p.MachineID, Valid: true}
		sitePG := pgtype.UUID{}
		if m.SiteID != uuid.Nil {
			sitePG = pgtype.UUID{Bytes: m.SiteID, Valid: true}
		}
		if _, aerr := q.EnterpriseAuditInsertEvent(ctx, db.EnterpriseAuditInsertEventParams{
			OrganizationID: m.OrganizationID,
			ActorType:      "machine",
			ActorID:        pgtype.Text{String: p.MachineID.String(), Valid: true},
			Action:         "mqtt.command_ack_command_id_mismatch",
			ResourceType:   "command_ledger",
			ResourceID:     pgtype.Text{String: cmdRow.ID.String(), Valid: true},
			MachineID:      machinePG,
			SiteID:         sitePG,
			Metadata:       mdMismatch,
			Outcome:        "failure",
			OccurredAt:     pgtype.Timestamptz{},
		}); aerr != nil {
			return CommandReceiptTransitionResult{}, aerr
		}
		mqttprom.RecordCommandAckRejected("command_id_mismatch")
		return CommandReceiptTransitionResult{}, fmt.Errorf("postgres: command receipt rejected: command_id mismatch")
	}
	if cmdRow.OrganizationID != m.OrganizationID {
		mqttprom.RecordCommandAckRejected("ledger_organization_mismatch")
		return CommandReceiptTransitionResult{}, fmt.Errorf("postgres: command receipt rejected: ledger organization mismatch")
	}

	openAtt, openErr := q.GetLatestOpenMachineCommandAttemptForCommand(ctx, cmdRow.ID)
	var att db.MachineCommandAttempt
	hasAttempt := false
	if openErr == nil {
		att = openAtt
		hasAttempt = true
	} else if isNoRows(openErr) {
		lateAtt, aErr := q.GetLatestMachineCommandAttemptByCommandID(ctx, cmdRow.ID)
		if aErr == nil {
			att = lateAtt
			hasAttempt = true
		} else if !isNoRows(aErr) {
			return CommandReceiptTransitionResult{}, aErr
		}
	} else {
		return CommandReceiptTransitionResult{}, openErr
	}

	now := time.Now().UTC()

	if hasAttempt {
		if att.MachineID != p.MachineID {
			mqttprom.RecordCommandAckRejected("attempt_machine_mismatch")
			return CommandReceiptTransitionResult{}, errors.New("postgres: command attempt does not belong to machine")
		}
		mappedAttempt := mapDeviceReceiptToAttemptStatus(p.Status)
		wantsSuccess := p.Status == "acked" && mappedAttempt == "completed"

		if wantsSuccess {
			if cmdRow.TimeoutAt.Valid && now.After(cmdRow.TimeoutAt.Time) {
				mqttprom.RecordCommandAckRejected("ledger_timeout")
				mqttprom.RecordCommandAckTimeout("ledger_timeout")
				return CommandReceiptTransitionResult{}, fmt.Errorf("postgres: command receipt rejected: ledger timeout exceeded")
			}
			if att.Status == "sent" && att.AckDeadlineAt.Valid && now.After(att.AckDeadlineAt.Time) {
				mqttprom.RecordCommandAckRejected("ack_deadline_exceeded")
				mqttprom.RecordCommandAckTimeout("ack_deadline_exceeded")
				return CommandReceiptTransitionResult{}, fmt.Errorf("postgres: command receipt rejected: ack deadline exceeded")
			}
			if att.Status == "expired" || att.Status == "ack_timeout" {
				mqttprom.RecordCommandAckRejected("attempt_expired")
				mqttprom.RecordCommandAckTimeout("attempt_expired")
				return CommandReceiptTransitionResult{}, fmt.Errorf("postgres: command receipt rejected: attempt no longer accepts success")
			}
		}
	}

	if hasAttempt && isTerminalMachineAttemptStatus(att.Status) {
		latestRec, lrErr := q.GetLatestDeviceCommandReceiptByMachineSequence(ctx, db.GetLatestDeviceCommandReceiptByMachineSequenceParams{
			MachineID: p.MachineID,
			Sequence:  p.Sequence,
		})
		if lrErr == nil {
			if latestRec.Status == p.Status {
				mqttprom.RecordCommandAckDuplicate()
				return CommandReceiptTransitionResult{ReceiptReplay: true}, nil
			}
			md, _ := json.Marshal(map[string]any{
				"machine_id":                p.MachineID.String(),
				"sequence":                  p.Sequence,
				"incoming_status":           p.Status,
				"latest_receipt_status":     latestRec.Status,
				"attempt_status":            att.Status,
				"incoming_dedupe_key":       p.DedupeKey,
				"latest_receipt_dedupe_key": latestRec.DedupeKey,
			})
			machinePG := pgtype.UUID{Bytes: p.MachineID, Valid: true}
			sitePG := pgtype.UUID{}
			if m.SiteID != uuid.Nil {
				sitePG = pgtype.UUID{Bytes: m.SiteID, Valid: true}
			}
			if _, aerr := q.EnterpriseAuditInsertEvent(ctx, db.EnterpriseAuditInsertEventParams{
				OrganizationID: m.OrganizationID,
				ActorType:      "machine",
				ActorID:        pgtype.Text{String: p.MachineID.String(), Valid: true},
				Action:         "mqtt.command_ack_conflict",
				ResourceType:   "command_ledger",
				ResourceID:     pgtype.Text{String: cmdRow.ID.String(), Valid: true},
				MachineID:      machinePG,
				SiteID:         sitePG,
				Metadata:       md,
				Outcome:        "success",
				OccurredAt:     pgtype.Timestamptz{},
			}); aerr != nil {
				return CommandReceiptTransitionResult{}, aerr
			}
			mqttprom.RecordCommandAckConflict()
			if err := tx.Commit(ctx); err != nil {
				return CommandReceiptTransitionResult{}, err
			}
			return CommandReceiptTransitionResult{IgnoredConflict: true}, nil
		}
		if !isNoRows(lrErr) {
			return CommandReceiptTransitionResult{}, lrErr
		}
	}

	payload := enrichCommandReceiptPayload(p.Payload, p.OccurredAt)
	if len(payload) == 0 {
		payload = []byte("{}")
	}

	var cmdAttempt pgtype.UUID
	if hasAttempt {
		cmdAttempt = uuidToPg(att.ID)
	}
	_, err = q.InsertDeviceCommandReceipt(ctx, db.InsertDeviceCommandReceiptParams{
		MachineID:        p.MachineID,
		Sequence:         p.Sequence,
		Status:           p.Status,
		CorrelationID:    optionalUUIDToPg(p.CorrelationID),
		Payload:          payload,
		DedupeKey:        p.DedupeKey,
		CommandAttemptID: cmdAttempt,
	})
	if err != nil {
		if isUniqueViolation(err) {
			mqttprom.RecordCommandAckDuplicate()
			return CommandReceiptTransitionResult{ReceiptReplay: true}, nil
		}
		return CommandReceiptTransitionResult{}, err
	}

	if hasAttempt {
		prevAttemptStatus := att.Status
		if newStatus := mapDeviceReceiptToAttemptStatus(p.Status); newStatus != "" {
			if uErr := q.UpdateMachineCommandAttemptAfterDeviceReceipt(ctx, db.UpdateMachineCommandAttemptAfterDeviceReceiptParams{
				ID:     att.ID,
				Status: newStatus,
			}); uErr != nil {
				return CommandReceiptTransitionResult{}, uErr
			}
			if newStatus == "completed" && prevAttemptStatus == "sent" {
				mqttprom.ObserveCommandAckLatency(now.Sub(att.SentAt))
			}
		}
	}

	if len(p.ReportedShadowJSON) > 0 {
		if _, err := q.UpsertMachineShadowReported(ctx, db.UpsertMachineShadowReportedParams{
			MachineID:     p.MachineID,
			ReportedState: p.ReportedShadowJSON,
		}); err != nil {
			return CommandReceiptTransitionResult{}, err
		}
	}

	if err := q.TouchMachineConnectivity(ctx, p.MachineID); err != nil {
		return CommandReceiptTransitionResult{}, err
	}

	if err := maybeInsertAuditLog(ctx, q, p.Audit, p.MachineID); err != nil {
		return CommandReceiptTransitionResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CommandReceiptTransitionResult{}, err
	}

	return CommandReceiptTransitionResult{ReceiptReplay: false}, nil
}

var _ platformmqtt.DeviceIngest = (*Store)(nil)

// IngestTelemetry persists a telemetry row and bumps machine connectivity (idempotent on dedupe_key).
func (s *Store) IngestTelemetry(ctx context.Context, in platformmqtt.TelemetryIngest) error {
	if in.MachineID == uuid.Nil {
		return errors.New("postgres: machine_id is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	if _, err := q.GetMachineByIDForUpdate(ctx, in.MachineID); err != nil {
		return err
	}
	_, err = q.InsertDeviceTelemetryEvent(ctx, db.InsertDeviceTelemetryEventParams{
		MachineID: in.MachineID,
		EventType: in.EventType,
		Payload:   in.Payload,
		DedupeKey: optionalStringPtrToPgText(in.DedupeKey),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return err
	}
	if err := q.TouchMachineConnectivity(ctx, in.MachineID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IngestShadowReported merges reported_state into machine_shadow and bumps connectivity.
func (s *Store) IngestShadowReported(ctx context.Context, in platformmqtt.ShadowReportedIngest) error {
	if in.MachineID == uuid.Nil {
		return errors.New("postgres: machine_id is required")
	}
	if len(in.ReportedJSON) == 0 {
		return errors.New("postgres: reported json is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	if _, err := q.GetMachineByIDForUpdate(ctx, in.MachineID); err != nil {
		return err
	}
	if _, err := q.UpsertMachineShadowReported(ctx, db.UpsertMachineShadowReportedParams{
		MachineID:     in.MachineID,
		ReportedState: in.ReportedJSON,
	}); err != nil {
		return err
	}
	if err := q.TouchMachineConnectivity(ctx, in.MachineID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IngestShadowDesired merges desired_state into machine_shadow and bumps connectivity (MQTT shadow/desired).
func (s *Store) IngestShadowDesired(ctx context.Context, in platformmqtt.ShadowDesiredIngest) error {
	if in.MachineID == uuid.Nil {
		return errors.New("postgres: machine_id is required")
	}
	if len(in.DesiredJSON) == 0 {
		return errors.New("postgres: desired json is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	if _, err := q.GetMachineByIDForUpdate(ctx, in.MachineID); err != nil {
		return err
	}
	if _, err := q.UpsertMachineShadowDesired(ctx, db.UpsertMachineShadowDesiredParams{
		MachineID:    in.MachineID,
		DesiredState: in.DesiredJSON,
	}); err != nil {
		return err
	}
	if err := q.TouchMachineConnectivity(ctx, in.MachineID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IngestCommandReceipt records an edge command outcome and bumps connectivity (idempotent on dedupe_key).
func (s *Store) IngestCommandReceipt(ctx context.Context, in platformmqtt.CommandReceiptIngest) error {
	if in.MachineID == uuid.Nil {
		return errors.New("postgres: machine_id is required")
	}
	if in.DedupeKey == "" {
		return errors.New("postgres: dedupe_key is required")
	}
	_, err := s.ApplyCommandReceiptTransition(ctx, CommandReceiptTransitionParams{
		MachineID:     in.MachineID,
		Sequence:      in.Sequence,
		Status:        in.Status,
		CorrelationID: in.CorrelationID,
		Payload:       in.Payload,
		DedupeKey:     in.DedupeKey,
		CommandID:     in.CommandID,
		OccurredAt:    in.OccurredAt,
	})
	return err
}
