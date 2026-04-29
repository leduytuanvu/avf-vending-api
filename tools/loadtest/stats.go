package loadtest

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// LatencyRecorder collects durations for percentile / rate computation (thread-safe).
type LatencyRecorder struct {
	mu    sync.Mutex
	ns    []int64
	errs  int64
	total int64
}

func (r *LatencyRecorder) Add(d time.Duration, err bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.total++
	if err {
		r.errs++
		return
	}
	r.ns = append(r.ns, d.Nanoseconds())
}

func (r *LatencyRecorder) Merge(o *LatencyRecorder) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs += o.errs
	r.total += o.total
	r.ns = append(r.ns, o.ns...)
}

// Report summarizes collected samples.
func (r *LatencyRecorder) Report(duration time.Duration) Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	rep := Report{
		Requests:  r.total,
		Errors:    r.errs,
		Duration:  duration,
		SamplesOK: len(r.ns),
	}
	if r.total > 0 {
		rep.ErrorRate = float64(r.errs) / float64(r.total)
	}
	if duration > 0 && r.total > 0 {
		rep.ReqPerSec = float64(r.total) / duration.Seconds()
	}
	if len(r.ns) == 0 {
		return rep
	}
	ns := append([]int64(nil), r.ns...)
	sort.Slice(ns, func(i, j int) bool { return ns[i] < ns[j] })
	rep.P50 = time.Duration(percentileSorted(ns, 0.50))
	rep.P95 = time.Duration(percentileSorted(ns, 0.95))
	rep.P99 = time.Duration(percentileSorted(ns, 0.99))
	return rep
}

// Report is a printable load-test aggregate.
type Report struct {
	Requests  int64
	Errors    int64
	SamplesOK int
	Duration  time.Duration
	ReqPerSec float64
	ErrorRate float64
	P50       time.Duration
	P95       time.Duration
	P99       time.Duration
}

func (rep Report) String() string {
	return fmt.Sprintf("requests=%d errors=%d err_rate=%.4f rps=%.2f ok_samples=%d p50=%s p95=%s p99=%s window=%s",
		rep.Requests, rep.Errors, rep.ErrorRate, rep.ReqPerSec, rep.SamplesOK,
		rep.P50.Round(time.Millisecond), rep.P95.Round(time.Millisecond), rep.P99.Round(time.Millisecond),
		rep.Duration.Round(time.Millisecond))
}

func percentileSorted(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
