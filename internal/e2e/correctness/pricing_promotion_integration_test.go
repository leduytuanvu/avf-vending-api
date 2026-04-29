package correctness

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/assortmentapp"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	appsalecatalog "github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func cleanupPricingTestMachine(ctx context.Context, t *testing.T, pool *pgxpool.Pool, machineID uuid.UUID, assortmentID uuid.UUID) {
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

// TestPricingPromotion_catalogOrderPaymentAmountAlignment checks catalog price, ResolveSaleLine, and order totals stay aligned.
func TestPricingPromotion_catalogOrderPaymentAmountAlignment(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := db.New(pool)

	assRow, err := q.FleetAdminInsertAssortment(ctx, db.FleetAdminInsertAssortmentParams{
		OrganizationID: testfixtures.DevOrganizationID,
		Name:           "pricing-promo-e2e-" + uuid.NewString(),
		Status:         "published",
		Description:    "",
		Meta:           []byte(`{}`),
	})
	require.NoError(t, err)
	assortmentID := assRow.ID
	defer cleanupPricingTestMachine(ctx, t, pool, testfixtures.DevMachineID, assortmentID)

	_, err = q.FleetAdminUpsertAssortmentItem(ctx, db.FleetAdminUpsertAssortmentItemParams{
		OrganizationID: testfixtures.DevOrganizationID,
		AssortmentID:   assortmentID,
		ProductID:      testfixtures.DevProductWater,
		SortOrder:      1,
		Notes:          []byte(`{}`),
	})
	require.NoError(t, err)
	arepo := postgres.NewAssortmentRepository(pool)
	require.NoError(t, arepo.BindMachineAssortment(ctx, assortmentapp.BindMachineAssortmentInput{
		MachineID:    testfixtures.DevMachineID,
		AssortmentID: assortmentID,
	}))

	repo := postgres.NewSetupRepository(pool)
	require.NoError(t, repo.UpsertMachineTopology(ctx, testfixtures.DevMachineID,
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
	))
	slotIdx := int32(1)
	water := testfixtures.DevProductWater
	require.NoError(t, repo.SaveDraftOrCurrentSlotConfigs(ctx, testfixtures.DevMachineID, setupapp.SlotConfigSaveInput{
		PlanogramID:         testfixtures.DevPlanogramID,
		PlanogramRevision:   1,
		PublishAsCurrent:    true,
		SyncLegacyReadModel: true,
		Items: []setupapp.SlotConfigSaveItem{{
			CabinetCode:     "A",
			LayoutKey:       "default",
			LayoutRevision:  1,
			SlotCode:        "S1",
			LegacySlotIndex: &slotIdx,
			ProductID:       &water,
			MaxQuantity:     10,
			PriceMinor:      4242,
			EffectiveFrom:   time.Now().UTC(),
			Metadata:        []byte(`{}`),
		}},
	}))

	store := postgres.NewStore(pool)
	line, err := store.ResolveSaleLine(ctx, appcommerce.ResolveSaleLineInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      &slotIdx,
	})
	require.NoError(t, err)

	cat := appsalecatalog.NewService(pool)
	snap, err := cat.BuildSnapshot(ctx, testfixtures.DevMachineID, appsalecatalog.Options{
		IncludeUnavailable: true,
		IncludeImages:      false,
	})
	require.NoError(t, err)
	var item *appsalecatalog.Item
	for i := range snap.Items {
		if snap.Items[i].ProductID == testfixtures.DevProductWater {
			item = &snap.Items[i]
			break
		}
	}
	require.NotNil(t, item)
	require.Equal(t, line.PriceMinor, item.PriceMinor)
	require.Equal(t, line.PricingFingerprint, item.PricingFingerprint)
	require.Equal(t, line.TotalMinor, item.PriceMinor)

	evAt := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	line2, err := store.EvaluateSaleLineAt(ctx, appcommerce.ResolveSaleLineInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      &slotIdx,
	}, evAt, 1)
	require.NoError(t, err)
	line3, err := store.EvaluateSaleLineAt(ctx, appcommerce.ResolveSaleLineInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      &slotIdx,
	}, evAt, 1)
	require.NoError(t, err)
	require.Equal(t, line2.TotalMinor, line3.TotalMinor)
	require.Equal(t, line2.PricingFingerprint, line3.PricingFingerprint)
}
