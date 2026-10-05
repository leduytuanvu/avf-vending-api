package commerce

import (
	"context"
	"errors"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestCreateOfflineOrderFromSnapshot_singleLineUsesCheckoutMirror(t *testing.T) {
	t.Parallel()
	slotID := uuid.New()
	productID := uuid.New()
	machineID := uuid.New()
	slotIdx := int32(4)
	slotsJSON := []byte(`[{"slotCode":"A4","productId":"` + productID.String() + `","priceMinor":5000,"localPricingRevision":1}]`)
	orderVend := &captureOrderVendWorkflow{}
	svc := NewService(Deps{
		OrderVend: orderVend,
		SaleLines: mirrorFallbackResolver{
			fail: true,
			byCode: map[string]db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{
				"A4": {
					ID:          slotID,
					SlotCode:    "A4",
					CabinetCode: "A",
					SlotIndex:   pgtype.Int4{Int32: slotIdx, Valid: true},
				},
			},
		},
		LocalLayoutMirror: stubMirrorReader{
			mirror: LocalLayoutMirror{Revision: 1, SlotsJSON: slotsJSON},
		},
	})
	snap := appCheckoutPricingSnapshot{
		SnapshotID:        "snap-1",
		PayableTotalMinor: 5000,
		Currency:          "VND",
		Lines: []appCheckoutLine{
			{SlotCode: "A4", ProductID: productID.String(), UnitPriceMinor: 5000, Quantity: 1},
		},
	}
	pricingSnap := machinePricingSnapshotFromAppCheckout(snap, 5000)

	orderID, err := svc.createOfflineOrderFromSnapshot(
		t.Context(),
		machineID,
		snap,
		pricingSnap,
		"VND",
		"offline-sale:test",
	)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, orderID)
	require.Equal(t, machineID, orderVend.lastInput.MachineID)
	require.Equal(t, productID, orderVend.lastInput.ProductID)
}

type publishedAssortmentStaleResolver struct {
	byCode map[string]db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow
}

func (r publishedAssortmentStaleResolver) ResolveSaleLine(context.Context, ResolveSaleLineInput) (ResolvedSaleLine, error) {
	return ResolvedSaleLine{}, errors.Join(
		ErrInvalidArgument,
		errors.New("product is not in the machine's published assortment"),
	)
}

func (r publishedAssortmentStaleResolver) LookupCurrentSlotConfigByCode(_ context.Context, _ uuid.UUID, slotCode string) (db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, error) {
	row, ok := r.byCode[slotCode]
	if !ok {
		return db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{}, ErrNotFound
	}
	return row, nil
}

func (r publishedAssortmentStaleResolver) LookupSlotDisplay(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int32) (ResolvedSaleLine, error) {
	return ResolvedSaleLine{}, ErrNotFound
}

func TestCreateOfflineOrderFromSnapshot_mirrorWhenPublishedAssortmentStale(t *testing.T) {
	t.Parallel()
	slotID := uuid.New()
	productID := uuid.New()
	machineID := uuid.New()
	slotIdx := int32(4)
	slotsJSON := []byte(`[{"slotCode":"A4","productId":"` + productID.String() + `","priceMinor":5000,"localPricingRevision":1}]`)
	orderVend := &captureOrderVendWorkflow{}
	svc := NewService(Deps{
		OrderVend: orderVend,
		SaleLines: publishedAssortmentStaleResolver{
			byCode: map[string]db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{
				"A4": {
					ID:          slotID,
					SlotCode:    "A4",
					CabinetCode: "A",
					SlotIndex:   pgtype.Int4{Int32: slotIdx, Valid: true},
				},
			},
		},
		LocalLayoutMirror: stubMirrorReader{
			mirror: LocalLayoutMirror{Revision: 1, SlotsJSON: slotsJSON},
		},
	})
	snap := appCheckoutPricingSnapshot{
		SnapshotID:        uuid.NewString(),
		PayableTotalMinor: 5000,
		Lines: []appCheckoutLine{
			{SlotCode: "A4", ProductID: productID.String(), UnitPriceMinor: 5000, Quantity: 1},
		},
	}
	pricingSnap := machinePricingSnapshotFromAppCheckout(snap, 5000)

	orderID, err := svc.createOfflineOrderFromSnapshot(
		t.Context(),
		machineID,
		snap,
		pricingSnap,
		"VND",
		"offline-sale:stale",
	)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, orderID)
	require.Equal(t, slotIdx, orderVend.lastInput.SlotIndex)
}
