package machineruntime

import (
	"strings"

	"github.com/google/uuid"
)

// DeviceEventIDFromCashMovementIdempotencyKey extracts the hardware device_event_id suffix
// from outbox keys such as cash_movement:<machineId>:<hex> or cash_movement:v2:<machineId>:<hex>
// and cash_movement:<machineId>:<bootId>:<hex>.
func DeviceEventIDFromCashMovementIdempotencyKey(machineID uuid.UUID, idempotencyKey string) string {
	key := strings.TrimSpace(idempotencyKey)
	if !strings.HasPrefix(key, "cash_movement:") {
		return ""
	}
	rest := strings.TrimPrefix(key, "cash_movement:")
	if strings.HasPrefix(rest, "v2:") {
		rest = strings.TrimPrefix(rest, "v2:")
	}
	mid := machineID.String()
	if !strings.HasPrefix(rest, mid) {
		return ""
	}
	rest = strings.TrimPrefix(rest, mid)
	rest = strings.TrimPrefix(rest, ":")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, "payout:") {
		return rest
	}
	// boot-scoped bill keys: <bootId>:<deviceHex> — device id is the last segment
	if strings.Contains(rest, ":") {
		parts := strings.Split(rest, ":")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return rest
}
