package pricingengine

import (
	"github.com/google/uuid"
)

// SaleLineSelector identifies one vendable slot line (same semantics as commerce.ResolveSaleLineInput).
type SaleLineSelector struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	ProductID      uuid.UUID
	SlotID         *uuid.UUID
	CabinetCode    string
	SlotCode       string
	SlotIndex      *int32
}

// PriceLineInput is one catalog/order line at evaluation time.
type PriceLineInput struct {
	OrganizationID uuid.UUID
	MachineID      uuid.UUID
	ProductID      uuid.UUID
	// SlotListUnitMinor is the planogram / slot-config list price before machine_price_overrides.
	SlotListUnitMinor int64
	SlotConfigID      uuid.UUID
	CabinetCode       string
	SlotCode          string
	SlotIndex         int32
	Quantity          int32
}

// LinePriceResult is the canonical priced line used by catalog and commerce.
type LinePriceResult struct {
	SlotListUnitMinor   int64
	RegisterUnitMinor   int64 // after machine override; promotion discounts apply to this base
	DiscountUnitMinor   int64
	EffectiveUnitMinor  int64
	SubtotalMinor       int64
	TaxMinor            int64
	TotalMinor          int64
	Currency            string
	AppliedPromotionIDs []uuid.UUID
	PromotionLabel      string
	PricingFingerprint  string
	CalcTrace           []string
}

// SkippedPromotionRule explains why a rule did not apply (admin preview / audit).
type SkippedPromotionRule struct {
	PromotionID uuid.UUID `json:"promotionId"`
	RuleID      uuid.UUID `json:"ruleId,omitempty"`
	RuleType    string    `json:"ruleType"`
	Reason      string    `json:"reason"`
}
