package postgres_test

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/inventoryapp"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/stretchr/testify/require"
)

func TestInventoryRepository_CreateInventoryAdjustmentBatch_quantityBeforeMismatch(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var origQty int32
	err := pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND planogram_id = $2 AND slot_index = 0`,
		testfixtures.DevMachineID, testfixtures.DevPlanogramID,
	).Scan(&origQty)
	require.NoError(t, err)

	defer func() {
		_, _ = pool.Exec(ctx,
			`DELETE FROM inventory_events WHERE machine_id = $1`,
			testfixtures.DevMachineID,
		)
	}()

	inv := postgres.NewInventoryRepository(pool)
	_, err = inv.CreateInventoryAdjustmentBatch(ctx, inventoryapp.AdjustmentBatchInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		Reason:         "restock",
		Items: []inventoryapp.AdjustmentItem{{
			PlanogramID:    testfixtures.DevPlanogramID,
			SlotIndex:      0,
			QuantityBefore: origQty + 50,
			QuantityAfter:  origQty + 51,
			SlotCode:       "legacy-0",
		}},
	})
	require.ErrorIs(t, err, inventoryapp.ErrQuantityBeforeMismatch)
}

func TestInventoryRepository_CreateInventoryAdjustmentBatch_invalidReason(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	inv := postgres.NewInventoryRepository(pool)
	_, err := inv.CreateInventoryAdjustmentBatch(ctx, inventoryapp.AdjustmentBatchInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		Reason:         "not_a_valid_reason",
		Items: []inventoryapp.AdjustmentItem{{
			PlanogramID:    testfixtures.DevPlanogramID,
			SlotIndex:      0,
			QuantityBefore: 0,
			QuantityAfter:  0,
		}},
	})
	require.ErrorIs(t, err, inventoryapp.ErrInvalidStockAdjustmentReason)
}

func TestInventoryRepository_CreateInventoryAdjustmentBatch_unknownSlot(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	inv := postgres.NewInventoryRepository(pool)
	_, err := inv.CreateInventoryAdjustmentBatch(ctx, inventoryapp.AdjustmentBatchInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		Reason:         "restock",
		Items: []inventoryapp.AdjustmentItem{{
			PlanogramID:    testfixtures.DevPlanogramID,
			SlotIndex:      99999,
			QuantityBefore: 0,
			QuantityAfter:  1,
		}},
	})
	require.ErrorIs(t, err, inventoryapp.ErrAdjustmentSlotNotFound)
}
