package postgres

import (
	"context"
	"encoding/json"
	"errors"
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

	now := time.Now().UTC()
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

		row := map[string]any{
			"organization_id": in.OrganizationID.String(),
			"machine_id":      in.MachineID.String(),
			"event_type":      eventType,
			"quantity_delta":  delta,
			"occurred_at":     now.Format(time.RFC3339Nano),
			"metadata":        meta,
		}
		if in.CorrelationID != nil {
			row["correlation_id"] = in.CorrelationID.String()
		}
		if in.OperatorSessionID != nil {
			row["operator_session_id"] = in.OperatorSessionID.String()
		}
		if it.MachineCabinetID != nil {
			row["machine_cabinet_id"] = it.MachineCabinetID.String()
		}
		if strings.TrimSpace(it.SlotCode) != "" {
			row["slot_code"] = strings.TrimSpace(it.SlotCode)
		}
		if it.ProductID != nil {
			row["product_id"] = it.ProductID.String()
		}
		row["quantity_after"] = it.QuantityAfter

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
		occ := now
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
