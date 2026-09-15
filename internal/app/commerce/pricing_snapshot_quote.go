package commerce

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

const (
	pricingSnapshotMaxSkew       = 24 * time.Hour
	pricingSnapshotMaxFutureSkew = 5 * time.Minute
	pricingMaxUnitPriceMinor     = 100_000_000
)

type mirrorSlotPrice struct {
	SlotCode             string `json:"slotCode"`
	ProductID            string `json:"productId"`
	PriceMinor           int64  `json:"priceMinor"`
	LocalPricingRevision int64  `json:"localPricingRevision"`
}

func validateMachinePricingSnapshotMultiLine(snap MachinePricingSnapshotInput, lineCount int) error {
	if err := validateMachinePricingSnapshotTotals(snap); err != nil {
		return err
	}
	if len(snap.Lines) == 0 {
		if lineCount == 1 {
			return validateMachinePricingSnapshotLegacySingleLine(snap)
		}
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot lines required for multi-line quote"))
	}
	if lineCount > 0 && len(snap.Lines) != lineCount {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot line count mismatch"))
	}
	if !snap.CapturedAt.IsZero() {
		age := time.Now().UTC().Sub(snap.CapturedAt.UTC())
		if age < -pricingSnapshotMaxFutureSkew || age > pricingSnapshotMaxSkew {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot captured_at outside allowed skew"))
		}
	}
	var lineSum int64
	seenSeq := map[int32]struct{}{}
	seenIdentity := map[string]struct{}{}
	for _, ln := range snap.Lines {
		if ln.LineSequence <= 0 {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot line_sequence must be positive"))
		}
		if _, dup := seenSeq[ln.LineSequence]; dup {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot duplicate line_sequence"))
		}
		seenSeq[ln.LineSequence] = struct{}{}
		identity := strings.ToUpper(strings.TrimSpace(ln.CabinetCode)) + "|" +
			strings.ToUpper(strings.TrimSpace(ln.SlotCode)) + "|" + ln.ProductID.String()
		if _, dup := seenIdentity[identity]; dup {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot duplicate line identity"))
		}
		seenIdentity[identity] = struct{}{}
		if ln.Quantity <= 0 {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot quantity must be positive"))
		}
		qty := int64(ln.Quantity)
		if ln.UnitPriceMinor <= 0 || ln.UnitPriceMinor > pricingMaxUnitPriceMinor {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot unit_price_minor out of range"))
		}
		if ln.UnitPriceMinor > math.MaxInt64/qty {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot line subtotal overflow"))
		}
		if ln.LineSubtotalMinor != ln.UnitPriceMinor*qty {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot line subtotal mismatch"))
		}
		if lineSum > math.MaxInt64-ln.LineSubtotalMinor {
			return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot subtotal overflow"))
		}
		lineSum += ln.LineSubtotalMinor
	}
	if lineSum != snap.SubtotalMinor {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot subtotal does not match line sum"))
	}
	if len(snap.Lines) == 1 && snap.UnitPriceMinor > 0 && snap.UnitPriceMinor != snap.Lines[0].UnitPriceMinor {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot unit_price_minor must match the single line unit price"))
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
	for _, ln := range snap.Lines {
		key := mirrorSlotKey(ln.SlotCode, ln.ProductID.String())
		mirrorSlot, ok := prices[key]
		if !ok || mirrorSlot.PriceMinor != ln.UnitPriceMinor {
			return PricingSourceMachineLocalUnverified
		}
		if snap.LocalPricingRevision > 0 && mirrorSlot.LocalPricingRevision > 0 &&
			mirrorSlot.LocalPricingRevision < snap.LocalPricingRevision {
			return PricingSourceMachineLocalUnverified
		}
	}
	return PricingSourceMachineLocalVerified
}

func parseMirrorSlotPrices(slotsJSON []byte) (map[string]mirrorSlotPrice, error) {
	trim := strings.TrimSpace(string(slotsJSON))
	if trim == "" || trim == "[]" {
		return nil, nil
	}
	var slots []mirrorSlotPrice
	if err := json.Unmarshal(slotsJSON, &slots); err != nil {
		return nil, err
	}
	out := make(map[string]mirrorSlotPrice, len(slots))
	for _, slot := range slots {
		code := strings.TrimSpace(slot.SlotCode)
		if code == "" {
			continue
		}
		out[mirrorSlotKey(code, strings.TrimSpace(slot.ProductID))] = slot
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
