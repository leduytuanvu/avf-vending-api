package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/pricingengine"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

func slotIdxFromInventoryRow(r db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) (int32, bool) {
	if !r.SlotIndex.Valid {
		return 0, false
	}
	return r.SlotIndex.Int32, true
}

// ResolveSaleLine implements appcommerce.SaleLineResolver.
func (s *Store) ResolveSaleLine(ctx context.Context, in appcommerce.ResolveSaleLineInput) (appcommerce.ResolvedSaleLine, error) {
	if s == nil || s.pool == nil {
		return appcommerce.ResolvedSaleLine{}, errors.New("postgres: nil store")
	}
	eng := s.pricing
	if eng == nil {
		eng = pricingengine.New(s.pool)
	}
	sel := pricingengine.SaleLineSelector{
		MachineID:   in.MachineID,
		ProductID:   in.ProductID,
		SlotID:      in.SlotID,
		CabinetCode: in.CabinetCode,
		SlotCode:    in.SlotCode,
		SlotIndex:   in.SlotIndex,
	}
	line, row, err := eng.EvaluateSaleLine(ctx, sel, pricingengine.NowUTC(), 1)
	if err != nil {
		return appcommerce.ResolvedSaleLine{}, err
	}
	si, ok := slotIdxFromInventoryRow(row)
	if !ok {
		return appcommerce.ResolvedSaleLine{}, errors.New("postgres: slot config has no slot_index")
	}
	return pricingengine.MapToResolvedSaleLine(row.ID, row.CabinetCode, row.SlotCode, si, line), nil
}

// LookupCurrentSlotConfigByCode returns the current slot config row for slotCode (product match not required).
func (s *Store) LookupCurrentSlotConfigByCode(
	ctx context.Context,
	machineID uuid.UUID,
	slotCode string,
) (db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, error) {
	var zero db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow
	if s == nil || s.pool == nil {
		return zero, errors.New("postgres: nil store")
	}
	code := strings.TrimSpace(slotCode)
	if machineID == uuid.Nil || code == "" {
		return zero, errors.Join(appcommerce.ErrInvalidArgument, errors.New("machine_id and slot_code required"))
	}
	rows, err := db.New(s.pool).InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return zero, err
	}
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.SlotCode), code) {
			return row, nil
		}
	}
	return zero, appcommerce.ErrNotFound
}

// LookupSlotDisplay returns current slot identity for an order line without re-checking assortment (replay / read enrichment).
func (s *Store) LookupSlotDisplay(ctx context.Context, companyID, machineID, productID uuid.UUID, slotIndex int32) (appcommerce.ResolvedSaleLine, error) {
	idx := slotIndex
	return s.ResolveSaleLine(ctx, appcommerce.ResolveSaleLineInput{
		MachineID: machineID,
		ProductID: productID,
		SlotIndex: &idx,
	})
}

// EvaluateSaleLineAt evaluates a sale line at a fixed time (idempotency / tests).
func (s *Store) EvaluateSaleLineAt(ctx context.Context, in appcommerce.ResolveSaleLineInput, at time.Time, qty int32) (appcommerce.ResolvedSaleLine, error) {
	if s == nil || s.pool == nil {
		return appcommerce.ResolvedSaleLine{}, errors.New("postgres: nil store")
	}
	eng := s.pricing
	if eng == nil {
		eng = pricingengine.New(s.pool)
	}
	sel := pricingengine.SaleLineSelector{
		MachineID:   in.MachineID,
		ProductID:   in.ProductID,
		SlotID:      in.SlotID,
		CabinetCode: in.CabinetCode,
		SlotCode:    in.SlotCode,
		SlotIndex:   in.SlotIndex,
	}
	line, row, err := eng.EvaluateSaleLine(ctx, sel, at.UTC(), qty)
	if err != nil {
		return appcommerce.ResolvedSaleLine{}, err
	}
	si, ok := slotIdxFromInventoryRow(row)
	if !ok {
		return appcommerce.ResolvedSaleLine{}, errors.New("postgres: slot config has no slot_index")
	}
	return pricingengine.MapToResolvedSaleLine(row.ID, row.CabinetCode, row.SlotCode, si, line), nil
}
