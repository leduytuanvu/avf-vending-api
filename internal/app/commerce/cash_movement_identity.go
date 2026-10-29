package commerce

import "strings"

// QualifyCashMovementDeviceEventID prefixes ICT hex (and hex:episode) with boot so
// (machine_id, device_event_id) uniqueness tracks one physical bill, not a wrapping register.
func QualifyCashMovementDeviceEventID(deviceEventID, bootID string) string {
	id := strings.TrimSpace(deviceEventID)
	boot := strings.TrimSpace(bootID)
	if id == "" || boot == "" {
		return id
	}
	if strings.HasPrefix(id, boot+":") {
		return id
	}
	if !shouldPrefixCashMovementBoot(id) {
		return id
	}
	return boot + ":" + id
}

func shouldPrefixCashMovementBoot(deviceEventID string) bool {
	id := strings.TrimSpace(deviceEventID)
	switch {
	case strings.HasPrefix(id, "payout:"),
		strings.HasPrefix(id, "bill_reject:"),
		strings.HasPrefix(id, "record_ambiguous:"),
		strings.HasPrefix(id, "returned:"),
		strings.HasPrefix(id, "escrow:"),
		strings.HasPrefix(id, "fault:"),
		strings.HasPrefix(id, "stacked:"),
		strings.HasPrefix(id, "stored:"):
		return false
	}
	if isIctHexDeviceEventID(id) {
		return true
	}
	colon := strings.IndexByte(id, ':')
	if colon != 6 || !isIctHexDeviceEventID(id[:6]) {
		return false
	}
	tail := id[7:]
	if tail == "" {
		return false
	}
	for _, r := range tail {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isIctHexDeviceEventID(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'f',
			r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
