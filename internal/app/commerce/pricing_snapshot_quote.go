package commerce

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	pricingSnapshotMaxSkew       = 24 * time.Hour
	pricingMaxUnitPriceMinor     = 100_000_000
)

type mirrorSlotPrice struct {
	SlotCode   string `json:"slotCode"`
	ProductID  string `json:"productId"`
	PriceMinor int64  `json:"priceMinor"`
}

func validateMachinePricingSnapshotMultiLine(snap MachinePricingSnapshotInput, lineCount int) error {
	if err := validateMachinePricingSnapshotTotals(snap); err != nil {
		return err
	}
	if len(snap.Lines) == 0 {
		if lineCount == 1 {
			return validateMachinePricingSnapshot(snap)
		}
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot lines required for multi-line quote"))
	}
	if len(snap.Lines) != lineCount {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot line count mismatch"))
	}
	if !snap.CapturedAt.IsZero() {
		age := time.Now().UTC().Sub(snap.CapturedAt.UTC())
		if age < 0 || age > pricingSnapshotMaxSkew {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot captured_at outside allowed skew"))
		}
	}
	var lineSum int64
	seenSeq := map[int32]struct{}{}
	for _, ln := range snap.Lines {
		if ln.LineSequence <= 0 {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot line_sequence must be positive"))
		}
		if _, dup := seenSeq[ln.LineSequence]; dup {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot duplicate line_sequence"))
		}
		seenSeq[ln.LineSequence] = struct{}{}
		qty := ln.Quantity
		if qty <= 0 {
			qty = 1
		}
		if ln.UnitPriceMinor <= 0 || ln.UnitPriceMinor > pricingMaxUnitPriceMinor {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot unit_price_minor out of range"))
		}
		if ln.LineSubtotalMinor != ln.UnitPriceMinor*int64(qty) {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot line subtotal mismatch"))
		}
		lineSum += ln.LineSubtotalMinor
	}
	if lineSum != snap.SubtotalMinor {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot subtotal does not match line sum"))
	}
	return nil
}

func validateMachinePricingSnapshotTotals(snap MachinePricingSnapshotInput) error {
	if snap.TotalMinor <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot total_minor must be positive"))
	}
	if snap.SubtotalMinor < 0 || snap.TaxMinor < 0 {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot amounts must be non-negative"))
	}
	if snap.SubtotalMinor+snap.TaxMinor != snap.TotalMinor {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot line sum does not match total_minor"))
	}
	if snap.LocalPricingRevision < 0 {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot local_pricing_revision must be non-negative"))
	}
	return nil
}

func classifyMachineLocalPricingSourceFromMirror(snap MachinePricingSnapshotInput, mirror LocalLayoutMirror) string {
	if len(snap.Lines) == 0 {
		return PricingSourceMachineLocalUnverified
	}
	prices, err := parseMirrorSlotPrices(mirror.SlotsJSON)
	if err != nil || len(prices) == 0 {
		return PricingSourceMachineLocalUnverified
	}
	if snap.LocalPricingRevision > 0 && mirror.Revision > 0 && snap.LocalPricingRevision < int64(mirror.Revision) {
		return PricingSourceMachineLocalUnverified
	}
	for _, ln := range snap.Lines {
		key := mirrorSlotKey(ln.SlotCode, ln.ProductID.String())
		mirrorPrice, ok := prices[key]
		if !ok || mirrorPrice != ln.UnitPriceMinor {
			return PricingSourceMachineLocalUnverified
		}
	}
	return PricingSourceMachineLocalVerified
}

func parseMirrorSlotPrices(slotsJSON []byte) (map[string]int64, error) {
	trim := strings.TrimSpace(string(slotsJSON))
	if trim == "" || trim == "[]" {
		return nil, nil
	}
	var slots []mirrorSlotPrice
	if err := json.Unmarshal(slotsJSON, &slots); err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(slots))
	for _, slot := range slots {
		code := strings.TrimSpace(slot.SlotCode)
		if code == "" {
			continue
		}
		out[mirrorSlotKey(code, strings.TrimSpace(slot.ProductID))] = slot.PriceMinor
	}
	return out, nil
}

func mirrorSlotKey(slotCode, productID string) string {
	return strings.ToUpper(slotCode) + "|" + strings.TrimSpace(productID)
}

func snapshotLineUnitPrice(snap MachinePricingSnapshotInput, seq int32, fallback int64) int64 {
	for _, ln := range snap.Lines {
		if ln.LineSequence == seq {
			return ln.UnitPriceMinor
		}
	}
	return fallback
}
