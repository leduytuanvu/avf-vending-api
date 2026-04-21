package httpserver

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestV1AdminMachineListItem_JSON_keys(t *testing.T) {
	t.Parallel()
	item := V1AdminMachineListItem{
		MachineID:           "3fa85f64-5717-4562-b3fc-2c963f66afa6",
		MachineName:         "M1",
		OrganizationID:      "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		SiteID:              "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		SiteName:            "Dock",
		SerialNumber:        "SN-1",
		Name:                "M1",
		Status:              "online",
		CommandSequence:     1,
		CreatedAt:           "2026-01-02T03:04:05.123456789Z",
		UpdatedAt:           "2026-01-02T03:04:05.123456789Z",
		EffectiveTimezone:   "UTC",
		AssignedTechnicians: []V1AdminAssignedTechnician{},
		InventorySummary:    V1AdminMachineInventorySummary{},
	}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{
		`"machineName"`,
		`"siteName"`,
		`"assignedTechnicians"`,
		`"inventorySummary"`,
		`"totalSlots"`,
	} {
		if !strings.Contains(s, key) {
			t.Fatalf("missing key %s in %s", key, s)
		}
	}
}
