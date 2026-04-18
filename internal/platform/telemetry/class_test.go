package telemetry

import "testing"

func TestClassifyEventType(t *testing.T) {
	cases := []struct {
		in   string
		want Class
	}{
		{"heartbeat", ClassHeartbeat},
		{"health.ping", ClassHeartbeat},
		{"metrics.cpu", ClassMetrics},
		{"metric.temp", ClassMetrics},
		{"incident.door", ClassIncident},
		{"alert.foo", ClassIncident},
		{"diagnostic.bundle_ready", ClassDiagnosticBundleReady},
		{"state.v1", ClassState},
		{"shadow.reported", ClassState},
		{"unknown_noise", ClassMetrics},
	}
	for _, tc := range cases {
		if got := ClassifyEventType(tc.in); got != tc.want {
			t.Fatalf("%q: want %s got %s", tc.in, tc.want, got)
		}
	}
}

func TestMaxIngestPayloadBytesDefault(t *testing.T) {
	t.Setenv("TELEMETRY_MAX_INGEST_BYTES", "")
	if n := MaxIngestPayloadBytes(); n != DefaultMaxIngestBytes {
		t.Fatalf("default: %d", n)
	}
}
