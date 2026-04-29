package workermetrics

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var errMetricsWarmup = errors.New("metrics_warmup")

func TestWorkerCycleMetricsGatherDoesNotPanic(t *testing.T) {
	t.Parallel()
	RecordWorkerCycleEnd("metrics_warmup_job", time.Millisecond, nil, "ok")
	RecordWorkerCycleEnd("metrics_warmup_job", time.Millisecond, errMetricsWarmup, "error")
	if _, err := prometheus.DefaultGatherer.Gather(); err != nil {
		t.Fatal(err)
	}
}
