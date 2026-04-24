package telemetry

import "testing"

func TestClassifyEventType(t *testing.T) {
	cases := []struct {
		in   string
		want Class
	}{
		{"heartbeat", ClassHeartbeat},
		{"presence", ClassHeartbeat},
		{"state.heartbeat", ClassHeartbeat},
		{"health.ping", ClassHeartbeat},
		{"telemetry.incident.door", ClassIncident},
		{"telemetry.snapshot.board", ClassMetrics},
		{"events.vend", ClassMetrics},
		{"shadow.desired.patch", ClassState},
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

func TestCriticalityForEventType(t *testing.T) {
	cases := []struct {
		in   string
		want Criticality
	}{
		{"heartbeat", CriticalityDroppableMetrics},
		{"metrics.cpu", CriticalityDroppableMetrics},
		{"debug.trace", CriticalityDroppableMetrics},
		{"telemetry.snapshot.board", CriticalityCompactableLatest},
		{"shadow.reported", CriticalityCompactableLatest},
		{"events.vend", CriticalityCriticalNoDrop},
		{"vend.success", CriticalityCriticalNoDrop},
		{"vend.failure", CriticalityCriticalNoDrop},
		{"payment.authorized", CriticalityCriticalNoDrop},
		{"payments.capture", CriticalityCriticalNoDrop},
		{"webhook.payment.completed", CriticalityCriticalNoDrop},
		{"webhook.cashless.event", CriticalityCriticalNoDrop},
		{"webhook.generic", CriticalityDroppableMetrics},
		{"cash.inserted", CriticalityCriticalNoDrop},
		{"events.inventory", CriticalityCriticalNoDrop},
		{"config.ack", CriticalityCriticalNoDrop},
		{"telemetry.incident.jam", CriticalityCriticalNoDrop},
		{"telemetry.incident.door", CriticalityCriticalNoDrop},
		{"incident.door_open", CriticalityCriticalNoDrop},
		{"telemetry.incident.supply_low", CriticalityCompactableLatest},
		{"incident.info", CriticalityCompactableLatest},
	}
	for _, tc := range cases {
		if got := CriticalityForEventType(tc.in); got != tc.want {
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
