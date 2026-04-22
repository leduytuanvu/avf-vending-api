package httpserver

import (
	"fmt"
	"strings"
	"time"
)

// formatAPITimeRFC3339Nano formats an instant for JSON responses using RFC3339
// with an explicit timezone offset. Fractional seconds are preserved when present.
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

// parseAPITimeRFC3339 accepts RFC3339 with or without fractional seconds and requires a timezone offset.
func parseAPITimeRFC3339(raw string) (time.Time, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return time.Time{}, fmt.Errorf("time is required")
	}
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return t, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}
