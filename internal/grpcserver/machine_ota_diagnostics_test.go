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

func TestDeviceReportOTAResultQueryScopesMachineAndOrg(t *testing.T) {
	t.Parallel()
	sql := db.DeviceReportOTAResult
	for _, want := range []string{
		"r.machine_id = $1",
		"r.campaign_id = $2",
		"c.organization_id = $3",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("DeviceReportOTAResult missing scope predicate %q", want)
		}
	}
}
