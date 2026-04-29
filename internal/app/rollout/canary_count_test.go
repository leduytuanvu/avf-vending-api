package rollout

import "testing"

func TestRolloutCampaignDispatchShouldStop(t *testing.T) {
	t.Parallel()
	if !rolloutCampaignDispatchShouldStop("paused") || !rolloutCampaignDispatchShouldStop("cancelled") {
		t.Fatal("expected pause/cancel to stop dispatch loop")
	}
	if rolloutCampaignDispatchShouldStop("running") || rolloutCampaignDispatchShouldStop("pending") {
		t.Fatal("running states must continue dispatch")
	}
}

func TestComputeCanaryCount(t *testing.T) {
	t.Parallel()
	if got := computeCanaryCount(47, 10); got != 5 {
		t.Fatalf("47 machines @ 10%%: want 5 got %d", got)
	}
	if got := computeCanaryCount(100, 10); got != 10 {
		t.Fatalf("100 machines @ 10%%: want 10 got %d", got)
	}
	if got := computeCanaryCount(5, 10); got != 1 {
		t.Fatalf("small fleet must include at least one machine (want 1 got %d)", got)
	}
	if got := computeCanaryCount(1000, 1); got != 10 {
		t.Fatalf("1000 machines @ 1%%: want ceil(10)=%d got %d", 10, got)
	}
}
