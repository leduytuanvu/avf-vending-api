package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

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
		"cash",
		"offline-sale:test",
		uuid.Nil,
		time.Time{},
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
		"cash",
		"offline-sale:stale",
		uuid.Nil,
		time.Time{},
	)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, orderID)
	require.Equal(t, slotIdx, orderVend.lastInput.SlotIndex)
}

func TestOptionalClientOrderID(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("67af9332-f121-4860-a4bb-a0c44ae033b2")
	require.Equal(t, id, optionalClientOrderID(id.String()))
	require.Equal(t, uuid.Nil, optionalClientOrderID("p06-offline-sale-not-a-uuid:order"))
	require.Equal(t, uuid.Nil, optionalClientOrderID(""))
}

func TestCreateOfflineOrderFromSnapshot_reusesClientOrderID(t *testing.T) {
	t.Parallel()
	slotID := uuid.New()
	productID := uuid.New()
	machineID := uuid.New()
	clientOrderID := uuid.MustParse("67af9332-f121-4860-a4bb-a0c44ae033b2")
	slotIdx := int32(11)
	slotsJSON := []byte(`[{"slotCode":"B1","productId":"` + productID.String() + `","priceMinor":10000,"localPricingRevision":1}]`)
	orderVend := &captureOrderVendWorkflow{}
	svc := NewService(Deps{
		OrderVend: orderVend,
		SaleLines: mirrorFallbackResolver{
			fail: true,
			byCode: map[string]db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{
				"B1": {
					ID:          slotID,
					SlotCode:    "B1",
					CabinetCode: "B",
					SlotIndex:   pgtype.Int4{Int32: slotIdx, Valid: true},
				},
			},
		},
		LocalLayoutMirror: stubMirrorReader{
			mirror: LocalLayoutMirror{Revision: 1, SlotsJSON: slotsJSON},
		},
	})
	snap := appCheckoutPricingSnapshot{
		SnapshotID:        "snap-client",
		PayableTotalMinor: 10000,
		Currency:          "VND",
		Lines: []appCheckoutLine{
			{SlotCode: "B1", ProductID: productID.String(), UnitPriceMinor: 10000, Quantity: 1},
		},
	}
	pricingSnap := machinePricingSnapshotFromAppCheckout(snap, 10000)

	_, err := svc.createOfflineOrderFromSnapshot(
		t.Context(),
		machineID,
		snap,
		pricingSnap,
		"VND",
		"cash",
		"offline-sale:client-uuid",
		clientOrderID,
		time.Time{},
	)
	require.NoError(t, err)
	require.Equal(t, clientOrderID, orderVend.lastInput.OrderID)
}

func TestCreateOfflineOrderFromSnapshot_forwardsCreatedAt(t *testing.T) {
	t.Parallel()
	slotID := uuid.New()
	productID := uuid.New()
	machineID := uuid.New()
	createdAt := time.UnixMilli(1_700_000_100_000).UTC()
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
		SnapshotID:        "snap-created-at",
		PayableTotalMinor: 5000,
		Currency:          "VND",
		Lines: []appCheckoutLine{
			{SlotCode: "A4", ProductID: productID.String(), UnitPriceMinor: 5000, Quantity: 1},
		},
	}
	pricingSnap := machinePricingSnapshotFromAppCheckout(snap, 5000)

	_, err := svc.createOfflineOrderFromSnapshot(
		t.Context(),
		machineID,
		snap,
		pricingSnap,
		"VND",
		"cash",
		"offline-sale:created-at",
		uuid.Nil,
		createdAt,
	)
	require.NoError(t, err)
	require.Equal(t, createdAt, orderVend.lastInput.CreatedAt)
}

func TestProcessOfflineSale_stampsCreatedAtFromOccurredAt(t *testing.T) {
	t.Parallel()
	slotID := uuid.New()
	productID := uuid.New()
	machineID := uuid.New()
	occurredAt := time.Now().UTC().Add(-7 * time.Minute).Truncate(time.Millisecond)
	capturedMs := time.Now().UTC().Add(-30 * time.Second).UnixMilli()
	slotIdx := int32(4)
	slotsJSON := []byte(`[{"slotCode":"A4","productId":"` + productID.String() + `","priceMinor":5000,"localPricingRevision":1}]`)
	orderVend := &captureOrderVendWorkflow{}
	life := &cashConfirmLifecycle{
		order: domainOrder(uuidNew(), machineID, 5000, PricingSourceMachineLocalUnverified),
	}
	payments := &cashConfirmPayments{}
	life.payments = payments
	svc := NewService(Deps{
		OrderVend:     orderVend,
		PaymentOutbox: payments,
		Lifecycle:     life,
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
	payload, err := json.Marshal(map[string]any{
		"orderId":           uuid.NewString(),
		"machineId":         machineID.String(),
		"transactionId":     "tx-created-at",
		"cashReceivedMinor": 5000,
		"payableTotalMinor": 5000,
		"currency":          "VND",
		"pricingSnapshot": map[string]any{
			"snapshotId":           "snap-created-at",
			"payableTotalMinor":    5000,
			"currency":             "VND",
			"localPricingRevision": 1,
			"slotConfigVersion":    1,
			"capturedAtEpochMs":    capturedMs,
			"lines": []map[string]any{
				{"slotCode": "A4", "productId": productID.String(), "unitPriceMinor": 5000, "quantity": 1},
			},
		},
	})
	require.NoError(t, err)

	err = svc.ProcessOfflineSale(
		t.Context(),
		machineID,
		"offline-sale:occurred-at",
		"client-event-1",
		payload,
		OfflineSaleOutboxConfig{Topic: "commerce", EventType: "cash", AggregateType: "order"},
		occurredAt,
	)
	require.NoError(t, err)
	require.Equal(t, occurredAt, orderVend.lastInput.CreatedAt)
	require.NotEqual(t, time.UnixMilli(capturedMs).UTC(), orderVend.lastInput.CreatedAt)
}
