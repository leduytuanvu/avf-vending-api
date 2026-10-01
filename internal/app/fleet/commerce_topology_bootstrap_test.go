package fleet

import (
	"encoding/json"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestDefaultBootstrapGridDimensions(t *testing.T) {
	rows, cols := DefaultBootstrapGridDimensions()
	require.Equal(t, int32(6), rows)
	require.Equal(t, int32(10), cols)
}

func TestDefaultCommerceTopologyConstants(t *testing.T) {
	require.Equal(t, "CAB-A", defaultCommerceCabinetCode)
	require.Equal(t, "default", defaultCommerceLayoutKey)
	require.Equal(t, int32(1), defaultCommerceLayoutRevision)
}

func TestCurrentSlotConfigMatchesDesired(t *testing.T) {
	cabID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	layoutID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	existing := db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{
		MachineCabinetID:    cabID,
		MachineSlotLayoutID: layoutID,
		SlotIndex:           pgtype.Int4{Int32: 1, Valid: true},
		ProductID:           pgtype.UUID{Bytes: pid, Valid: true},
		MaxQuantity:         10,
		PriceMinor:          5000,
	}
	require.True(t, currentSlotConfigMatchesDesired(
		existing, cabID, layoutID,
		pgtype.Int4{Int32: 1, Valid: true},
		pgtype.UUID{Bytes: pid, Valid: true},
		10, 5000,
	))
	require.False(t, currentSlotConfigMatchesDesired(
		existing, uuid.New(), layoutID,
		pgtype.Int4{Int32: 1, Valid: true},
		pgtype.UUID{Bytes: pid, Valid: true},
		10, 5000,
	))
}

func TestPgUUIDEqual(t *testing.T) {
	require.True(t, pgUUIDEqual(pgtype.UUID{Valid: false}, pgtype.UUID{Valid: false}))
	a := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	require.True(t, pgUUIDEqual(a, a))
	require.False(t, pgUUIDEqual(a, pgtype.UUID{Valid: false}))
}

func TestCommerceCabinetMetadataJSON(t *testing.T) {
	raw := commerceCabinetMetadataJSON("bootstrap", "tcn")
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	require.Equal(t, "bootstrap", m["materializedBy"])
	require.Equal(t, defaultBoardProtocol, m["board_protocol"])
	require.Equal(t, defaultBillProtocol, m["bill_protocol"])
	require.Equal(t, defaultCashTopology, m["cash_topology"])
}

func TestMergeCommerceCabinetMetadata_preservesExistingKeys(t *testing.T) {
	existing := []byte(`{"materializedBy":"legacy","payment_authority":"local"}`)
	merged := mergeCommerceCabinetMetadata(existing, "reconcile", "tcn")
	var m map[string]any
	require.NoError(t, json.Unmarshal(merged, &m))
	require.Equal(t, "reconcile", m["materializedBy"])
	require.Equal(t, "local", m["payment_authority"])
	require.Equal(t, defaultBoardProtocol, m["board_protocol"])
	require.Equal(t, defaultBillProtocol, m["bill_protocol"])
	require.Equal(t, defaultCashTopology, m["cash_topology"])
}

func TestCommerceCabinetMetadataComplete(t *testing.T) {
	require.False(t, commerceCabinetMetadataComplete([]byte(`{"materializedBy":"bootstrap"}`)))
	require.True(t, commerceCabinetMetadataComplete(commerceCabinetMetadataJSON("bootstrap", "tcn")))
}
