package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
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

	ordRow, err := q.LockOrderByIDAndOrgForUpdate(ctx, db.LockOrderByIDAndOrgForUpdateParams{
		ID:             in.OrderID,
		OrganizationID: in.OrganizationID,
	})
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
	finalV := vendRow
	machineID := ordRow.MachineID
	prodID := vendRow.ProductID

	if vStart == "in_progress" {
		if payRow.State != "captured" {
			return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrPaymentNotSettled
		}
		nv, err := q.UpdateVendSessionStateByOrderSlot(ctx, db.UpdateVendSessionStateByOrderSlotParams{
			OrderID:       in.OrderID,
			SlotIndex:     in.SlotIndex,
			State:         "success",
			FailureReason: pgtype.Text{},
		})
		if err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		finalV = nv

		or2, err := q.UpdateOrderStatusByOrg(ctx, db.UpdateOrderStatusByOrgParams{
			ID:             in.OrderID,
			OrganizationID: in.OrganizationID,
			Status:         "completed",
		})
		if err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		finalOrd = or2
	} else if vStart == "success" && ordRow.Status != "completed" {
		if payRow.State != "captured" {
			return appcommerce.FulfillSuccessfulVendResult{}, appcommerce.ErrPaymentNotSettled
		}
		or2, err := q.UpdateOrderStatusByOrg(ctx, db.UpdateOrderStatusByOrgParams{
			ID:             in.OrderID,
			OrganizationID: in.OrganizationID,
			Status:         "completed",
		})
		if err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		finalOrd = or2
	}

	invReplay, err := applyCommerceVendSuccessInventoryTx(ctx, q, in.OrganizationID, machineID, in.OrderID, in.SlotIndex, prodID, key, in.CorrelationID)
	if err != nil {
		return appcommerce.FulfillSuccessfulVendResult{}, err
	}

	orderVendReplay := (vStart == "success" && orderStartStatus == "completed")

	if !(orderVendReplay && invReplay) {
		payload, err := json.Marshal(map[string]any{
			"idempotency_key":   key,
			"inventory_replay":  invReplay,
			"order_vend_replay": orderVendReplay,
			"machine_id":        machineID.String(),
			"slot_index":        in.SlotIndex,
		})
		if err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
		if err := q.InsertOrderTimelineEvent(ctx, db.InsertOrderTimelineEventParams{
			OrganizationID: in.OrganizationID,
			OrderID:        in.OrderID,
			EventType:      "commerce_vend_dispense_succeeded",
			ActorType:      "system",
			ActorID:        pgtype.Text{},
			Payload:        payload,
			OccurredAt:     time.Now().UTC(),
		}); err != nil {
			return appcommerce.FulfillSuccessfulVendResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return appcommerce.FulfillSuccessfulVendResult{}, err
	}

	return appcommerce.FulfillSuccessfulVendResult{
		Order:           mapOrder(finalOrd),
		Vend:            mapVend(finalV),
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

	ordRow, err := q.LockOrderByIDAndOrgForUpdate(ctx, db.LockOrderByIDAndOrgForUpdateParams{
		ID:             in.OrderID,
		OrganizationID: in.OrganizationID,
	})
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
			Vend:   mapVend(vendRow),
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

	finalV, err := q.UpdateVendSessionStateByOrderSlot(ctx, db.UpdateVendSessionStateByOrderSlotParams{
		OrderID:       in.OrderID,
		SlotIndex:     in.SlotIndex,
		State:         "failed",
		FailureReason: fr,
	})
	if err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}

	finalOrd, err := q.UpdateOrderStatusByOrg(ctx, db.UpdateOrderStatusByOrgParams{
		ID:             in.OrderID,
		OrganizationID: in.OrganizationID,
		Status:         "failed",
	})
	if err != nil {
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

	timelinePayload, err := json.Marshal(map[string]any{
		"failure_reason":                  derefStr(in.FailureReason),
		"refund_required":                 payCaptured && !cashLocal,
		"local_cash_refund_required_hint": cashLocal && payCaptured,
		"slot_index":                      in.SlotIndex,
	})
	if err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}
	if err := q.InsertOrderTimelineEvent(ctx, db.InsertOrderTimelineEventParams{
		OrganizationID: in.OrganizationID,
		OrderID:        in.OrderID,
		EventType:      "commerce_vend_dispense_failed",
		ActorType:      "system",
		ActorID:        pgtype.Text{},
		Payload:        timelinePayload,
		OccurredAt:     time.Now().UTC(),
	}); err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return appcommerce.FulfillFailedVendResult{}, err
	}

	return appcommerce.FulfillFailedVendResult{
		Order:  mapOrder(finalOrd),
		Vend:   mapVend(finalV),
		Replay: false,
	}, nil
}
