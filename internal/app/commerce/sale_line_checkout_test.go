package commerce

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type mirrorFallbackResolver struct {
	stubSaleLineResolver
	byCode map[string]db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow
	fail   bool
}

func (r mirrorFallbackResolver) ResolveSaleLine(context.Context, ResolveSaleLineInput) (ResolvedSaleLine, error) {
	if r.fail {
		return ResolvedSaleLine{}, errors.Join(ErrInvalidArgument, errors.New("no matching current slot config for this product and slot selector"))
	}
	return ResolvedSaleLine{}, nil
}

func (r mirrorFallbackResolver) LookupCurrentSlotConfigByCode(_ context.Context, _ uuid.UUID, slotCode string) (db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, error) {
	row, ok := r.byCode[strings.ToUpper(strings.TrimSpace(slotCode))]
	if !ok {
		return db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{}, ErrNotFound
	}
	return row, nil
}

type stubMirrorReader struct {
	mirror LocalLayoutMirror
}

func (s stubMirrorReader) GetLocalLayoutMirror(context.Context, uuid.UUID) (LocalLayoutMirror, error) {
	return s.mirror, nil
}

func TestResolveCheckoutSaleLine_fallsBackToPricingSnapshotWhenAssortmentStale(t *testing.T) {
	slotID := uuid.New()
	productID := uuid.New()
	machineID := uuid.New()
	slotIdx := int32(12)
	slotsJSON := []byte(`[{"slotCode":"A1","productId":"` + productID.String() + `","priceMinor":10000,"localPricingRevision":3}]`)
	svc := &Service{
		saleLines: mirrorFallbackResolver{
			fail: true,
			byCode: map[string]db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{
				"A1": {
					ID:          slotID,
					SlotCode:    "A1",
					CabinetCode: "A",
					SlotIndex:   pgtype.Int4{Int32: slotIdx, Valid: true},
				},
			},
		},
		layoutMirror: stubMirrorReader{
			mirror: LocalLayoutMirror{Revision: 3, SlotsJSON: slotsJSON},
		},
	}
	snap := MachinePricingSnapshotInput{
		SubtotalMinor:        10000,
		TotalMinor:           10000,
		UnitPriceMinor:       10000,
		LocalPricingRevision: 3,
		Lines: []MachinePricingSnapshotLineInput{
			{
				LineSequence:      1,
				ProductID:         productID,
				SlotCode:          "A1",
				Quantity:          1,
				UnitPriceMinor:    10000,
				LineSubtotalMinor: 10000,
			},
		},
	}
	line, err := svc.resolveCheckoutSaleLine(t.Context(), ResolveSaleLineInput{
		MachineID: machineID,
		ProductID: productID,
		SlotCode:  "A1",
	}, &snap, 1)
	require.NoError(t, err)
	require.Equal(t, slotID, line.SlotConfigID)
	require.Equal(t, int64(10000), line.PriceMinor)
	require.Equal(t, slotIdx, line.SlotIndex)
}
