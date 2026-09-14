package commerce

import (
	"context"
	"errors"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// IsAssortmentStaleError reports server published assortment / slot config lag vs machine-local checkout.
func IsAssortmentStaleError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no matching current slot config") ||
		strings.Contains(msg, "product is not in the machine's published assortment")
}

type slotConfigByCodeReader interface {
	LookupCurrentSlotConfigByCode(ctx context.Context, machineID uuid.UUID, slotCode string) (db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow, error)
}

// resolveCheckoutSaleLine resolves a vend line for checkout, falling back to the machine pricing
// snapshot + local layout mirror when the published assortment has not caught up yet.
func (s *Service) resolveCheckoutSaleLine(
	ctx context.Context,
	in ResolveSaleLineInput,
	snap *MachinePricingSnapshotInput,
	qty int32,
) (ResolvedSaleLine, error) {
	if s == nil || s.saleLines == nil {
		return ResolvedSaleLine{}, ErrNotConfigured
	}
	var (
		resolved ResolvedSaleLine
		err      error
	)
	if qa, ok := s.saleLines.(QuantityAwareSaleLineResolver); ok {
		resolved, err = qa.ResolveSaleLineWithQuantity(ctx, in, qty)
	} else {
		resolved, err = s.saleLines.ResolveSaleLine(ctx, in)
		if err == nil && qty > 1 {
			resolved.TotalMinor = resolved.PriceMinor * int64(qty)
			resolved.SubtotalMinor = resolved.TotalMinor
		}
	}
	if err == nil {
		return resolved, nil
	}
	if snap == nil || !IsAssortmentStaleError(err) {
		return ResolvedSaleLine{}, err
	}
	return s.resolveSaleLineFromPricingSnapshot(ctx, in, *snap, qty)
}

func (s *Service) resolveSaleLineFromPricingSnapshot(
	ctx context.Context,
	in ResolveSaleLineInput,
	snap MachinePricingSnapshotInput,
	qty int32,
) (ResolvedSaleLine, error) {
	if err := validateMachinePricingSnapshot(snap); err != nil {
		return ResolvedSaleLine{}, err
	}
	slotCode := strings.TrimSpace(in.SlotCode)
	if slotCode == "" {
		return ResolvedSaleLine{}, errors.Join(ErrInvalidArgument, errors.New("slot_code required for mirror checkout resolution"))
	}
	snapLine, ok := snapshotLineForSelector(snap, in)
	if !ok {
		return ResolvedSaleLine{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot missing line for slot selector"))
	}
	if qty <= 0 {
		qty = snapLine.Quantity
	}
	if qty <= 0 {
		qty = 1
	}
	unit := snapLine.UnitPriceMinor
	if unit <= 0 {
		unit = snap.UnitPriceMinor
	}
	if unit <= 0 {
		return ResolvedSaleLine{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot unit_price_minor invalid"))
	}
	lineTotal := unit * int64(qty)

	reader, ok := s.saleLines.(slotConfigByCodeReader)
	if !ok {
		return ResolvedSaleLine{}, errors.Join(ErrInvalidArgument, errors.New("slot topology lookup unavailable"))
	}
	row, err := reader.LookupCurrentSlotConfigByCode(ctx, in.MachineID, slotCode)
	if err != nil {
		return ResolvedSaleLine{}, err
	}
	slotIdx, ok := slotIndexFromInventoryRow(row)
	if !ok {
		return ResolvedSaleLine{}, errors.Join(ErrInvalidArgument, errors.New("slot config has no slot_index"))
	}
	cab := strings.TrimSpace(row.CabinetCode)
	if cab == "" {
		cab = strings.TrimSpace(in.CabinetCode)
	}
	pricingSource := PricingSourceMachineLocalUnverified
	if s.layoutMirror != nil {
		if mirror, merr := s.layoutMirror.GetLocalLayoutMirror(ctx, in.MachineID); merr == nil {
			pricingSource = classifyMachineLocalPricingSourceFromMirror(snap, mirror)
		}
	}
	_ = pricingSource
	return ResolvedSaleLine{
		SlotConfigID:       row.ID,
		CabinetCode:        cab,
		SlotCode:           strings.TrimSpace(row.SlotCode),
		SlotIndex:          slotIdx,
		PriceMinor:         unit,
		SubtotalMinor:      lineTotal,
		TaxMinor:           0,
		TotalMinor:         lineTotal,
		Currency:           "",
		PricingFingerprint: strings.TrimSpace(snap.PricingFingerprint),
	}, nil
}

func snapshotLineForSelector(snap MachinePricingSnapshotInput, in ResolveSaleLineInput) (MachinePricingSnapshotLineInput, bool) {
	slotCode := strings.TrimSpace(in.SlotCode)
	productID := in.ProductID
	if len(snap.Lines) == 0 {
		if slotCode == "" || productID == uuid.Nil {
			return MachinePricingSnapshotLineInput{}, false
		}
		return MachinePricingSnapshotLineInput{
			LineSequence:   1,
			ProductID:      productID,
			SlotCode:       slotCode,
			Quantity:       1,
			UnitPriceMinor: snap.UnitPriceMinor,
			LineSubtotalMinor: snap.SubtotalMinor,
		}, true
	}
	for _, ln := range snap.Lines {
		if productID != uuid.Nil && ln.ProductID != uuid.Nil && ln.ProductID != productID {
			continue
		}
		if slotCode != "" && !strings.EqualFold(strings.TrimSpace(ln.SlotCode), slotCode) {
			continue
		}
		return ln, true
	}
	return MachinePricingSnapshotLineInput{}, false
}

func slotIndexFromInventoryRow(row db.InventoryAdminListCurrentMachineSlotConfigsByMachineRow) (int32, bool) {
	if !row.SlotIndex.Valid {
		return 0, false
	}
	return row.SlotIndex.Int32, true
}
