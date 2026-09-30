package commerceadmin

import (
	"encoding/json"
	"strings"
)

type storedPricingSnapshot struct {
	UnitPriceMinor int64                       `json:"unit_price_minor"`
	Lines          []storedPricingSnapshotLine `json:"lines"`
}

type storedPricingSnapshotLine struct {
	LineSequence      int32  `json:"line_sequence"`
	ProductID         string `json:"product_id"`
	SlotCode          string `json:"slot_code"`
	SlotIndex         int32  `json:"slot_index"`
	Quantity          int32  `json:"quantity"`
	UnitPriceMinor    int64  `json:"unit_price_minor"`
	LineSubtotalMinor int64  `json:"line_subtotal_minor"`
}

func parseMachinePricingSnapshot(raw []byte) (storedPricingSnapshot, bool) {
	if len(raw) == 0 {
		return storedPricingSnapshot{}, false
	}
	var snap storedPricingSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return storedPricingSnapshot{}, false
	}
	if len(snap.Lines) == 0 && snap.UnitPriceMinor <= 0 {
		return storedPricingSnapshot{}, false
	}
	return snap, true
}

func snapshotLineForItem(
	snap storedPricingSnapshot,
	lineSequence int32,
	productID string,
	slotCode string,
	slotIndex int32,
) (unitPriceMinor int64, lineSubtotalMinor int64, ok bool) {
	productID = strings.TrimSpace(productID)
	slotCode = strings.TrimSpace(slotCode)

	for _, ln := range snap.Lines {
		if ln.LineSequence == lineSequence && ln.LineSequence > 0 {
			return snapshotLineAmounts(ln)
		}
	}
	for _, ln := range snap.Lines {
		if strings.TrimSpace(ln.ProductID) != productID || productID == "" {
			continue
		}
		if ln.SlotIndex > 0 && ln.SlotIndex == slotIndex {
			return snapshotLineAmounts(ln)
		}
		if slotCode != "" && strings.TrimSpace(ln.SlotCode) == slotCode {
			return snapshotLineAmounts(ln)
		}
	}
	if len(snap.Lines) == 1 && snap.UnitPriceMinor > 0 {
		unit := snap.UnitPriceMinor
		return unit, unit, true
	}
	if len(snap.Lines) == 0 && snap.UnitPriceMinor > 0 {
		unit := snap.UnitPriceMinor
		return unit, unit, true
	}
	return 0, 0, false
}

func snapshotLineAmounts(ln storedPricingSnapshotLine) (int64, int64, bool) {
	unit := ln.UnitPriceMinor
	lineTotal := ln.LineSubtotalMinor
	qty := ln.Quantity
	if qty <= 0 {
		qty = 1
	}
	if unit <= 0 && lineTotal > 0 {
		unit = lineTotal / int64(qty)
	}
	if unit <= 0 {
		return 0, 0, false
	}
	// Order detail rows are one vend session each (quantity 1).
	return unit, unit, true
}

func applySnapshotPricingToItems(items []OrderLineItemDetail, raw []byte) {
	snap, ok := parseMachinePricingSnapshot(raw)
	if !ok {
		return
	}
	for i := range items {
		if items[i].UnitPriceMinor > 0 || items[i].LineSubtotalMinor > 0 {
			continue
		}
		unit, lineTotal, matched := snapshotLineForItem(
			snap,
			items[i].LineSequence,
			items[i].ProductID,
			items[i].SlotCode,
			items[i].SlotIndex,
		)
		if !matched {
			continue
		}
		items[i].UnitPriceMinor = unit
		items[i].LineSubtotalMinor = lineTotal
	}
}

func normalizeOrderLinePricing(item OrderLineItemDetail) OrderLineItemDetail {
	unitPrice := item.UnitPriceMinor
	lineTotal := item.LineSubtotalMinor
	qty := item.Quantity
	if qty <= 0 {
		qty = 1
		item.Quantity = qty
	}
	if unitPrice <= 0 && lineTotal > 0 {
		unitPrice = lineTotal / int64(qty)
	}
	if lineTotal <= 0 && unitPrice > 0 {
		lineTotal = unitPrice * int64(qty)
	}
	item.UnitPriceMinor = unitPrice
	item.LineSubtotalMinor = lineTotal
	return item
}
