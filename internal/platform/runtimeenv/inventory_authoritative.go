package runtimeenv

import (
	"os"
	"strings"
)

// InventoryAuthoritativeServer is true when the API should accept web/app planogram publish
// and inventory adjustment writes. Set INVENTORY_AUTHORITATIVE_SERVER=false for snapshot-only.
func InventoryAuthoritativeServer() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("INVENTORY_AUTHORITATIVE_SERVER")))
	switch v {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
