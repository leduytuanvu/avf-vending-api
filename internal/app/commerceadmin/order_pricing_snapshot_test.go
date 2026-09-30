package commerceadmin

import (
	"encoding/json"
	"testing"
)

func TestApplySnapshotPricingToItems_multiLineLocalPrices(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"schema_version": 2,
		"subtotal_minor": 35000,
		"total_minor":    35000,
		"lines": []map[string]any{
			{
				"line_sequence":       1,
				"product_id":          "7up-product-id",
				"slot_code":           "49",
				"slot_index":          49,
				"quantity":            1,
				"unit_price_minor":    10000,
				"line_subtotal_minor": 10000,
			},
			{
				"line_sequence":       2,
				"product_id":          "7up-product-id",
				"slot_code":           "50",
				"slot_index":          50,
				"quantity":            1,
				"unit_price_minor":    10000,
				"line_subtotal_minor": 10000,
			},
			{
				"line_sequence":       3,
				"product_id":          "7up-product-id",
				"slot_code":           "51",
				"slot_index":          51,
				"quantity":            1,
				"unit_price_minor":    10000,
				"line_subtotal_minor": 10000,
			},
			{
				"line_sequence":       4,
				"product_id":          "aqua-product-id",
				"slot_code":           "52",
				"slot_index":          52,
				"quantity":            1,
				"unit_price_minor":    5000,
				"line_subtotal_minor": 5000,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	items := []OrderLineItemDetail{
		{LineSequence: 1, ProductID: "7up-product-id", SlotCode: "49", SlotIndex: 49, Quantity: 1},
		{LineSequence: 2, ProductID: "7up-product-id", SlotCode: "50", SlotIndex: 50, Quantity: 1},
		{LineSequence: 3, ProductID: "7up-product-id", SlotCode: "51", SlotIndex: 51, Quantity: 1},
		{LineSequence: 4, ProductID: "aqua-product-id", SlotCode: "52", SlotIndex: 52, Quantity: 1},
	}
	applySnapshotPricingToItems(items, raw)

	if items[0].UnitPriceMinor != 10000 || items[3].UnitPriceMinor != 5000 {
		t.Fatalf("unexpected unit prices: %#v", items)
	}
	if items[0].LineSubtotalMinor != 10000 || items[3].LineSubtotalMinor != 5000 {
		t.Fatalf("unexpected line totals: %#v", items)
	}

	// Must not fabricate equal split (35000/4 = 8750).
	for _, item := range items {
		if item.UnitPriceMinor == 8750 {
			t.Fatalf("equal-split fallback leaked into snapshot pricing: %#v", items)
		}
	}
}

func TestApplySnapshotPricingToItems_preservesExistingQuotePricing(t *testing.T) {
	items := []OrderLineItemDetail{
		{
			LineSequence:      1,
			ProductID:         "7up-product-id",
			SlotCode:          "49",
			SlotIndex:         49,
			Quantity:          1,
			UnitPriceMinor:    10000,
			LineSubtotalMinor: 10000,
		},
	}
	raw, err := json.Marshal(map[string]any{
		"lines": []map[string]any{
			{
				"line_sequence":    1,
				"product_id":       "7up-product-id",
				"unit_price_minor": 5000,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	applySnapshotPricingToItems(items, raw)
	if items[0].UnitPriceMinor != 10000 {
		t.Fatalf("expected quote pricing to win, got %#v", items[0])
	}
}

func TestApplySnapshotPricingToItems_singleLineLegacySnapshot(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"unit_price_minor": 12000,
		"subtotal_minor":   12000,
		"total_minor":      12000,
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	items := []OrderLineItemDetail{
		{LineSequence: 1, ProductID: "prod-1", SlotCode: "A1", SlotIndex: 1, Quantity: 1},
	}
	applySnapshotPricingToItems(items, raw)
	if items[0].UnitPriceMinor != 12000 || items[0].LineSubtotalMinor != 12000 {
		t.Fatalf("unexpected legacy snapshot pricing: %#v", items[0])
	}
}
