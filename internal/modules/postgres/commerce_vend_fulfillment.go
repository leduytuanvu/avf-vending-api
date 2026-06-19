package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// FulfillSuccessfulVendAtomically completes a captured payment vend in one transaction: optional order repair,
// inventory decrement idempotently, and timeline event (unless full replay).
func (s *Store) FulfillSuccessfulVendAtomically(ctx context.Context, in appcommerce.FulfillSuccessfulVendInput) (appcommerce.FulfillSuccessfulVendResult, error) {
	if s == nil || s.pool == nil {
		return appcommerce.FulfillSuccessfulVendResult{}, errors.New("postgres: nil store")
	}
	key := strings.TrimSpace(in.InventoryDedupeKey)
	if key == "" {
		return appcommerce.FulfillSuccessfulVendResult{}, errors.Join(appcommerce.ErrInvalidArgument, errors.New("inventory dedupe key is required"))
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return appcommerce.FulfillSuccessfulVendResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	ordRow, err := q.LockOrderByIDAndOrgForUpdate(ctx, in.OrderID)
	if err != nil {
		if isNoRows(err) {
			return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrNotFound
		}
		return appcommerce.FulfillSuccessfulVendResult{}, err
	}

	vendRow, err := q.LockVendSessionByOrderAndSlotForUpdate(ctx, db.LockVendSessionByOrderAndSlotForUpdateParams{
		OrderID:   in.OrderID,
		SlotIndex: in.SlotIndex,
	})
	if err != nil {
		if isNoRows(err) {
			return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrNotFound
		}
		return appcommerce.FulfillSuccessfulVendResult{}, err
	}
	if vendRow.MachineID != ordRow.MachineID {
		return appcommerce.FulfillSuccessfulVendResult{}, fmt.Errorf("postgres: vend row machine mismatch order")
	}
	if ordRow.Status == "cancelled" {
		return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrIllegalTransition
	}

	vStart := vendRow.State
	orderStartStatus := ordRow.Status

	switch vStart {
	case "failed", "pending":
		return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrIllegalTransition
	case "success", "in_progress":
	default:
		return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrIllegalTransition
	}

	payRow, err := q.GetLatestPaymentForOrder(ctx, in.OrderID)
	if err != nil {
		if isNoRows(err) {
			return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrPaymentNotSettled
		}
		return appcommerce.FulfillSuccessfulVendResult{}, err
	}

	finalOrd := ordRow
	finalVend := mapVendLockRow(vendRow)
	machineID := ordRow.MachineID
	prodID := vendRow.ProductID

	if vStart == "in_progress" {
		if payRow.State != "captured" {
			return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrPaymentNotSettled
		}
		nv, err := q.UpdateVendSessionStateByOrderSlot(ctx, db.UpdateVendSessionStateByOrderSlotParams{State: "success",
			FailureReason: pgtype.Text{},

			OrderID:   in.OrderID,
			SlotIndex: in.SlotIndex,
		})
		if err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		finalVend = mapVendUpdateRow(nv)

		or2, err := q.UpdateOrderStatusByOrg(ctx, db.UpdateOrderStatusByOrgParams{Status: "completed",

			ID: in.OrderID,
		})
		if err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		finalOrd = or2
	} else if vStart == "success" && ordRow.Status != "completed" {
		if payRow.State != "captured" {
			return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrPaymentNotSettled
		}
		or2, err := q.UpdateOrderStatusByOrg(ctx, db.UpdateOrderStatusByOrgParams{Status: "completed",

			ID: in.OrderID,
		})
		if err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		finalOrd = or2
	}

	invReplay, err := applyCommerceVendSuccessInventoryTx(ctx, q, uuid.Nil, machineID, in.OrderID, in.SlotIndex, prodID, key, in.CorrelationID)
	if err != nil {
		return appcommerce.FulfillSuccessfulVendResult{}, err
	}

	orderVendReplay := (vStart == "success" && orderStartStatus == "completed")

	verificationStatus := strings.TrimSpace(in.VerificationStatus)
	if verificationStatus == "" {
		verificationStatus = domaincommerce.VerificationUnverified
	}
	if !(orderVendReplay && invReplay) {
		if _, err := q.SetVendSessionVerificationStatus(ctx, db.SetVendSessionVerificationStatusParams{
			VerificationStatus: verificationStatus,
			OrderID:            in.OrderID,
			SlotIndex:          in.SlotIndex,
		}); err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		evidenceDedupe := key + ":hardware_evidence"
		if _, err := persistVendHardwareEvidenceTx(ctx, q, persistEvidenceInput{
			OrderID:       in.OrderID,
			VendSessionID: vendRow.ID,
			MachineID:     machineID,
			SlotIndex:     in.SlotIndex,
			Evidence:      in.Evidence,
			DedupeKey:     evidenceDedupe,
		}); err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		corrID := uuid.Nil
		if in.CorrelationID != nil {
			corrID = *in.CorrelationID
		}
		if err := emitVendSuccessFinancialLedgerTx(
			ctx,
			q,
			machineID,
			in.OrderID,
			payRow.ID,
			payRow.AmountMinor,
			payRow.Currency,
			corrID,
			key+":financial_ledger",
			"",
			"",
			in.SlotIndex,
			evidenceDedupe,
		); err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		payload, err := json.Marshal(map[string]any{
			"idempotency_key":     key,
			"inventory_replay":    invReplay,
			"order_vend_replay":   orderVendReplay,
			"machine_id":          machineID.String(),
			"slot_index":          in.SlotIndex,
			"verification_status": verificationStatus,
		})
		if err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		if err := q.InsertOrderTimelineEvent(ctx, db.InsertOrderTimelineEventParams{
			OrderID:    in.OrderID,
			EventType:  "commerce_vend_dispense_succeeded",
			ActorType:  "system",
			ActorID:    pgtype.Text{},
			Payload:    payload,
			OccurredAt: time.Now().UTC(),
		}); err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}

		outboxIdem := strings.TrimSpace(in.OutboxIdempotencyKey)
		if outboxIdem == "" {
			outboxIdem = key + ":vend_outbox"
		}
		obPayload, err := vendSuccessOutboxPayload(in.OrderID, in.SlotIndex, machineID, verificationStatus, in.Evidence, key)
		if err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		simRun := pgtypeTextString(ordRow.SimulationRunID)
		simScen := pgtypeTextString(ordRow.SimulationScenario)
		if err := insertOutboxEventIdempotent(ctx, q, insertOutboxParams{
			Topic:          in.OutboxTopic,
			EventType:      in.OutboxEventType,
			Payload:        obPayload,
			AggregateType:  in.OutboxAggregateType,
			AggregateID:    in.OrderID,
			IdempotencyKey: outboxIdem,
			Simulated:      ordRow.Simulated,
			SimulationRun:  simRun,
			SimulationScen: simScen,
		}); err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		if verificationStatus == domaincommerce.VerificationHardwareUnverified {
			reconType := strings.TrimSpace(in.ReconciliationEventType)
			if reconType != "" {
				reconPayload, err := json.Marshal(map[string]any{
					"order_id":            in.OrderID.String(),
					"slot_index":          in.SlotIndex,
					"verification_status": verificationStatus,
					"reason":              "hardware_evidence_missing_or_unverified",
				})
				if err != nil {
					return appcommerce.FulfillSuccessfulVendResult{}, err
				}
				if err := insertOutboxEventIdempotent(ctx, q, insertOutboxParams{
					Topic:          in.OutboxTopic,
					EventType:      reconType,
					Payload:        reconPayload,
					AggregateType:  in.OutboxAggregateType,
					AggregateID:    in.OrderID,
					IdempotencyKey: outboxIdem + ":reconciliation",
					Simulated:      ordRow.Simulated,
					SimulationRun:  simRun,
					SimulationScen: simScen,
				}); err != nil {
					return appcommerce.FulfillSuccessfulVendResult{}, err
				}
			}
			reconMeta, err := json.Marshal(map[string]any{
				"verification_status": verificationStatus,
				"slot_index":          in.SlotIndex,
			})
			if err != nil {
				return appcommerce.FulfillSuccessfulVendResult{}, err
			}
			if _, err := q.UpsertCommerceReconciliationCase(ctx, db.UpsertCommerceReconciliationCaseParams{
				CaseType:       "vend_started_no_terminal_ack",
				Severity:       "warning",
				Reason:         "hardware_evidence_missing_or_unverified",
				Metadata:       reconMeta,
				OrderID:        pgtype.UUID{Bytes: in.OrderID, Valid: true},
				VendSessionID:  pgtype.UUID{Bytes: vendRow.ID, Valid: true},
				MachineID:      pgtype.UUID{Bytes: machineID, Valid: true},
				CorrelationKey: pgtype.Text{String: key + ":reconciliation_case", Valid: true},
			}); err != nil {
				return appcommerce.FulfillSuccessfulVendResult{}, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return appcommerce.FulfillSuccessfulVendResult{}, err
	}

	return appcommerce.FulfillSuccessfulVendResult{
		Order:           mapOrder(finalOrd),
		Vend:            finalVend,
		InventoryReplay: invReplay,
		OrderVendReplay: orderVendReplay,
	}, nil
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func pgtypeTextString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return strings.TrimSpace(t.String)
}

// FulfillFailedVendAtomically records vend + order failure in one transaction and appends timeline when monetary compensation may be needed.
func (s *Store) FulfillFailedVendAtomically(ctx context.Context, in appcommerce.FulfillFailedVendInput) (appcommerce.FulfillFailedVendResult, error) {
	if s == nil || s.pool == nil {
		return appcommerce.FulfillFailedVendResult{}, errors.New("postgres: nil store")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	ordRow, err := q.LockOrderByIDAndOrgForUpdate(ctx, in.OrderID)
	if err != nil {
		if isNoRows(err) {
			return appcommerce.FulfillFailedVendResult{}, appcommerce.ErrNotFound
		}
		return appcommerce.FulfillFailedVendResult{}, err
	}

	vendRow, err := q.LockVendSessionByOrderAndSlotForUpdate(ctx, db.LockVendSessionByOrderAndSlotForUpdateParams{
		OrderID:   in.OrderID,
		SlotIndex: in.SlotIndex,
	})
	if err != nil {
		if isNoRows(err) {
			return appcommerce.FulfillFailedVendResult{}, appcommerce.ErrNotFound
		}
		return appcommerce.FulfillFailedVendResult{}, err
	}
	if vendRow.MachineID != ordRow.MachineID {
		return appcommerce.FulfillFailedVendResult{}, fmt.Errorf("postgres: vend row machine mismatch order")
	}

	if vendRow.State == "failed" && ordRow.Status == "failed" {
		if err := tx.Commit(ctx); err != nil {
			return appcommerce.FulfillFailedVendResult{}, err
		}
		return appcommerce.FulfillFailedVendResult{
			Order:  mapOrder(ordRow),
			Vend:   mapVendLockRow(vendRow),
			Replay: true,
		}, nil
	}

	if vendRow.State == "success" || vendRow.State == "pending" {
		return appcommerce.FulfillFailedVendResult{}, appcommerce.ErrIllegalTransition
	}
	if vendRow.State != "in_progress" {
		return appcommerce.FulfillFailedVendResult{}, appcommerce.ErrIllegalTransition
	}

	var fr pgtype.Text
	if in.FailureReason != nil && strings.TrimSpace(*in.FailureReason) != "" {
		fr = pgtype.Text{String: strings.TrimSpace(*in.FailureReason), Valid: true}
	}

	finalV, err := q.UpdateVendSessionStateByOrderSlot(ctx, db.UpdateVendSessionStateByOrderSlotParams{State: "failed",
		FailureReason: fr,

		OrderID:   in.OrderID,
		SlotIndex: in.SlotIndex,
	})
	if err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}

	finalOrd, err := q.UpdateOrderStatusByOrg(ctx, db.UpdateOrderStatusByOrgParams{Status: "failed",

		ID: in.OrderID,
	})
	if err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}

	// Persist failure-path hardware evidence + verification status (mirrors success). Evidence may be
	// nil on a legitimately unverified failure (persist no-ops); verification status is still recorded.
	verificationStatus := strings.TrimSpace(in.VerificationStatus)
	if verificationStatus == "" {
		verificationStatus = domaincommerce.VerificationUnverified
	}
	if _, err := q.SetVendSessionVerificationStatus(ctx, db.SetVendSessionVerificationStatusParams{
		VerificationStatus: verificationStatus,
		OrderID:            in.OrderID,
		SlotIndex:          in.SlotIndex,
	}); err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}
	evidenceDedupe := strings.TrimSpace(in.OutboxIdempotencyKey)
	if evidenceDedupe == "" {
		evidenceDedupe = fmt.Sprintf("commerce_vend_failed:%s|%d", in.OrderID.String(), in.SlotIndex)
	}
	evidenceDedupe += ":hardware_evidence"
	if _, err := persistVendHardwareEvidenceTx(ctx, q, persistEvidenceInput{
		OrderID:       in.OrderID,
		VendSessionID: vendRow.ID,
		MachineID:     ordRow.MachineID,
		SlotIndex:     in.SlotIndex,
		Evidence:      in.Evidence,
		DedupeKey:     evidenceDedupe,
	}); err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}

	payRow, payErr := q.GetLatestPaymentForOrder(ctx, in.OrderID)
	payCaptured := payErr == nil && (payRow.State == "captured" || payRow.State == "partially_refunded")
	cashLocal := false
	if payCaptured && payErr == nil {
		cashLocal = strings.EqualFold(strings.TrimSpace(payRow.Provider), "cash")
	}
	if payErr != nil && !isNoRows(payErr) {
		return appcommerce.FulfillFailedVendResult{}, payErr
	}
	refundRequired := in.RefundRequired
	localCashRefund := in.LocalCashRefundRequired
	if !in.RefundRequired && !in.LocalCashRefundRequired {
		refundRequired = payCaptured && !cashLocal
		localCashRefund = cashLocal && payCaptured
	}

	timelinePayload, err := json.Marshal(map[string]any{
		"failure_reason":                  derefStr(in.FailureReason),
		"refund_required":                 refundRequired,
		"local_cash_refund_required_hint": localCashRefund,
		"slot_index":                      in.SlotIndex,
	})
	if err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}
	if err := q.InsertOrderTimelineEvent(ctx, db.InsertOrderTimelineEventParams{
		OrderID:    in.OrderID,
		EventType:  "commerce_vend_dispense_failed",
		ActorType:  "system",
		ActorID:    pgtype.Text{},
		Payload:    timelinePayload,
		OccurredAt: time.Now().UTC(),
	}); err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}

	outboxIdem := strings.TrimSpace(in.OutboxIdempotencyKey)
	if outboxIdem == "" {
		outboxIdem = fmt.Sprintf("commerce_vend_failed:%s|%d", in.OrderID.String(), in.SlotIndex)
	}
	failPayload, err := vendFailedOutboxPayload(in.OrderID, in.SlotIndex, derefStr(in.FailureReason), refundRequired, localCashRefund)
	if err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}
	simRun := pgtypeTextString(ordRow.SimulationRunID)
	simScen := pgtypeTextString(ordRow.SimulationScenario)
	if err := insertOutboxEventIdempotent(ctx, q, insertOutboxParams{
		Topic:          in.OutboxTopic,
		EventType:      in.OutboxEventType,
		Payload:        failPayload,
		AggregateType:  in.OutboxAggregateType,
		AggregateID:    in.OrderID,
		IdempotencyKey: outboxIdem,
		Simulated:      ordRow.Simulated,
		SimulationRun:  simRun,
		SimulationScen: simScen,
	}); err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}
	if refundRequired || localCashRefund {
		reconType := strings.TrimSpace(in.ReconciliationEventType)
		if reconType != "" {
			reconPayload, err := json.Marshal(map[string]any{
				"order_id":                        in.OrderID.String(),
				"slot_index":                      in.SlotIndex,
				"refund_required":                 refundRequired,
				"local_cash_refund_required_hint": localCashRefund,
			})
			if err != nil {
				return appcommerce.FulfillFailedVendResult{}, err
			}
			if err := insertOutboxEventIdempotent(ctx, q, insertOutboxParams{
				Topic:          in.OutboxTopic,
				EventType:      reconType,
				Payload:        reconPayload,
				AggregateType:  in.OutboxAggregateType,
				AggregateID:    in.OrderID,
				IdempotencyKey: outboxIdem + ":reconciliation",
				Simulated:      ordRow.Simulated,
				SimulationRun:  simRun,
				SimulationScen: simScen,
			}); err != nil {
				return appcommerce.FulfillFailedVendResult{}, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}

	return appcommerce.FulfillFailedVendResult{
		Order:  mapOrder(finalOrd),
		Vend:   mapVendUpdateRow(finalV),
		Replay: false,
	}, nil
}
