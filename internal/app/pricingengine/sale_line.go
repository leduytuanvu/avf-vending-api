package pricingengine

import (
	"context"
	"errors"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

func rowSlotIndex(r db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) (int32, bool) {
	if !r.SlotIndex.Valid {
		return 0, false
	}
	return r.SlotIndex.Int32, true
}

func rowProductUUID(r db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) (uuid.UUID, bool) {
	if !r.ProductID.Valid {
		return uuid.Nil, false
	}
	return uuid.UUID(r.ProductID.Bytes), true
}

func matchSlotRow(in SaleLineSelector, r db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) bool {
	pid, ok := rowProductUUID(r)
	if !ok || pid != in.ProductID {
		return false
	}
	switch {
	case in.SlotID != nil && *in.SlotID != uuid.Nil:
		return r.ID == *in.SlotID
	case strings.TrimSpace(in.CabinetCode) != "" && strings.TrimSpace(in.SlotCode) != "":
		return strings.TrimSpace(r.CabinetCode) == strings.TrimSpace(in.CabinetCode) &&
			strings.TrimSpace(r.SlotCode) == strings.TrimSpace(in.SlotCode)
	case in.SlotIndex != nil && *in.SlotIndex >= 0:
		si, ok := rowSlotIndex(r)
		return ok && si == *in.SlotIndex
	default:
		return false
	}
}

func pickSlotRow(rows []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, org uuid.UUID, in SaleLineSelector) (db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, error) {
	var hits []db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow
	for _, r := range rows {
		if r.OrganizationID != org {
			continue
		}
		if matchSlotRow(in, r) {
			hits = append(hits, r)
		}
	}
	switch len(hits) {
	case 0:
		return db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{}, errors.Join(appcommerce.ErrInvalidArgument, errors.New("no matching current slot config for this product and slot selector"))
	case 1:
		return hits[0], nil
	default:
		return db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow{}, errors.Join(appcommerce.ErrInvalidArgument, errors.New("ambiguous slot selector; multiple slot configs matched"))
	}
}

// EvaluateSaleLine resolves the current slot row, applies machine overrides and active promotions.
func (e *Engine) EvaluateSaleLine(ctx context.Context, in SaleLineSelector, at time.Time, qty int32) (LinePriceResult, db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, error) {
	var zeroR db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow
	if e == nil || e.pool == nil {
		return LinePriceResult{}, zeroR, errors.New("pricingengine: nil engine")
	}
	q := db.New(e.pool)
	ok, err := q.CommerceIsProductInMachinePublishedAssortment(ctx, db.CommerceIsProductInMachinePublishedAssortmentParams{
		ID:             in.MachineID,
		OrganizationID: in.OrganizationID,
		ProductID:      in.ProductID,
	})
	if err != nil {
		return LinePriceResult{}, zeroR, err
	}
	if !ok {
		return LinePriceResult{}, zeroR, errors.Join(appcommerce.ErrInvalidArgument, errors.New("product is not in the machine's published assortment"))
	}
	rows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, in.MachineID)
	if err != nil {
		return LinePriceResult{}, zeroR, err
	}
	row, err := pickSlotRow(rows, in.OrganizationID, in)
	if err != nil {
		return LinePriceResult{}, zeroR, err
	}
	si, ok := rowSlotIndex(row)
	if !ok {
		return LinePriceResult{}, zeroR, errors.New("pricingengine: slot config has no slot_index")
	}
	batch, err := e.newBatch(ctx, in.OrganizationID, in.MachineID, at)
	if err != nil {
		return LinePriceResult{}, zeroR, err
	}
	res, err := batch.PriceLine(ctx, PriceLineInput{
		OrganizationID:    in.OrganizationID,
		MachineID:         in.MachineID,
		ProductID:         in.ProductID,
		SlotListUnitMinor: row.PriceMinor,
		SlotConfigID:      row.ID,
		CabinetCode:       row.CabinetCode,
		SlotCode:          row.SlotCode,
		SlotIndex:         si,
		Quantity:          qty,
	})
	if err != nil {
		return LinePriceResult{}, zeroR, err
	}
	return res, row, nil
}

// MapToResolvedSaleLine maps engine output to commerce sale line totals.
func MapToResolvedSaleLine(slotCfgID uuid.UUID, cab, slot string, slotIdx int32, r LinePriceResult) appcommerce.ResolvedSaleLine {
	return appcommerce.ResolvedSaleLine{
		SlotConfigID:        slotCfgID,
		CabinetCode:         cab,
		SlotCode:            slot,
		SlotIndex:           slotIdx,
		PriceMinor:          r.EffectiveUnitMinor,
		SubtotalMinor:       r.SubtotalMinor,
		TaxMinor:            r.TaxMinor,
		TotalMinor:          r.TotalMinor,
		BasePriceMinor:      r.RegisterUnitMinor,
		DiscountUnitMinor:   r.DiscountUnitMinor,
		PromotionLabel:      r.PromotionLabel,
		Currency:            r.Currency,
		PricingFingerprint:  r.PricingFingerprint,
		AppliedPromotionIDs: append([]uuid.UUID(nil), r.AppliedPromotionIDs...),
	}
}
