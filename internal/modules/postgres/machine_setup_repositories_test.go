package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/assortmentapp"
	"github.com/avf/avf-vending-api/internal/app/inventoryapp"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func cleanupMachineSetupArtifacts(ctx context.Context, t *testing.T, pool *pgxpool.Pool, machineID uuid.UUID, assortmentID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `DELETE FROM inventory_events WHERE machine_id = $1`, machineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_slot_configs WHERE machine_id = $1`, machineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_slot_layouts WHERE machine_id = $1`, machineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_assortment_bindings WHERE machine_id = $1`, machineID)
	require.NoError(t, err)
	if assortmentID != uuid.Nil {
		_, err = pool.Exec(ctx, `DELETE FROM assortment_items WHERE assortment_id = $1`, assortmentID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM assortments WHERE id = $1`, assortmentID)
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, `DELETE FROM machine_cabinets WHERE machine_id = $1`, machineID)
	require.NoError(t, err)
}

func TestSetupRepository_UpsertMachineTopology(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	defer cleanupMachineSetupArtifacts(ctx, t, pool, testfixtures.DevMachineID, uuid.Nil)

	repo := postgres.NewSetupRepository(pool)
	err := repo.UpsertMachineTopology(ctx, testfixtures.DevMachineID,
		[]setupapp.CabinetUpsert{{
			Code:      "A",
			Title:     "Alpha",
			SortOrder: 1,
			Metadata:  []byte(`{}`),
		}},
		[]setupapp.TopologyLayoutUpsert{{
			CabinetCode: "A",
			LayoutKey:   "default",
			Revision:    1,
			LayoutSpec:  []byte(`{"rows":1}`),
			Status:      "published",
		}},
	)
	require.NoError(t, err)

	boot, err := repo.GetMachineBootstrap(ctx, testfixtures.DevMachineID)
	require.NoError(t, err)
	require.Len(t, boot.Cabinets, 1)
	require.Equal(t, "A", boot.Cabinets[0].Code)

	view, err := repo.GetMachineSlotView(ctx, testfixtures.DevMachineID)
	require.NoError(t, err)
	require.NotEmpty(t, view.LegacySlots)
}

func TestAssortmentRepository_BindMachineAssortment(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)

	assRow, err := q.FleetAdminInsertAssortment(ctx, db.FleetAdminInsertAssortmentParams{
		OrganizationID: testfixtures.DevOrganizationID,
		Name:           "test-assort-" + uuid.NewString(),
		Status:         "published",
		Description:    "",
		Meta:           []byte(`{}`),
	})
	require.NoError(t, err)
	assortmentID := assRow.ID
	defer cleanupMachineSetupArtifacts(ctx, t, pool, testfixtures.DevMachineID, assortmentID)

	_, err = q.FleetAdminUpsertAssortmentItem(ctx, db.FleetAdminUpsertAssortmentItemParams{
		OrganizationID: testfixtures.DevOrganizationID,
		AssortmentID:   assortmentID,
		ProductID:      testfixtures.DevProductCola,
		SortOrder:      1,
		Notes:          []byte(`{}`),
	})
	require.NoError(t, err)

	arepo := postgres.NewAssortmentRepository(pool)
	err = arepo.BindMachineAssortment(ctx, assortmentapp.BindMachineAssortmentInput{
		MachineID:    testfixtures.DevMachineID,
		AssortmentID: assortmentID,
	})
	require.NoError(t, err)

	boot, err := postgres.NewSetupRepository(pool).GetMachineBootstrap(ctx, testfixtures.DevMachineID)
	require.NoError(t, err)
	require.Len(t, boot.AssortmentProducts, 1)
	require.Equal(t, testfixtures.DevProductCola, boot.AssortmentProducts[0].ProductID)
}

func TestInventoryRepository_CreateInventoryAdjustmentBatch(t *testing.T) {
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
			`UPDATE machine_slot_state SET current_quantity = $1 WHERE machine_id = $2 AND planogram_id = $3 AND slot_index = 0`,
			origQty, testfixtures.DevMachineID, testfixtures.DevPlanogramID,
		)
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_events WHERE machine_id = $1`, testfixtures.DevMachineID)
	}()

	op := postgres.NewOperatorRepository(pool)
	sess, err := op.StartOperatorSession(ctx, operator.StartOperatorSessionParams{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ActorType:      operator.ActorTypeTechnician,
		TechnicianID:   ptrUUID(testfixtures.DevTechnicianID),
		ClientMetadata: []byte(`{}`),
	})
	require.NoError(t, err)
	defer func() {
		_, _ = op.EndOperatorSession(ctx, operator.EndOperatorSessionParams{
			SessionID: sess.ID,
			Status:    operator.SessionStatusEnded,
			EndedAt:   time.Now().UTC(),
		})
	}()

	corr := uuid.New()
	idem := "adj-test-" + uuid.NewString()
	inv := postgres.NewInventoryRepository(pool)
	res, err := inv.CreateInventoryAdjustmentBatch(ctx, inventoryapp.AdjustmentBatchInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		OperatorSessionID: &sess.ID,
		CorrelationID:     &corr,
		Reason:            "manual_adjustment",
		IdempotencyKey:    idem,
		Items: []inventoryapp.AdjustmentItem{{
			PlanogramID:    testfixtures.DevPlanogramID,
			SlotIndex:      0,
			QuantityBefore: origQty,
			QuantityAfter:  origQty - 1,
			SlotCode:       "legacy-0",
			ProductID:      ptrUUID(testfixtures.DevProductCola),
		}},
	})
	require.NoError(t, err)
	require.False(t, res.Replay)
	require.Len(t, res.EventIDs, 1)

	var eventType string
	err = pool.QueryRow(ctx,
		`SELECT event_type FROM inventory_events WHERE machine_id = $1 ORDER BY id DESC LIMIT 1`,
		testfixtures.DevMachineID,
	).Scan(&eventType)
	require.NoError(t, err)
	require.Equal(t, "adjustment", eventType)

	var opSess pgtype.UUID
	err = pool.QueryRow(ctx,
		`SELECT operator_session_id FROM inventory_events WHERE machine_id = $1 ORDER BY id DESC LIMIT 1`,
		testfixtures.DevMachineID,
	).Scan(&opSess)
	require.NoError(t, err)
	require.True(t, opSess.Valid)
	require.Equal(t, sess.ID, opSess.Bytes)

	var qty int32
	err = pool.QueryRow(ctx,
		`SELECT current_quantity FROM machine_slot_state WHERE machine_id = $1 AND planogram_id = $2 AND slot_index = 0`,
		testfixtures.DevMachineID, testfixtures.DevPlanogramID,
	).Scan(&qty)
	require.NoError(t, err)
	require.Equal(t, origQty-1, qty)

	res2, err := inv.CreateInventoryAdjustmentBatch(ctx, inventoryapp.AdjustmentBatchInput{
		OrganizationID:    testfixtures.DevOrganizationID,
		MachineID:         testfixtures.DevMachineID,
		OperatorSessionID: &sess.ID,
		CorrelationID:     &corr,
		Reason:            "manual_adjustment",
		IdempotencyKey:    idem,
		Items: []inventoryapp.AdjustmentItem{{
			PlanogramID:    testfixtures.DevPlanogramID,
			SlotIndex:      0,
			QuantityBefore: origQty - 1,
			QuantityAfter:  origQty - 1,
			SlotCode:       "legacy-0",
		}},
	})
	require.NoError(t, err)
	require.True(t, res2.Replay)

	var cnt int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM machine_action_attributions WHERE machine_id = $1 AND resource_type = 'inventory_events'`,
		testfixtures.DevMachineID,
	).Scan(&cnt)
	require.NoError(t, err)
	require.GreaterOrEqual(t, cnt, 1)
}

func ptrUUID(u uuid.UUID) *uuid.UUID { return &u }
