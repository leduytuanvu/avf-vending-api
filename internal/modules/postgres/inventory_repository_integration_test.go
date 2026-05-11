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
	// When a machine cabinet row exists, inventory events store cabinet_code; otherwise legacy default CAB-A.
	require.Equal(t, "A", cab)
}

func TestInventoryRepository_CreateInventoryAdjustmentBatch_idempotentReplay(t *testing.T) {
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
			`DELETE FROM inventory_events WHERE machine_id = $1 AND metadata->>'idempotency_key' = 'grpc-idem-replay-test'`,
			testfixtures.DevMachineID,
		)
		_, _ = pool.Exec(ctx,
			`UPDATE machine_slot_state SET current_quantity = $1 WHERE machine_id = $2 AND planogram_id = $3 AND slot_index = 0`,
			origQty, testfixtures.DevMachineID, testfixtures.DevPlanogramID,
		)
	}()

	key := "grpc-idem-replay-test"
	repo := postgres.NewInventoryRepository(pool)
	in := inventoryapp.AdjustmentBatchInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		Reason:         "manual_adjustment",
		IdempotencyKey: key,
		ClientEventID:  "ce-replay-1",
		Items: []inventoryapp.AdjustmentItem{{
			PlanogramID:    testfixtures.DevPlanogramID,
			SlotIndex:      0,
			QuantityBefore: origQty,
			QuantityAfter:  origQty + 1,
			SlotCode:       "legacy-0",
		}},
	}
	res1, err := repo.CreateInventoryAdjustmentBatch(ctx, in)
	require.NoError(t, err)
	require.False(t, res1.Replay)
	require.NotEmpty(t, res1.EventIDs)

	res2, err := repo.CreateInventoryAdjustmentBatch(ctx, in)
	require.NoError(t, err)
	require.True(t, res2.Replay)
}

func TestInventoryRepository_CreateInventoryAdjustmentBatch_idempotencyKeyConflict(t *testing.T) {
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
			`DELETE FROM inventory_events WHERE machine_id = $1 AND metadata->>'idempotency_key' = 'grpc-idem-conflict-test'`,
			testfixtures.DevMachineID,
		)
		_, _ = pool.Exec(ctx,
			`UPDATE machine_slot_state SET current_quantity = $1 WHERE machine_id = $2 AND planogram_id = $3 AND slot_index = 0`,
			origQty, testfixtures.DevMachineID, testfixtures.DevPlanogramID,
		)
	}()

	key := "grpc-idem-conflict-test"
	repo := postgres.NewInventoryRepository(pool)
	_, err = repo.CreateInventoryAdjustmentBatch(ctx, inventoryapp.AdjustmentBatchInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		Reason:         "manual_adjustment",
		IdempotencyKey: key,
		ClientEventID:  "ce-1",
		Items: []inventoryapp.AdjustmentItem{{
			PlanogramID:    testfixtures.DevPlanogramID,
			SlotIndex:      0,
			QuantityBefore: origQty,
			QuantityAfter:  origQty + 1,
			SlotCode:       "legacy-0",
		}},
	})
	require.NoError(t, err)

	_, err = repo.CreateInventoryAdjustmentBatch(ctx, inventoryapp.AdjustmentBatchInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		Reason:         "manual_adjustment",
		IdempotencyKey: key,
		ClientEventID:  "ce-2",
		Items: []inventoryapp.AdjustmentItem{{
			PlanogramID:    testfixtures.DevPlanogramID,
			SlotIndex:      0,
			QuantityBefore: origQty,
			QuantityAfter:  origQty + 2,
			SlotCode:       "legacy-0",
		}},
	})
	require.ErrorIs(t, err, inventoryapp.ErrIdempotencyKeyConflict)
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
