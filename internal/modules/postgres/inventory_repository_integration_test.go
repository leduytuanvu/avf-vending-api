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

func TestInventoryRepository_CreateInventoryAdjustmentBatch_writesLedgerQuantities(t *testing.T) {
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

	newQty := origQty + 2
	repo := postgres.NewInventoryRepository(pool)
	_, err = repo.CreateInventoryAdjustmentBatch(ctx, inventoryapp.AdjustmentBatchInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		Reason:         "manual_adjustment",
		Items: []inventoryapp.AdjustmentItem{{
			PlanogramID:    testfixtures.DevPlanogramID,
			SlotIndex:      0,
			QuantityBefore: origQty,
			QuantityAfter:  newQty,
			SlotCode:       "legacy-0",
		}},
	})
	require.NoError(t, err)

	var qb, qa, delta int32
	var reasonCode, curr, cab, sc string
	err = pool.QueryRow(ctx,
		`SELECT quantity_before, quantity_delta, quantity_after, reason_code, currency, coalesce(cabinet_code, ''), slot_code
		 FROM inventory_events WHERE machine_id = $1 ORDER BY id DESC LIMIT 1`,
		testfixtures.DevMachineID,
	).Scan(&qb, &delta, &qa, &reasonCode, &curr, &cab, &sc)
	require.NoError(t, err)
	require.Equal(t, origQty, qb)
	require.Equal(t, int32(2), delta)
	require.Equal(t, newQty, qa)
	require.Equal(t, "manual_adjustment", reasonCode)
	require.NotEmpty(t, curr)
	require.Equal(t, "legacy-0", sc)
	// cabinet_code falls back to CAB-A when unset (see inventory_repository.resolveAdjustmentCabinetCode).
	require.Equal(t, "CAB-A", cab)
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
