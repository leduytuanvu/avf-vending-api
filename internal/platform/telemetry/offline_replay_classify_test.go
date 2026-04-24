package telemetry

import "testing"

// Ensures event_type strings used in testdata/telemetry samples map to expected criticality.
func TestOfflineReplay_contractSamples_criticality(t *testing.T) {
	t.Parallel()
	cases := map[string]Criticality{
		"vend.success":     CriticalityCriticalNoDrop,
		"vend.failure":     CriticalityCriticalNoDrop,
		"payment.captured": CriticalityCriticalNoDrop,
		"cash.inserted":    CriticalityCriticalNoDrop,
		"inventory.delta":  CriticalityCriticalNoDrop,
		"state.heartbeat":  CriticalityDroppableMetrics,
		"events.vend":      CriticalityCriticalNoDrop,
		"events.cash":      CriticalityCriticalNoDrop,
		"events.inventory": CriticalityCriticalNoDrop,
	}
	for et, want := range cases {
		t.Run(et, func(t *testing.T) {
			t.Parallel()
			if got := CriticalityForEventType(et); got != want {
				t.Fatalf("CriticalityForEventType(%q)=%s want %s", et, got, want)
			}
		})
	}
}
