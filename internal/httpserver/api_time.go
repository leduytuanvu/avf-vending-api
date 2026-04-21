package httpserver

import "time"

// formatAPITimeRFC3339Nano formats an instant for JSON responses: UTC, fractional seconds, and explicit offset (Z).
func formatAPITimeRFC3339Nano(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func formatAPITimeRFC3339NanoPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatAPITimeRFC3339Nano(*t)
	return &s
}
