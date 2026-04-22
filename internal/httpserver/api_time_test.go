package httpserver

import (
	"testing"
	"time"
)

func TestParseAPITimeRFC3339_AcceptsRFC3339AndNano(t *testing.T) {
	t.Parallel()

	cases := []string{
		"2026-04-22T10:11:12Z",
		"2026-04-22T10:11:12.123456789+07:00",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			got, err := parseAPITimeRFC3339(tc)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if roundTrip := got.Format(time.RFC3339Nano); roundTrip == "" {
				t.Fatal("expected round-trip RFC3339 string")
			}
		})
	}
}

func TestFormatAPITimeRFC3339Nano_UsesOffset(t *testing.T) {
	t.Parallel()

	got := formatAPITimeRFC3339Nano(time.Date(2026, 4, 22, 10, 11, 12, 123000000, time.FixedZone("UTC+7", 7*3600)))
	if got != "2026-04-22T03:11:12.123Z" {
		t.Fatalf("got %q", got)
	}
}
