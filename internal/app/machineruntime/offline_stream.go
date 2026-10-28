package machineruntime

import (
	"strings"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

const (
	LegacyOfflineStreamName   = "offline"
	OfflineStreamKindCash     = "cash"
	OfflineStreamKindCommerce = "commerce"
)

// OfflineStreamName returns the machine_sync_cursors stream key for an offline push/get-cursor RPC.
// Non-empty stream_id isolates cursor per app installation epoch; legacy clients keep "offline".
func OfflineStreamName(meta *machinev1.MachineRequestMeta) string {
	if meta == nil {
		return LegacyOfflineStreamName
	}
	streamID := strings.TrimSpace(meta.GetStreamId())
	if streamID == "" {
		return LegacyOfflineStreamName
	}
	return streamID
}

// QualifyOfflineStreamName appends :cash or :commerce so the two ledgers do not share a cursor.
func QualifyOfflineStreamName(streamID, kind string) string {
	base := strings.TrimSpace(streamID)
	if base == "" {
		base = LegacyOfflineStreamName
	}
	normalizedKind := strings.TrimSpace(kind)
	if normalizedKind == "" {
		return base
	}
	for _, suffix := range []string{OfflineStreamKindCash, OfflineStreamKindCommerce} {
		if strings.HasSuffix(base, ":"+suffix) {
			base = strings.TrimSuffix(base, ":"+suffix)
			break
		}
	}
	return base + ":" + normalizedKind
}
