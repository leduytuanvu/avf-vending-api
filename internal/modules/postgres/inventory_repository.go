package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/inventoryapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InventoryRepository implements inventoryapp.LedgerRepository.
type InventoryRepository struct {
	pool *pgxpool.Pool
}

// NewInventoryRepository returns a Postgres-backed inventory ledger writer.
func NewInventoryRepository(pool *pgxpool.Pool) *InventoryRepository {
	if pool == nil {
		panic("postgres.NewInventoryRepository: nil pool")
	}
	return &InventoryRepository{pool: pool}
}

var _ inventoryapp.LedgerRepository = (*InventoryRepository)(nil)

func legacyPriceForSlot(rows []db.InventoryAdminListMachineSlotsRow, planogramID uuid.UUID, slotIndex int32) int64 {
	for _, snap := range rows {
		if snap.PlanogramID == planogramID && snap.SlotIndex == slotIndex {
			return snap.PriceMinor
		}
	}
	return 0
}

func legacyRevisionForSlot(rows []db.InventoryAdminListMachineSlotsRow, planogramID uuid.UUID, slotIndex int32) int32 {
	for _, snap := range rows {
		if snap.PlanogramID == planogramID && snap.SlotIndex == slotIndex {
			return snap.PlanogramRevisionApplied
		}
	}
	return 0
}

func resolveAdjustmentCabinetCode(it inventoryapp.AdjustmentItem, legacy []db.InventoryAdminListMachineSlotsRow) string {
	cc := strings.TrimSpace(it.CabinetCode)
	if cc != "" {
		return cc
	}
	for _, snap := range legacy {
		if snap.PlanogramID == it.PlanogramID && snap.SlotIndex == it.SlotIndex {
			if strings.TrimSpace(snap.CabinetCode) != "" {
				return strings.TrimSpace(snap.CabinetCode)
			}
			break
		}
	}
	// Align with inventory_admin slot resolution default for legacy-only machines.
	return "CAB-A"
}

// CreateInventoryAdjustmentBatch appends adjustment events and updates legacy machine_slot_state in one transaction.
func (r *InventoryRepository) CreateInventoryAdjustmentBatch(ctx context.Context, in inventoryapp.AdjustmentBatchInput) (inventoryapp.AdjustmentBatchResult, error) {
	if len(in.Items) == 0 {
		return inventoryapp.AdjustmentBatchResult{}, errors.New("postgres: adjustment items required")
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return inventoryapp.AdjustmentBatchResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	m, err := q.GetMachineByIDForUpdate(ctx, in.MachineID)
	if err != nil {
		if isNoRows(err) {
			return inventoryapp.AdjustmentBatchResult{}, errors.New("postgres: machine not found")
		}
		return inventoryapp.AdjustmentBatchResult{}, err
	}
	if m.OrganizationID != in.OrganizationID {
		return inventoryapp.AdjustmentBatchResult{}, errOrganizationMachineMismatch
	}

	key := strings.TrimSpace(in.IdempotencyKey)
	if key != "" {
		cnt, err := q.InventoryAdminCountInventoryEventsByIdempotencyKey(ctx, db.InventoryAdminCountInventoryEventsByIdempotencyKeyParams{
			MachineID: in.MachineID,
			Column2:   key,
		})
		if err != nil {
			return inventoryapp.AdjustmentBatchResult{}, err
		}
		if cnt > 0 {
			if err := tx.Commit(ctx); err != nil {
				return inventoryapp.AdjustmentBatchResult{}, err
			}
			return inventoryapp.AdjustmentBatchResult{Replay: true, EventIDs: nil}, nil
		}
	}

	legacySnapshot, err := q.InventoryAdminListMachineSlots(ctx, in.MachineID)
	if err != nil {
		return inventoryapp.AdjustmentBatchResult{}, err
	}

	eventType, err := inventoryapp.StockAdjustmentReasonToEventType(in.Reason)
	if err != nil {
		return inventoryapp.AdjustmentBatchResult{}, err
	}

	currency, err := q.InventoryAdminGetOrgDefaultCurrency(ctx, m.OrganizationID)
	if err != nil {
		return inventoryapp.AdjustmentBatchResult{}, err
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		currency = "USD"
	}

	var technicianID *uuid.UUID
	if in.OperatorSessionID != nil {
		sess, serr := q.GetOperatorSessionByID(ctx, *in.OperatorSessionID)
		if serr != nil {
			if !isNoRows(serr) {
				return inventoryapp.AdjustmentBatchResult{}, serr
			}
		} else if sess.TechnicianID.Valid {
			tid := uuid.UUID(sess.TechnicianID.Bytes)
			technicianID = &tid
		}
	}

	recordedAt := time.Now().UTC()
	occurredAt := recordedAt
	if in.OccurredAt != nil {
		occurredAt = in.OccurredAt.UTC()
	}

	for _, it := range in.Items {
		if it.QuantityAfter < 0 {
			return inventoryapp.AdjustmentBatchResult{}, errors.New("postgres: quantity_after must be non-negative")
		}
		var cur int32
		found := false
		for _, snap := range legacySnapshot {
			if snap.PlanogramID == it.PlanogramID && snap.SlotIndex == it.SlotIndex {
				cur = snap.CurrentQuantity
				found = true
				break
			}
		}
		if !found {
			return inventoryapp.AdjustmentBatchResult{}, inventoryapp.ErrAdjustmentSlotNotFound
		}
		if cur != it.QuantityBefore {
			return inventoryapp.AdjustmentBatchResult{}, inventoryapp.ErrQuantityBeforeMismatch
		}
	}

	objs := make([]map[string]any, 0, len(in.Items))
	for _, it := range in.Items {
		delta := it.QuantityDelta()
		meta := map[string]any{
			"reason":          in.Reason,
			"quantity_before": it.QuantityBefore,
			"quantity_after":  it.QuantityAfter,
		}
		if key != "" {
			meta["idempotency_key"] = key
		}

		slotCode := strings.TrimSpace(it.SlotCode)
		if slotCode == "" {
			slotCode = fmt.Sprintf("legacy-%d", it.SlotIndex)
		}
		cabinetCode := resolveAdjustmentCabinetCode(it, legacySnapshot)
		price := legacyPriceForSlot(legacySnapshot, it.PlanogramID, it.SlotIndex)

		row := map[string]any{
			"organization_id":   in.OrganizationID.String(),
			"machine_id":        in.MachineID.String(),
			"event_type":        eventType,
			"reason_code":       strings.TrimSpace(in.Reason),
			"quantity_delta":    int(delta),
			"quantity_before":   strconv.FormatInt(int64(it.QuantityBefore), 10),
			"quantity_after":    strconv.FormatInt(int64(it.QuantityAfter), 10),
			"unit_price_minor":  strconv.FormatInt(price, 10),
			"currency":          currency,
			"cabinet_code":      cabinetCode,
			"slot_code":         slotCode,
			"occurred_at":       occurredAt.Format(time.RFC3339Nano),
			"recorded_at":       recordedAt.Format(time.RFC3339Nano),
			"metadata":          meta,
		}
		if in.CorrelationID != nil {
			row["correlation_id"] = in.CorrelationID.String()
		}
		if in.OperatorSessionID != nil {
			row["operator_session_id"] = in.OperatorSessionID.String()
		}
		if technicianID != nil {
			row["technician_id"] = technicianID.String()
		}
		if it.MachineCabinetID != nil {
			row["machine_cabinet_id"] = it.MachineCabinetID.String()
		}
		if it.ProductID != nil {
			row["product_id"] = it.ProductID.String()
		}

		objs = append(objs, row)
	}

	payload, err := json.Marshal(objs)
	if err != nil {
		return inventoryapp.AdjustmentBatchResult{}, err
	}

	eventIDs, err := q.InventoryAdminInsertInventoryEventsBatch(ctx, payload)
	if err != nil {
		return inventoryapp.AdjustmentBatchResult{}, err
	}

	for _, it := range in.Items {
		var found bool
		for _, snap := range legacySnapshot {
			if snap.PlanogramID == it.PlanogramID && snap.SlotIndex == it.SlotIndex {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		nq := it.QuantityAfter
		if nq < 0 {
			nq = 0
		}
		_, err = q.InventoryAdminUpsertMachineSlotState(ctx, db.InventoryAdminUpsertMachineSlotStateParams{
			MachineID:                in.MachineID,
			PlanogramID:              it.PlanogramID,
			SlotIndex:                it.SlotIndex,
			CurrentQuantity:          nq,
			PriceMinor:               legacyPriceForSlot(legacySnapshot, it.PlanogramID, it.SlotIndex),
			PlanogramRevisionApplied: legacyRevisionForSlot(legacySnapshot, it.PlanogramID, it.SlotIndex),
		})
		if err != nil {
			return inventoryapp.AdjustmentBatchResult{}, err
		}
	}

	if len(eventIDs) > 0 && in.OperatorSessionID != nil {
		occ := occurredAt
		if err := insertOperatorSessionAttribution(ctx, q, operatorAttributionSpec{
			MachineID:         in.MachineID,
			OrganizationID:    in.OrganizationID,
			OperatorSessionID: in.OperatorSessionID,
			ActionDomain:      "inventory",
			ActionType:        "inventory.adjustment_batch",
			ResourceTable:     "inventory_events",
			ResourceID:        strconv.FormatInt(eventIDs[0], 10),
			CorrelationID:     in.CorrelationID,
			OccurredAt:        &occ,
		}); err != nil {
			return inventoryapp.AdjustmentBatchResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return inventoryapp.AdjustmentBatchResult{}, err
	}
	return inventoryapp.AdjustmentBatchResult{Replay: false, EventIDs: eventIDs}, nil
}
