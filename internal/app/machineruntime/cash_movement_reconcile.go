package machineruntime

import (
	"context"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// DeviceEventIDFromCashMovementIdempotencyKey extracts the forensic device_event_id
// from outbox keys such as cash_movement:<machineId>:<hex>,
// cash_movement:<machineId>:<bootId>:<hex>[:episode], and v2/v3 variants.
// Kind suffixes (:bill_credit, :bill_lifecycle:escrow_held) and v3 recovery
// nonces (digit tails longer than 8) are stripped. Boot and episode stay in
// the returned id so existence checks match boot-scoped acceptance rows.
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
	if i := strings.Index(rest, "payout:"); i >= 0 {
		payoutID := strings.TrimSpace(rest[i:])
		return strings.TrimSuffix(payoutID, ":payout")
	}
	return stripCashMovementKindAndNonce(rest)
}

// BootIDFromCashMovementIdempotencyKey returns the boot prefix of a boot-scoped
// cash_movement key, or empty when the key has no boot segment.
func BootIDFromCashMovementIdempotencyKey(machineID uuid.UUID, idempotencyKey string) string {
	id := DeviceEventIDFromCashMovementIdempotencyKey(machineID, idempotencyKey)
	if id == "" || strings.HasPrefix(id, "payout:") {
		return ""
	}
	idx := strings.IndexByte(id, ':')
	if idx <= 0 {
		return ""
	}
	return id[:idx]
}

func CashMovementForensicRowExists(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	idempotencyKey string,
) (bool, error) {
	key := strings.TrimSpace(idempotencyKey)
	deviceID := DeviceEventIDFromCashMovementIdempotencyKey(machineID, key)
	if deviceID == "" {
		return true, nil
	}
	if strings.Contains(key, ":payout:") || strings.HasPrefix(deviceID, "payout:") {
		return q.CashPayoutExistsForMachineDevice(ctx, db.CashPayoutExistsForMachineDeviceParams{
			MachineID:     machineID,
			DeviceEventID: deviceID,
		})
	}
	if strings.Contains(key, ":bill_lifecycle:") {
		return q.CashBillLifecycleExistsForMachineDevice(ctx, db.CashBillLifecycleExistsForMachineDeviceParams{
			MachineID:     machineID,
			DeviceEventID: deviceID,
		})
	}
	if strings.Contains(key, ":recycler_observation") {
		return true, nil
	}
	return q.CashAcceptanceExistsForMachineDevice(ctx, db.CashAcceptanceExistsForMachineDeviceParams{
		MachineID:     machineID,
		DeviceEventID: deviceID,
	})
}

func stripCashMovementKindAndNonce(rest string) string {
	trimmed := strings.TrimSpace(rest)
	if i := strings.Index(trimmed, ":bill_lifecycle:"); i >= 0 {
		trimmed = trimmed[:i]
	} else if i := strings.Index(trimmed, ":bill_credit"); i >= 0 {
		trimmed = trimmed[:i]
	} else if i := strings.Index(trimmed, ":recycler_observation"); i >= 0 {
		trimmed = trimmed[:i]
	}
	// v3 recovery nonce is nanoTime (>> 8 digits). Keep short episode seq tails.
	if i := strings.LastIndex(trimmed, ":"); i >= 0 {
		tail := trimmed[i+1:]
		if tail != "" && isAllDigits(tail) && len(tail) > 8 {
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
