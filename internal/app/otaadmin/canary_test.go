package otaadmin

import "testing"

func TestCanaryFirstWaveSize(t *testing.T) {
	t.Parallel()
	if got := canaryFirstWaveSize(10, 10, strategyCanary); got != 1 {
		t.Fatalf("10%% of 10 -> %d want 1", got)
	}
	if got := canaryFirstWaveSize(10, 100, strategyCanary); got != 10 {
		t.Fatalf("100%% -> %d want 10", got)
	}
	if got := canaryFirstWaveSize(10, 0, strategyCanary); got != 10 {
		t.Fatalf("0%% sentinel -> full fleet %d want 10", got)
	}
	if got := canaryFirstWaveSize(10, 20, strategyImmediate); got != 10 {
		t.Fatalf("immediate -> %d want 10", got)
	}
}
