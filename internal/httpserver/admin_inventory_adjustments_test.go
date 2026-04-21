package httpserver

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/app/inventoryapp"
)

func TestStockAdjustmentReasonToEventType(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{"restock", "restock"},
		{"RESTOCK", "restock"},
		{"cycle_count", "count"},
		{"manual_adjustment", "adjustment"},
		{"machine_reconcile", "reconcile"},
	}
	for _, tc := range cases {
		got, err := inventoryapp.StockAdjustmentReasonToEventType(tc.reason)
		if err != nil {
			t.Fatalf("%q: %v", tc.reason, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.reason, got, tc.want)
		}
	}
	_, err := inventoryapp.StockAdjustmentReasonToEventType("invalid")
	if err == nil {
		t.Fatal("expected error")
	}
}
