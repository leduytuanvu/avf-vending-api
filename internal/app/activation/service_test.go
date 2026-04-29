package activation

import "testing"

func TestMachineEligibleForRuntimeBlocksDisabledAndRetired(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"maintenance", "retired", " MAINTENANCE "} {
		if machineEligibleForRuntime(status) {
			t.Fatalf("status %q should not be eligible for runtime activation or refresh", status)
		}
	}

	for _, status := range []string{"provisioning", "online", "offline", ""} {
		if !machineEligibleForRuntime(status) {
			t.Fatalf("status %q should remain eligible for runtime activation or refresh", status)
		}
	}
}
