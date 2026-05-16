package grpcserver

import (
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
)

func TestNormalizeOTAStatus_AllowsOnlySafeLifecycleStates(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"acked":      "acked",
		"downloaded": "downloaded",
		"installed":  "installed",
		"success":    "installed",
		"failed":     "failed",
		"shell":      "",
		"reboot":     "",
		"":           "",
	}
	for in, want := range cases {
		if got := normalizeOTAStatus(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestDeviceReportOTAResultQueryScopesMachine(t *testing.T) {
	t.Parallel()
	sql := db.DeviceReportOTAResult
	for _, want := range []string{
		"r.machine_id = $3",
		"r.campaign_id = $4",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("DeviceReportOTAResult missing machine predicate %q", want)
		}
	}
}
