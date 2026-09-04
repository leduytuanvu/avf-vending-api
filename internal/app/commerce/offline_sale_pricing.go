package commerce

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

const (
	PricingSourceServerPriced           = "server_priced"
	PricingSourceMachineLocalVerified   = "machine_local_verified"
	PricingSourceMachineLocalUnverified = "machine_local_unverified"
)

// MachinePricingSnapshotInput is the machine-sealed checkout price for offline replay.
type MachinePricingSnapshotInput struct {
	SubtotalMinor        int64
	TaxMinor             int64
	TotalMinor           int64
	UnitPriceMinor       int64
	LocalPricingRevision int64
	PricingFingerprint   string
	CapturedAt           time.Time
}

// MachinePricingSnapshotFromProto maps the gRPC pricing snapshot into app-layer input.
func MachinePricingSnapshotFromProto(snap *machinev1.MachinePricingSnapshot) (MachinePricingSnapshotInput, error) {
	return machinePricingSnapshotFromProto(snap)
}

func machinePricingSnapshotFromProto(snap *machinev1.MachinePricingSnapshot) (MachinePricingSnapshotInput, error) {
	if snap == nil {
		return MachinePricingSnapshotInput{}, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot is required"))
	}
	out := MachinePricingSnapshotInput{
		SubtotalMinor:        snap.GetSubtotalMinor(),
		TaxMinor:             snap.GetTaxMinor(),
		TotalMinor:           snap.GetTotalMinor(),
		UnitPriceMinor:       snap.GetUnitPriceMinor(),
		LocalPricingRevision: snap.GetLocalPricingRevision(),
		PricingFingerprint:   strings.TrimSpace(snap.GetPricingFingerprint()),
	}
	if ts := snap.GetCapturedAt(); ts != nil && ts.IsValid() {
		out.CapturedAt = ts.AsTime().UTC()
	}
	return out, nil
}

// ValidateReplayPricingSnapshot ensures an idempotent replay carries the same declared amounts.
func ValidateReplayPricingSnapshot(snap *MachinePricingSnapshotInput, order domaincommerce.Order) error {
	return validateReplayPricingSnapshot(snap, order)
}

func validateReplayPricingSnapshot(snap *MachinePricingSnapshotInput, order domaincommerce.Order) error {
	if snap == nil || !isMachineLocalPricingSource(order.PricingSource) {
		return nil
	}
	if snap.TotalMinor != order.TotalMinor || snap.SubtotalMinor != order.SubtotalMinor || snap.TaxMinor != order.TaxMinor {
		return ErrIdempotencyPayloadConflict
	}
	return nil
}

func validateMachinePricingSnapshot(snap MachinePricingSnapshotInput) error {
	if snap.TotalMinor <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot total_minor must be positive"))
	}
	if snap.SubtotalMinor < 0 || snap.TaxMinor < 0 {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot amounts must be non-negative"))
	}
	if snap.SubtotalMinor+snap.TaxMinor != snap.TotalMinor {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot line sum does not match total_minor"))
	}
	if snap.UnitPriceMinor <= 0 {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot unit_price_minor must be positive"))
	}
	if snap.UnitPriceMinor != snap.SubtotalMinor {
		return errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot unit_price_minor must match subtotal_minor for single-line order"))
	}
	return nil
}

func machinePricingSnapshotJSON(snap MachinePricingSnapshotInput) ([]byte, error) {
	payload := map[string]any{
		"subtotal_minor":         snap.SubtotalMinor,
		"tax_minor":              snap.TaxMinor,
		"total_minor":            snap.TotalMinor,
		"unit_price_minor":       snap.UnitPriceMinor,
		"local_pricing_revision": snap.LocalPricingRevision,
		"pricing_fingerprint":    snap.PricingFingerprint,
	}
	if !snap.CapturedAt.IsZero() {
		payload["captured_at"] = snap.CapturedAt.UTC().Format(time.RFC3339Nano)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Join(ErrInvalidArgument, errors.New("pricing_snapshot could not be encoded"))
	}
	return raw, nil
}

func classifyMachineLocalPricingSource(snap MachinePricingSnapshotInput, server ResolvedSaleLine) string {
	if snap.UnitPriceMinor == server.PriceMinor &&
		snap.SubtotalMinor == server.SubtotalMinor &&
		snap.TotalMinor == server.TotalMinor {
		return PricingSourceMachineLocalVerified
	}
	return PricingSourceMachineLocalUnverified
}

func isMachineLocalPricingSource(source string) bool {
	switch strings.TrimSpace(source) {
	case PricingSourceMachineLocalVerified, PricingSourceMachineLocalUnverified:
		return true
	default:
		return false
	}
}
