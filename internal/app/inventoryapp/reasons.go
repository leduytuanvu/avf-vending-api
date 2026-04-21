package inventoryapp

import (
	"errors"
	"strings"
)

var ErrInvalidStockAdjustmentReason = errors.New("inventoryapp: invalid stock adjustment reason")

// StockAdjustmentReasonToEventType maps API reasons to inventory_events.event_type (DB CHECK).
func StockAdjustmentReasonToEventType(reason string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "restock":
		return "restock", nil
	case "cycle_count":
		return "count", nil
	case "manual_adjustment":
		return "adjustment", nil
	case "machine_reconcile":
		return "reconcile", nil
	default:
		return "", ErrInvalidStockAdjustmentReason
	}
}
