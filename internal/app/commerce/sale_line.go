package commerce

import (
	"context"

	"github.com/google/uuid"
)

// ResolveSaleLineInput selects a vendable slot on a machine for QR/checkout flows.
// Exactly one selector must be set: SlotID, or CabinetCode+SlotCode, or SlotIndex (deprecated).
type ResolveSaleLineInput struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	ProductID      uuid.UUID
	SlotID         *uuid.UUID
	CabinetCode    string
	SlotCode       string
	SlotIndex      *int32
}

// ResolvedSaleLine is authoritative slot identity and pricing from the published assortment + current slot config.
type ResolvedSaleLine struct {
	SlotConfigID  uuid.UUID
	CabinetCode   string
	SlotCode      string
	SlotIndex     int32
	PriceMinor    int64
	SubtotalMinor int64
	TaxMinor      int64
	TotalMinor    int64

	// Populated when resolved via pricingengine (promotions + machine overrides).
	BasePriceMinor      int64
	DiscountUnitMinor   int64
	PromotionLabel      string
	Currency            string
	PricingFingerprint  string
	AppliedPromotionIDs []uuid.UUID
}

// SaleLineResolver loads slot config rows and enforces published-assortment membership for the product.
type SaleLineResolver interface {
	ResolveSaleLine(ctx context.Context, in ResolveSaleLineInput) (ResolvedSaleLine, error)
	LookupSlotDisplay(ctx context.Context, organizationID, machineID, productID uuid.UUID, slotIndex int32) (ResolvedSaleLine, error)
}
