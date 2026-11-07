package postgres

import (
	"encoding/json"
	"testing"
)

func TestParseMetricsPayload_snapshotTopLevelNumbersWouldRollup(t *testing.T) {
	t.Parallel()
	samples := ParseMetricsPayload([]byte(`{"timestamp":1,"syncPendingCount":4,"customFields":{"cpu_pct":"9.5"}}`))
	if _, ok := samples["timestamp"]; !ok {
		t.Fatal("generic metrics would roll up HealthSnapshotPayload timestamp; snapshot ingest must not use that path")
	}
	if _, ok := samples["syncPendingCount"]; !ok {
		t.Fatal("generic metrics would roll up syncPendingCount")
	}
}

func TestParseDeviceHealthCustomFields_allowlistAndDropUnknown(t *testing.T) {
	t.Parallel()
	in := []byte(`{
		"machineId":"m1",
		"timestamp": 1,
		"syncPendingCount": 4,
		"customFields": {
			"cpu_pct": "12.5",
			"device_condition": "stressed",
			"net_online": "true",
			"heap_used_kb": "4096",
			"password": "secret",
			"temperatureCelsius": "4.5"
		}
	}`)
	got := ParseDeviceHealthCustomFields(in)
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatal(err)
	}
	if m["cpu_pct"] != 12.5 {
		t.Fatalf("cpu_pct=%v", m["cpu_pct"])
	}
	if m["device_condition"] != "stressed" {
		t.Fatalf("device_condition=%v", m["device_condition"])
	}
	if m["net_online"] != true {
		t.Fatalf("net_online=%v", m["net_online"])
	}
	if m["heap_used_kb"] != float64(4096) {
		t.Fatalf("heap_used_kb=%v", m["heap_used_kb"])
	}
	if _, ok := m["password"]; ok {
		t.Fatal("unknown key leaked")
	}
	if _, ok := m["temperatureCelsius"]; ok {
		t.Fatal("cabinet temp must not enter SoC allowlist")
	}
	if _, ok := m["syncPendingCount"]; ok {
		t.Fatal("top-level health counters must not enter metrics_state")
	}
}

func TestParseDeviceHealthCustomFields_emptyAndInvalid(t *testing.T) {
	t.Parallel()
	if string(ParseDeviceHealthCustomFields(nil)) != "{}" {
		t.Fatal("nil")
	}
	if string(ParseDeviceHealthCustomFields([]byte(`not-json`))) != "{}" {
		t.Fatal("invalid")
	}
	if string(ParseDeviceHealthCustomFields([]byte(`{"customFields":{}}`))) != "{}" {
		t.Fatal("empty customFields")
	}
}

func TestFilterDeviceHealthMetrics_allowlistOnly(t *testing.T) {
	t.Parallel()
	got := FilterDeviceHealthMetrics([]byte(`{
		"cpu_pct": 8.5,
		"vend_failure_rate": 0.2,
		"device_condition": "ok",
		"password": "x"
	}`))
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatal(err)
	}
	if m["cpu_pct"] != 8.5 || m["device_condition"] != "ok" {
		t.Fatalf("got %v", m)
	}
	if _, ok := m["vend_failure_rate"]; ok {
		t.Fatal("non-allowlist key leaked")
	}
	if _, ok := m["password"]; ok {
		t.Fatal("unknown key leaked")
	}
}

func TestParseDeviceHealthCustomFields_snakeCustomFields(t *testing.T) {
	t.Parallel()
	got := ParseDeviceHealthCustomFields([]byte(`{"custom_fields":{"net_quality":"good"}}`))
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatal(err)
	}
	if m["net_quality"] != "good" {
		t.Fatalf("got %v", m)
	}
}
