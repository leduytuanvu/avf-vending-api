package reliability_test

import (
	"context"
	"testing"
	"time"

	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubLeaseOutbox struct {
	leaseReturn []domainreliability.OutboxEvent
	listReturn  []domainreliability.OutboxEvent
	stats       domainreliability.OutboxPipelineStats
	leaseWorker string
	leaseTTL    time.Duration
	leaseMinAge time.Duration
	leaseLimit  int32
}

func (s *stubLeaseOutbox) ListUnpublished(ctx context.Context, limit int32) ([]domainreliability.OutboxEvent, error) {
	_ = ctx
	if int(limit) < len(s.listReturn) {
		return s.listReturn[:limit], nil
	}
	return s.listReturn, nil
}

func (s *stubLeaseOutbox) LeaseOutboxForPublish(ctx context.Context, workerID string, lockTTL time.Duration, minAge time.Duration, limit int32) ([]domainreliability.OutboxEvent, error) {
	_ = ctx
	s.leaseWorker = workerID
	s.leaseTTL = lockTTL
	s.leaseMinAge = minAge
	s.leaseLimit = limit
	return s.leaseReturn, nil
}

func (s *stubLeaseOutbox) RecordOutboxPublishFailure(ctx context.Context, rec domainreliability.OutboxPublishFailureRecord) error {
	_ = ctx
	_ = rec
	return nil
}

func (s *stubLeaseOutbox) GetOutboxPipelineStats(ctx context.Context) (domainreliability.OutboxPipelineStats, error) {
	_ = ctx
	return s.stats, nil
}

func TestPlanOutboxRepublishBatch_LeaseBypassesDecideMinAge(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ev := domainreliability.OutboxEvent{
		ID:          77,
		Topic:       "payments.x",
		CreatedAt:   now,
		AggregateID: uuid.New(),
		Status:      "publishing",
	}
	stub := &stubLeaseOutbox{
		leaseReturn: []domainreliability.OutboxEvent{ev},
	}
	svc := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: 24 * time.Hour})
	plan, err := svc.PlanOutboxRepublishBatch(context.Background(), appreliability.ScanRunContext{Now: now}, policy, appreliability.ScanLimits{MaxItems: 10}, &appreliability.OutboxLeaseParams{
		WorkerID: "lease-worker",
		LockTTL:  30 * time.Second,
	})
	require.NoError(t, err)
	require.Len(t, plan.Decisions, 1)
	require.True(t, plan.Decisions[0].ShouldRepublish)
	require.Equal(t, appreliability.ReasonOutboxLeaseClaim, plan.Decisions[0].ReasonCode)
	require.Equal(t, "lease-worker", stub.leaseWorker)
	require.Equal(t, int32(10), stub.leaseLimit)
}

func TestDecideOutboxReplay_ActiveLeaseHeld(t *testing.T) {
	svc := appreliability.NewService(appreliability.Deps{})
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	until := now.Add(30 * time.Second)
	ev := domainreliability.OutboxEvent{
		ID:          4,
		Topic:       "payments.x",
		CreatedAt:   now.Add(-10 * time.Minute),
		AggregateID: uuid.New(),
		Status:      "publishing",
		LockedUntil: &until,
	}
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	d := svc.DecideOutboxReplay(appreliability.ScanRunContext{Now: now}, policy, ev)
	require.False(t, d.ShouldRepublish)
	require.Equal(t, appreliability.ReasonNoopPolicy, d.ReasonCode)
}
