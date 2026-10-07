package httpserver

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestResolveMachineCatalogRevisions_prefersPublishedPlanogramVersion(t *testing.T) {
	bootstrap := setupapp.MachineBootstrap{PublishedPlanogramVersionNo: 7}
	snap := salecatalog.Snapshot{ConfigVersion: 99}
	inv := []db.InventoryAdminListMachineSlotsRow{{PlanogramRevisionApplied: 3}}
	catalogRev, planogramRev := resolveMachineCatalogRevisions(bootstrap, snap, inv)
	require.Equal(t, int32(7), catalogRev)
	require.Equal(t, int32(7), planogramRev)
}

func TestResolveMachineCatalogRevisions_fallsBackToInventoryAndConfig(t *testing.T) {
	bootstrap := setupapp.MachineBootstrap{}
	snap := salecatalog.Snapshot{ConfigVersion: 12}
	inv := []db.InventoryAdminListMachineSlotsRow{{PlanogramRevisionApplied: 5}}
	catalogRev, planogramRev := resolveMachineCatalogRevisions(bootstrap, snap, inv)
	require.Equal(t, int32(5), planogramRev)
	require.Equal(t, int32(5), catalogRev)
}

func TestMapSlotConfigsToPlanogram_mapsAssignmentAndStock(t *testing.T) {
	pid := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	cfgID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	configs := []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{
		{
			ID:           cfgID,
			SlotCode:     "A1",
			SlotIndex:    pgtype.Int4{Int32: 0, Valid: true},
			CabinetIndex: 0,
			ProductID:    pgtype.UUID{Bytes: pid, Valid: true},
			PriceMinor:   5000,
			MaxQuantity:  10,
		},
	}
	inv := map[int32]db.InventoryAdminListMachineSlotsRow{
		0: {CurrentQuantity: 4, IsEmpty: false},
	}
	out := mapSlotConfigsToPlanogram(configs, inv)
	require.Len(t, out, 1)
	require.Equal(t, "A1", out[0]["laneCode"])
	require.Equal(t, 4, out[0]["stockOnHand"])
	assign, ok := out[0]["assignment"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, pid.String(), assign["productId"])
	require.Equal(t, "5000", assign["priceMinor"])
}
