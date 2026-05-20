package reliability_test

import (
	"github.com/avf/avf-vending-api/internal/platform/id"
	"testing"
	"time"

	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/stretchr/testify/require"
)

func TestDecideOutboxReplay_DeadLetterNeverRepublishes(t *testing.T) {
	svc := appreliability.NewService(appreliability.Deps{})
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	dl := now.Add(-time.Minute)
	ev := domainreliability.OutboxEvent{
		ID:                  9,
		Topic:               "payments.x",
		CreatedAt:           now.Add(-time.Hour),
		AggregateID:         id.NewUUIDV7(),
		DeadLetteredAt:      &dl,
		PublishAttemptCount: 99,
	}
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	d := svc.DecideOutboxReplay(appreliability.ScanRunContext{Now: now}, policy, ev)
	require.False(t, d.ShouldRepublish)
	require.Equal(t, appreliability.ReasonOutboxDeadLettered, d.ReasonCode)
}

func TestDecideOutboxReplay_BackoffDefersEvenWhenOld(t *testing.T) {
	svc := appreliability.NewService(appreliability.Deps{})
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	next := now.Add(30 * time.Minute)
	ev := domainreliability.OutboxEvent{
		ID:               2,
		Topic:            "payments.x",
		CreatedAt:        now.Add(-24 * time.Hour),
		AggregateID:      id.NewUUIDV7(),
		NextPublishAfter: &next,
	}
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	d := svc.DecideOutboxReplay(appreliability.ScanRunContext{Now: now}, policy, ev)
	require.False(t, d.ShouldRepublish)
	require.Equal(t, appreliability.ReasonOutboxPublishBackoff, d.ReasonCode)
}

func TestDecideOutboxReplay_AgedEligibleRepublishes(t *testing.T) {
	svc := appreliability.NewService(appreliability.Deps{})
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	ev := domainreliability.OutboxEvent{
		ID:          3,
		Topic:       "payments.x",
		CreatedAt:   now.Add(-10 * time.Minute),
		AggregateID: id.NewUUIDV7(),
	}
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Minute})
	d := svc.DecideOutboxReplay(appreliability.ScanRunContext{Now: now}, policy, ev)
	require.True(t, d.ShouldRepublish)
	require.Equal(t, appreliability.ReasonOutboxAgedUnpublished, d.ReasonCode)
}
