// Package workermetrics registers Prometheus metrics for cmd/worker periodic jobs.
package workermetrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var workerCycleSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "avf",
	Subsystem: "worker",
	Name:      "job_cycle_seconds",
	Help:      "Wall time for one periodic worker cycle by job name and outcome.",
	Buckets:   prometheus.ExponentialBuckets(0.05, 2, 18),
}, []string{"job", "result"})

// RecordWorkerCycleEnd matches background.CycleEndMetricsHook for periodic worker loops.
func RecordWorkerCycleEnd(job string, duration time.Duration, err error, result string) {
	workerCycleSeconds.WithLabelValues(job, result).Observe(duration.Seconds())
}
