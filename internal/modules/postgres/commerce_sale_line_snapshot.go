package postgres

import (
	"context"
	"errors"
	"strings"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/pricingengine"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// ResolveQuoteLinesFromMachineSnapshot implements appcommerce.SnapshotQuoteTopologyResolver.
func (s *Store) ResolveQuoteLinesFromMachineSnapshot(
	ctx context.Context,
	machineID uuid.UUID,
	lines []appcommerce.QuoteSaleLineResolveInput,
	snap appcommerce.MachinePricingSnapshotInput,
) ([]appcommerce.ResolvedSaleLine, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("postgres: nil store")
	}
	if machineID == uuid.Nil || len(lines) == 0 {
		return nil, errors.Join(appcommerce.ErrInvalidArgument, errors.New("machine_id and lines required"))
	}
	q := db.New(s.pool)
	rows, err := q.InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	fp := strings.TrimSpace(snap.PricingFingerprint)
	out := make([]appcommerce.ResolvedSaleLine, len(lines))
	for i, line := range lines {
		qty := line.Qty
		if qty <= 0 {
			qty = 1
		}
		sel := pricingengine.SaleLineSelector{
			MachineID:   machineID,
			ProductID:   line.In.ProductID,
			SlotID:      line.In.SlotID,
			CabinetCode: line.In.CabinetCode,
			SlotCode:    line.In.SlotCode,
			SlotIndex:   line.In.SlotIndex,
		}
		row, err := pricingengine.PickSlotRowFromInventory(rows, sel)
		if err != nil {
			return nil, err
		}
		si, ok := slotIdxFromInventoryRow(row)
		if !ok {
			return nil, errors.New("postgres: slot config has no slot_index")
		}
		seq := int32(i + 1)
		unit := appcommerce.SnapshotLineUnitPrice(snap, seq, 0)
		if unit <= 0 {
			unit = snap.UnitPriceMinor
		}
		if unit <= 0 {
			return nil, errors.Join(appcommerce.ErrInvalidArgument, errors.New("pricing_snapshot unit_price_minor invalid"))
		}
		lineTotal := unit * int64(qty)
		out[i] = appcommerce.ResolvedSaleLine{
			SlotConfigID:       row.ID,
			CabinetCode:        strings.TrimSpace(row.CabinetCode),
			SlotCode:           strings.TrimSpace(row.SlotCode),
			SlotIndex:          si,
			PriceMinor:         unit,
			SubtotalMinor:      lineTotal,
			TotalMinor:         lineTotal,
			PricingFingerprint: fp,
		}
	}
	return out, nil
}
