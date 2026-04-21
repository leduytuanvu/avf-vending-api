package inventoryapp

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrQuantityBeforeMismatch = errors.New("inventoryapp: quantity_before does not match current stock for slot")
	ErrAdjustmentSlotNotFound = errors.New("inventoryapp: machine_slot_state row not found for planogram and slot_index")
)

// AdjustmentItem is one slot-level quantity change applied to inventory_events and machine_slot_state.
// QuantityBefore must match machine_slot_state.current_quantity at commit time; QuantityAfter is the desired end state.
type AdjustmentItem struct {
	PlanogramID      uuid.UUID
	SlotIndex        int32
	QuantityBefore   int32
	QuantityAfter    int32
	SlotCode         string
	MachineCabinetID *uuid.UUID
	ProductID        *uuid.UUID
}

// QuantityDelta returns QuantityAfter - QuantityBefore.
func (it AdjustmentItem) QuantityDelta() int32 {
	return it.QuantityAfter - it.QuantityBefore
}

// AdjustmentBatchInput appends adjustment events and updates legacy slot quantities in one transaction.
type AdjustmentBatchInput struct {
	OrganizationID    uuid.UUID
	MachineID         uuid.UUID
	OperatorSessionID *uuid.UUID
	CorrelationID     *uuid.UUID
	Reason            string
	IdempotencyKey    string
	Items             []AdjustmentItem
}

// AdjustmentBatchResult reports inserted event ids or a replayed idempotent outcome.
type AdjustmentBatchResult struct {
	Replay   bool
	EventIDs []int64
}

// LedgerRepository writes append-only inventory_events and keeps machine_slot_state aligned.
type LedgerRepository interface {
	CreateInventoryAdjustmentBatch(ctx context.Context, in AdjustmentBatchInput) (AdjustmentBatchResult, error)
}
