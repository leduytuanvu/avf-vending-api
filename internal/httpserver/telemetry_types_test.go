package httpserver

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestV1MachineTelemetrySnapshotResponse_JSON_keys(t *testing.T) {
	t.Parallel()
	ex := V1MachineTelemetrySnapshotResponse{
		MachineID:         "3fa85f64-5717-4562-b3fc-2c963f66afa6",
		OrganizationID:    "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		SiteID:            "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		ReportedState:     []byte(`{}`),
		MetricsState:      []byte(`{}`),
		UpdatedAt:         "2026-01-02T03:04:05.123456789Z",
		EffectiveTimezone: "UTC",
	}
	b, err := json.Marshal(ex)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{
		`"machineId"`,
		`"reportedState"`,
		`"metricsState"`,
		`"updatedAt"`,
		`"effectiveTimezone"`,
	} {
		if !strings.Contains(s, key) {
			t.Fatalf("missing key %s in %s", key, s)
		}
	}
}

func TestV1MachineTelemetryIncidentsResponse_JSON_keys(t *testing.T) {
	t.Parallel()
	ex := V1MachineTelemetryIncidentsResponse{
		Items: []V1MachineTelemetryIncidentItem{
			{
				ID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				Severity:  "warning",
				Code:      "X",
				Detail:    []byte(`{}`),
				OpenedAt:  "2026-01-02T03:04:05.123456789Z",
				UpdatedAt: "2026-01-02T03:04:05.123456789Z",
			},
		},
		Meta: V1MachineTelemetryIncidentsMeta{Limit: 50, Returned: 1},
	}
	b, err := json.Marshal(ex)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{`"openedAt"`, `"updatedAt"`, `"items"`, `"meta"`, `"limit"`} {
		if !strings.Contains(s, key) {
			t.Fatalf("missing key %s in %s", key, s)
		}
	}
}

func TestV1MachineTelemetryRollupsResponse_JSON_keys(t *testing.T) {
	t.Parallel()
	ex := V1MachineTelemetryRollupsResponse{
		Items: []V1MachineTelemetryRollupItem{
			{
				BucketStart: "2026-01-02T03:04:05.123456789Z",
				Granularity: "1m",
				MetricKey:   "k",
				SampleCount: 1,
				Extra:       []byte(`{}`),
			},
		},
		Meta: V1MachineTelemetryRollupsMeta{
			Granularity: "1m",
			From:        "2026-01-01T00:00:00.000000000Z",
			To:          "2026-01-02T00:00:00.000000000Z",
			Returned:    1,
			Note:        "n",
		},
	}
	b, err := json.Marshal(ex)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{`"bucketStart"`, `"from"`, `"to"`, `"granularity"`} {
		if !strings.Contains(s, key) {
			t.Fatalf("missing key %s in %s", key, s)
		}
	}
}
