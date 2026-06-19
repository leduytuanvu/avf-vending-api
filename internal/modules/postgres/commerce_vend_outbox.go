package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type insertOutboxParams struct {
	Topic          string
	EventType      string
	Payload        []byte
	AggregateType  string
	AggregateID    uuid.UUID
	IdempotencyKey string
	Simulated      bool
	SimulationRun  string
	SimulationScen string
}

func insertOutboxEventIdempotent(ctx context.Context, q *db.Queries, p insertOutboxParams) error {
	topic := strings.TrimSpace(p.Topic)
	evType := strings.TrimSpace(p.EventType)
	idem := strings.TrimSpace(p.IdempotencyKey)
	if topic == "" || evType == "" || idem == "" {
		return nil
	}
	row, err := q.InsertOutboxEventIdempotent(ctx, db.InsertOutboxEventIdempotentParams{
		Topic:              topic,
		EventType:          evType,
		Payload:            p.Payload,
		AggregateType:      strings.TrimSpace(p.AggregateType),
		AggregateID:        p.AggregateID,
		IdempotencyKey:     pgtype.Text{String: idem, Valid: true},
		Simulated:          p.Simulated,
		SimulationRunID:    pgTextOrEmpty(p.SimulationRun),
		SimulationScenario: pgTextOrEmpty(p.SimulationScen),
	})
	if err == nil {
		_ = row
		return nil
	}
	if !isNoRows(err) {
		return err
	}
	_, err = q.GetOutboxEventByTopicIdempotencyKey(ctx, db.GetOutboxEventByTopicIdempotencyKeyParams{
		Topic:          topic,
		IdempotencyKey: pgtype.Text{String: idem, Valid: true},
	})
	return err
}

func pgTextOrEmpty(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func persistVendHardwareEvidenceTx(ctx context.Context, q *db.Queries, in persistEvidenceInput) (inserted bool, err error) {
	if in.Evidence == nil {
		return false, nil
	}
	raw, err := json.Marshal(in.Evidence)
	if err != nil {
		return false, err
	}
	dedupe := strings.TrimSpace(in.DedupeKey)
	if dedupe == "" {
		return false, errors.New("postgres: evidence dedupe key required")
	}
	row, err := q.InsertVendHardwareEvidence(ctx, db.InsertVendHardwareEvidenceParams{
		OrderID:        in.OrderID,
		VendSessionID:  in.VendSessionID,
		MachineID:      in.MachineID,
		SlotIndex:      in.SlotIndex,
		VendAttemptID:  in.Evidence.VendAttemptID,
		CorrelationID:  in.Evidence.CorrelationID,
		CommandID:      strings.TrimSpace(in.Evidence.Command.CommandID),
		EvidenceDigest: in.Evidence.EvidenceDigest(),
		Raw:            raw,
		DedupeKey:      dedupe,
	})
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	_ = row
	return true, nil
}

type persistEvidenceInput struct {
	OrderID       uuid.UUID
	VendSessionID uuid.UUID
	MachineID     uuid.UUID
	SlotIndex     int32
	Evidence      *domaincommerce.VendHardwareEvidence
	DedupeKey     string
}

func vendSuccessOutboxPayload(orderID uuid.UUID, slotIndex int32, machineID uuid.UUID, verificationStatus string, evidence *domaincommerce.VendHardwareEvidence, idempotencyKey string) ([]byte, error) {
	m := map[string]any{
		"order_id":            orderID.String(),
		"slot_index":          slotIndex,
		"machine_id":          machineID.String(),
		"verification_status": verificationStatus,
		"idempotency_key":     idempotencyKey,
	}
	if evidence != nil {
		m["vend_attempt_id"] = evidence.VendAttemptID.String()
		m["correlation_id"] = evidence.CorrelationID.String()
		m["command_id"] = strings.TrimSpace(evidence.Command.CommandID)
		m["evidence_digest"] = evidence.EvidenceDigest()
	}
	return json.Marshal(m)
}

func vendFailedOutboxPayload(orderID uuid.UUID, slotIndex int32, failureReason string, refundRequired, localCashRefund bool) ([]byte, error) {
	return json.Marshal(map[string]any{
		"order_id":                        orderID.String(),
		"slot_index":                      slotIndex,
		"failure_reason":                  failureReason,
		"refund_required":                 refundRequired,
		"local_cash_refund_required_hint": localCashRefund,
	})
}
