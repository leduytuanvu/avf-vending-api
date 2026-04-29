package loadtest

import (
	"testing"
	"time"
)

func TestLatencyRecorder_Report(t *testing.T) {
	var r LatencyRecorder
	r.Add(10*time.Millisecond, false)
	r.Add(20*time.Millisecond, false)
	r.Add(30*time.Millisecond, true)
	rep := r.Report(100 * time.Millisecond)
	if rep.Requests != 3 || rep.SamplesOK != 2 || rep.Errors != 1 {
		t.Fatalf("unexpected agg: %+v", rep)
	}
	if rep.ErrorRate < 0.3 || rep.ErrorRate > 0.34 {
		t.Fatalf("error rate: %v", rep.ErrorRate)
	}
}

func TestPercentileSorted(t *testing.T) {
	s := []int64{10, 20, 30, 40, 50}
	if percentileSorted(s, 0.5) != 30 {
		t.Fatalf("p50")
	}
}
