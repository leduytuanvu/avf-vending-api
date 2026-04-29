package outboxmetrics

import (
	"testing"
	"time"

	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/prometheus/client_golang/prometheus"
)

func TestOutboxMetricsObserveAndGather(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	pl := domainreliability.OutboxPipelineStats{
		PendingTotal:           1,
		PendingDueNow:          1,
		DeadLetteredTotal:      0,
		PublishingLeasedTotal:  0,
		MaxPendingAttempts:     0,
		FailedPendingTotal:     0,
		OldestPendingCreatedAt: ptrTime(now.Add(-time.Minute)),
	}
	ObservePipelineGauges(now, pl)
	IncDispatchPublishFailed()
	IncDispatchPublished()
	IncDispatchDeadLettered()
	ObservePublishSuccessLag(2.5)
	if _, err := prometheus.DefaultGatherer.Gather(); err != nil {
		t.Fatal(err)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
