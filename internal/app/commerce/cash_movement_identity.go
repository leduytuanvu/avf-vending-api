package commerce

import "strings"

// CanonicalizeCashMovementDeviceEventID strips a trailing `:episode` from ICT hex
// identities so one physical note maps to one (machine_id, device_event_id) row.
func CanonicalizeCashMovementDeviceEventID(deviceEventID string) string {
	id := strings.TrimSpace(deviceEventID)
	if id == "" {
		return id
	}
	last := strings.LastIndexByte(id, ':')
	if last <= 0 {
		return id
	}
	tail := id[last+1:]
	if tail == "" || len(tail) > 8 || !cashEpisodeDigits(tail) {
		return id
	}
	head := id[:last]
	hexPart := head
	if i := strings.LastIndexByte(head, ':'); i >= 0 {
		hexPart = head[i+1:]
	}
	if !isIctHexDeviceEventID(hexPart) {
		return id
	}
	return head
}

func cashEpisodeDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// QualifyCashMovementDeviceEventID prefixes ICT hex with boot so
// (machine_id, device_event_id) uniqueness tracks one physical bill, not a wrapping register.
func QualifyCashMovementDeviceEventID(deviceEventID, bootID string) string {
	id := CanonicalizeCashMovementDeviceEventID(deviceEventID)
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
