package machineruntime

import (
	"strings"

	"github.com/google/uuid"
)

// DeviceEventIDFromCashMovementIdempotencyKey extracts the hardware device_event_id suffix
// from outbox keys such as cash_movement:<machineId>:<hex> or cash_movement:v2:<machineId>:<hex>
// and cash_movement:<machineId>:<bootId>:<hex>. Kind suffixes (:bill_credit,
// :bill_lifecycle:escrow_held) and v3 recovery nonces are stripped so forensic
// matching still uses the ICT hex, not the movement kind.
func DeviceEventIDFromCashMovementIdempotencyKey(machineID uuid.UUID, idempotencyKey string) string {
	key := strings.TrimSpace(idempotencyKey)
	if !strings.HasPrefix(key, "cash_movement:") {
		return ""
	}
	rest := strings.TrimPrefix(key, "cash_movement:")
	switch {
	case strings.HasPrefix(rest, "v3:"):
		rest = strings.TrimPrefix(rest, "v3:")
	case strings.HasPrefix(rest, "v2:"):
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
	rest = stripCashMovementKindAndNonce(rest)
	// boot-scoped bill keys: <bootId>:<deviceHex> — device id is the last segment
	if strings.Contains(rest, ":") {
		parts := strings.Split(rest, ":")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return rest
}

func stripCashMovementKindAndNonce(rest string) string {
	trimmed := strings.TrimSpace(rest)
	strippedKind := false
	if i := strings.Index(trimmed, ":bill_lifecycle:"); i >= 0 {
		trimmed = trimmed[:i]
		strippedKind = true
	} else if i := strings.Index(trimmed, ":bill_credit"); i >= 0 {
		trimmed = trimmed[:i]
		strippedKind = true
	} else if i := strings.Index(trimmed, ":recycler_observation"); i >= 0 {
		trimmed = trimmed[:i]
		strippedKind = true
	}
	// v3 recovery nonce is nanoTime (>> 6-char ICT hex). Do not strip hex like 010005.
	if i := strings.LastIndex(trimmed, ":"); i >= 0 {
		tail := trimmed[i+1:]
		if tail != "" && isAllDigits(tail) && (strippedKind || len(tail) > 8) {
			trimmed = trimmed[:i]
		}
	}
	return strings.TrimSpace(trimmed)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
