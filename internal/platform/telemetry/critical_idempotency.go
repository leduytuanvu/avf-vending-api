package telemetry

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ErrCriticalTelemetryMissingIdentity indicates a critical telemetry event lacked
// dedupe_key, event_id, and boot_id+seq_no (all are required to be absent for this error).
var ErrCriticalTelemetryMissingIdentity = errors.New("telemetry: critical event missing stable idempotency identity (set dedupe_key, event_id, or boot_id+seq_no)")

// CriticalIngestIdentity carries wire fields used to compute a stable idempotency key for critical telemetry.
type CriticalIngestIdentity struct {
	DedupeKey *string
	EventID   string
	BootID    *uuid.UUID
	SeqNo     *int64
}

// StableCriticalIdempotencyKey returns a dedupe string for critical_no_drop telemetry.
// Preference order: non-empty dedupe_key (trimmed, as provided by the device for compatibility),
// then machine-scoped event_id, then machine+boot_id+seq_no+normalized_event_type.
func StableCriticalIdempotencyKey(machineID uuid.UUID, eventType string, id CriticalIngestIdentity) (string, error) {
	if machineID == uuid.Nil {
		return "", fmt.Errorf("telemetry: machine_id is required")
	}
	et := strings.TrimSpace(strings.ToLower(eventType))
	if id.DedupeKey != nil {
		if s := strings.TrimSpace(*id.DedupeKey); s != "" {
			return s, nil
		}
	}
	if eid := strings.TrimSpace(id.EventID); eid != "" {
		return fmt.Sprintf("%s:%s:%s", machineID.String(), et, eid), nil
	}
	if id.BootID != nil && id.SeqNo != nil {
		return fmt.Sprintf("%s:%s:%d:%s", machineID.String(), id.BootID.String(), *id.SeqNo, et), nil
	}
	return "", ErrCriticalTelemetryMissingIdentity
}
