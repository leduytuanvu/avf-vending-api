package background_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	appbackground "github.com/avf/avf-vending-api/internal/app/background"
	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type stubOutboxRepo struct {
	list     []domainreliability.OutboxEvent
	recorded []domainreliability.OutboxPublishFailureRecord
	stats    domainreliability.OutboxPipelineStats
}

func (s *stubOutboxRepo) ListUnpublished(ctx context.Context, limit int32) ([]domainreliability.OutboxEvent, error) {
	_ = ctx
	if int(limit) < len(s.list) {
		return s.list[:limit], nil
	}
	return s.list, nil
}

func (s *stubOutboxRepo) RecordOutboxPublishFailure(ctx context.Context, rec domainreliability.OutboxPublishFailureRecord) error {
	_ = ctx
	s.recorded = append(s.recorded, rec)
	return nil
}

func (s *stubOutboxRepo) GetOutboxPipelineStats(ctx context.Context) (domainreliability.OutboxPipelineStats, error) {
	_ = ctx
	return s.stats, nil
}

func (s *stubOutboxRepo) LeaseOutboxForPublish(ctx context.Context, workerID string, lockTTL time.Duration, minAge time.Duration, limit int32) ([]domainreliability.OutboxEvent, error) {
	_ = ctx
	_ = workerID
	_ = lockTTL
	_ = minAge
	return s.ListUnpublished(ctx, limit)
}

type errPublisher struct{}

func (errPublisher) Publish(context.Context, domaincommerce.OutboxEvent) error {
	return errors.New("nats down")
}

type okPublisher struct{}

func (okPublisher) Publish(context.Context, domaincommerce.OutboxEvent) error {
	return nil
}

type countingPublisher struct {
	n atomic.Int32
}

func (c *countingPublisher) Publish(context.Context, domaincommerce.OutboxEvent) error {
	c.n.Add(1)
	return nil
}

type markTracker struct {
	lastID  int64
	marked  bool
	markErr error
}

func (m *markTracker) MarkOutboxPublished(ctx context.Context, outboxID int64) (bool, error) {
	_ = ctx
	m.lastID = outboxID
	return m.marked, m.markErr
}

type markOKOnSecondCall struct {
	calls atomic.Int32
}

func (m *markOKOnSecondCall) MarkOutboxPublished(ctx context.Context, outboxID int64) (bool, error) {
	_ = ctx
	_ = outboxID
	if m.calls.Add(1) == 1 {
		return false, errors.New("first mark failed")
	}
	return true, nil
}

type leaseSpyRepo struct {
	inner      *stubOutboxRepo
	leaseCalls atomic.Int32
}

func (l *leaseSpyRepo) ListUnpublished(ctx context.Context, limit int32) ([]domainreliability.OutboxEvent, error) {
	return l.inner.ListUnpublished(ctx, limit)
}

func (l *leaseSpyRepo) RecordOutboxPublishFailure(ctx context.Context, rec domainreliability.OutboxPublishFailureRecord) error {
	return l.inner.RecordOutboxPublishFailure(ctx, rec)
}

func (l *leaseSpyRepo) GetOutboxPipelineStats(ctx context.Context) (domainreliability.OutboxPipelineStats, error) {
	return l.inner.GetOutboxPipelineStats(ctx)
}

func (l *leaseSpyRepo) LeaseOutboxForPublish(ctx context.Context, workerID string, lockTTL time.Duration, minAge time.Duration, limit int32) ([]domainreliability.OutboxEvent, error) {
	l.leaseCalls.Add(1)
	return l.inner.LeaseOutboxForPublish(ctx, workerID, lockTTL, minAge, limit)
}

type dlqSpy struct {
	calls int
	last  domaincommerce.OutboxEvent
	err   error
}

func (d *dlqSpy) PublishOutboxDeadLettered(ctx context.Context, ev domaincommerce.OutboxEvent, lastPublishError string) error {
	_ = ctx
	_ = lastPublishError
	d.calls++
	d.last = ev
	return d.err
}

func TestOutboxDispatchTick_PublishFailureRecordsAttempt(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:                  42,
		Topic:               "payments.test",
		CreatedAt:           time.Now().UTC().Add(-time.Hour),
		AggregateID:         uuid.New(),
		PublishAttemptCount: 0,
	}
	stub := &stubOutboxRepo{
		list:  []domainreliability.OutboxEvent{ev},
		stats: domainreliability.OutboxPipelineStats{PendingDueNow: 1},
	}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{
		OutboxMinAge:             time.Millisecond,
		OutboxMaxPublishAttempts: 10,
		OutboxPublishBackoffBase: time.Minute,
		OutboxPublishBackoffMax:  time.Minute,
	})
	deps := appbackground.WorkerDeps{
		Log:         zap.NewNop(),
		Reliability: rel,
		Policy:      policy,
		Limits:      appreliability.ScanLimits{MaxItems: 50},
		OutboxList:  stub,
		OutboxMark:  &markTracker{marked: true},
		OutboxPub:   errPublisher{},
	}
	stub.list[0] = ev

	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Len(t, stub.recorded, 1)
	require.False(t, stub.recorded[0].DeadLettered)
	require.NotNil(t, stub.recorded[0].NextPublishAfter)
}

func TestOutboxDispatchTick_LastAttemptDeadLetters(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:                  7,
		Topic:               "payments.test",
		CreatedAt:           time.Now().UTC().Add(-2 * time.Hour),
		AggregateID:         uuid.New(),
		PublishAttemptCount: 2,
	}
	stub := &stubOutboxRepo{
		list:  []domainreliability.OutboxEvent{ev},
		stats: domainreliability.OutboxPipelineStats{},
	}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{
		OutboxMinAge:             time.Millisecond,
		OutboxMaxPublishAttempts: 3,
		OutboxPublishBackoffBase: time.Millisecond,
		OutboxPublishBackoffMax:  time.Millisecond,
	})
	mark := &markTracker{marked: true}
	deps := appbackground.WorkerDeps{
		Log:         zap.NewNop(),
		Reliability: rel,
		Policy:      policy,
		Limits:      appreliability.ScanLimits{MaxItems: 50},
		OutboxList:  stub,
		OutboxMark:  mark,
		OutboxPub:   errPublisher{},
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Len(t, stub.recorded, 1)
	require.True(t, stub.recorded[0].DeadLettered)
	require.Nil(t, stub.recorded[0].NextPublishAfter)
}

func TestOutboxDispatchTick_LastAttemptInvokesDeadLetterHook(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:                  77,
		Topic:               "payments.test",
		CreatedAt:           time.Now().UTC().Add(-2 * time.Hour),
		AggregateID:         uuid.New(),
		PublishAttemptCount: 2,
	}
	stub := &stubOutboxRepo{
		list:  []domainreliability.OutboxEvent{ev},
		stats: domainreliability.OutboxPipelineStats{},
	}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{
		OutboxMinAge:             time.Millisecond,
		OutboxMaxPublishAttempts: 3,
		OutboxPublishBackoffBase: time.Millisecond,
		OutboxPublishBackoffMax:  time.Millisecond,
	})
	dlq := &dlqSpy{}
	deps := appbackground.WorkerDeps{
		Log:              zap.NewNop(),
		Reliability:      rel,
		Policy:           policy,
		Limits:           appreliability.ScanLimits{MaxItems: 50},
		OutboxList:       stub,
		OutboxMark:       &markTracker{marked: true},
		OutboxPub:        errPublisher{},
		OutboxDeadLetter: dlq,
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Len(t, stub.recorded, 1)
	require.True(t, stub.recorded[0].DeadLettered)
	require.Equal(t, 1, dlq.calls)
	require.Equal(t, ev.ID, dlq.last.ID)
}

func TestOutboxDispatchTick_DeadLetterHookErrorStillReturnsNil(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:                  88,
		Topic:               "payments.test",
		CreatedAt:           time.Now().UTC().Add(-2 * time.Hour),
		AggregateID:         uuid.New(),
		PublishAttemptCount: 2,
	}
	stub := &stubOutboxRepo{list: []domainreliability.OutboxEvent{ev}, stats: domainreliability.OutboxPipelineStats{}}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{
		OutboxMinAge:             time.Millisecond,
		OutboxMaxPublishAttempts: 3,
		OutboxPublishBackoffBase: time.Millisecond,
		OutboxPublishBackoffMax:  time.Millisecond,
	})
	dlq := &dlqSpy{err: errors.New("dlq broker down")}
	deps := appbackground.WorkerDeps{
		Log:              zap.NewNop(),
		Reliability:      rel,
		Policy:           policy,
		Limits:           appreliability.ScanLimits{MaxItems: 50},
		OutboxList:       stub,
		OutboxMark:       &markTracker{marked: true},
		OutboxPub:        errPublisher{},
		OutboxDeadLetter: dlq,
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Len(t, stub.recorded, 1)
	require.True(t, stub.recorded[0].DeadLettered)
	require.Equal(t, 1, dlq.calls)
}

func TestOutboxDispatchTick_MarkNoRowStillSuccess(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:          99,
		Topic:       "payments.test",
		CreatedAt:   time.Now().UTC().Add(-2 * time.Hour),
		AggregateID: uuid.New(),
	}
	stub := &stubOutboxRepo{list: []domainreliability.OutboxEvent{ev}}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	mark := &markTracker{marked: false, markErr: nil}
	deps := appbackground.WorkerDeps{
		Log:         zap.NewNop(),
		Reliability: rel,
		Policy:      policy,
		Limits:      appreliability.ScanLimits{MaxItems: 50},
		OutboxList:  stub,
		OutboxMark:  mark,
		OutboxPub:   okPublisher{},
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Equal(t, int64(99), mark.lastID)
	require.Empty(t, stub.recorded)
}

type mirrorSpy struct {
	calls int
	last  domaincommerce.OutboxEvent
}

func (m *mirrorSpy) hook(ev domaincommerce.OutboxEvent) {
	m.calls++
	m.last = ev
}

func TestOutboxDispatchTick_OnOutboxPublishedMirrorAfterSuccessfulMark(t *testing.T) {
	agg := uuid.New()
	ev := domainreliability.OutboxEvent{
		ID:                  100,
		Topic:               "payments.test",
		CreatedAt:           time.Now().UTC().Add(-time.Hour),
		AggregateID:         agg,
		PublishAttemptCount: 0,
	}
	stub := &stubOutboxRepo{
		list:  []domainreliability.OutboxEvent{ev},
		stats: domainreliability.OutboxPipelineStats{PendingDueNow: 1},
	}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	spy := &mirrorSpy{}
	deps := appbackground.WorkerDeps{
		Log:                     zap.NewNop(),
		Reliability:             rel,
		Policy:                  policy,
		Limits:                  appreliability.ScanLimits{MaxItems: 50},
		OutboxList:              stub,
		OutboxMark:              &markTracker{marked: true},
		OutboxPub:               okPublisher{},
		OnOutboxPublishedMirror: spy.hook,
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Equal(t, 1, spy.calls)
	require.Equal(t, int64(100), spy.last.ID)
	require.Equal(t, "payments.test", spy.last.Topic)
	require.NotNil(t, spy.last.PublishedAt)
}

func TestOutboxDispatchTick_OnOutboxPublishedMirrorSkippedWhenMarkNoop(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:          101,
		Topic:       "payments.test",
		CreatedAt:   time.Now().UTC().Add(-time.Hour),
		AggregateID: uuid.New(),
	}
	stub := &stubOutboxRepo{list: []domainreliability.OutboxEvent{ev}, stats: domainreliability.OutboxPipelineStats{}}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	spy := &mirrorSpy{}
	deps := appbackground.WorkerDeps{
		Log:                     zap.NewNop(),
		Reliability:             rel,
		Policy:                  policy,
		Limits:                  appreliability.ScanLimits{MaxItems: 50},
		OutboxList:              stub,
		OutboxMark:              &markTracker{marked: false},
		OutboxPub:               okPublisher{},
		OnOutboxPublishedMirror: spy.hook,
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Equal(t, 0, spy.calls)
}

func TestOutboxDispatchTick_OnOutboxPublishedMirrorSkippedOnPublishFailure(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:                  102,
		Topic:               "payments.test",
		CreatedAt:           time.Now().UTC().Add(-2 * time.Hour),
		AggregateID:         uuid.New(),
		PublishAttemptCount: 0,
	}
	stub := &stubOutboxRepo{
		list:  []domainreliability.OutboxEvent{ev},
		stats: domainreliability.OutboxPipelineStats{},
	}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{
		OutboxMinAge:             time.Millisecond,
		OutboxMaxPublishAttempts: 5,
		OutboxPublishBackoffBase: time.Minute,
		OutboxPublishBackoffMax:  time.Minute,
	})
	spy := &mirrorSpy{}
	deps := appbackground.WorkerDeps{
		Log:                     zap.NewNop(),
		Reliability:             rel,
		Policy:                  policy,
		Limits:                  appreliability.ScanLimits{MaxItems: 50},
		OutboxList:              stub,
		OutboxMark:              &markTracker{marked: true},
		OutboxPub:               errPublisher{},
		OnOutboxPublishedMirror: spy.hook,
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Equal(t, 0, spy.calls)
}

func TestOutboxDispatchTick_AnalyticsHookPanicDoesNotFailDispatch(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:          103,
		Topic:       "payments.test",
		CreatedAt:   time.Now().UTC().Add(-time.Hour),
		AggregateID: uuid.New(),
	}
	stub := &stubOutboxRepo{list: []domainreliability.OutboxEvent{ev}, stats: domainreliability.OutboxPipelineStats{}}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	mark := &markTracker{marked: true}
	deps := appbackground.WorkerDeps{
		Log:         zap.NewNop(),
		Reliability: rel,
		Policy:      policy,
		Limits:      appreliability.ScanLimits{MaxItems: 50},
		OutboxList:  stub,
		OutboxMark:  mark,
		OutboxPub:   okPublisher{},
		OnOutboxPublishedMirror: func(domaincommerce.OutboxEvent) {
			panic("analytics sink bug")
		},
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Equal(t, int64(103), mark.lastID)
}

func TestOutboxDispatchTick_MarkErrorAfterPublishRetriesJetStreamPublish(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:                  201,
		Topic:               "payments.test",
		CreatedAt:           time.Now().UTC().Add(-time.Hour),
		AggregateID:         uuid.New(),
		PublishAttemptCount: 0,
	}
	base := &stubOutboxRepo{
		list:  []domainreliability.OutboxEvent{ev},
		stats: domainreliability.OutboxPipelineStats{PendingDueNow: 1},
	}
	rel := appreliability.NewService(appreliability.Deps{Outbox: base})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	pub := &countingPublisher{}
	mark := &markOKOnSecondCall{}
	deps := appbackground.WorkerDeps{
		Log:         zap.NewNop(),
		Reliability: rel,
		Policy:      policy,
		Limits:      appreliability.ScanLimits{MaxItems: 50},
		OutboxList:  base,
		OutboxMark:  mark,
		OutboxPub:   pub,
	}
	require.Error(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.EqualValues(t, 1, pub.n.Load())

	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.EqualValues(t, 2, pub.n.Load())
}

func TestOutboxDispatchTick_LeasePathInvokesLeaseOutboxForPublish(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:          501,
		Topic:       "payments.test",
		CreatedAt:   time.Now().UTC().Add(-time.Hour),
		AggregateID: uuid.New(),
		Status:      "publishing",
	}
	base := &stubOutboxRepo{
		list:  []domainreliability.OutboxEvent{ev},
		stats: domainreliability.OutboxPipelineStats{PendingDueNow: 1},
	}
	spy := &leaseSpyRepo{inner: base}
	rel := appreliability.NewService(appreliability.Deps{Outbox: spy})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	deps := appbackground.WorkerDeps{
		Log:         zap.NewNop(),
		Reliability: rel,
		Policy:      policy,
		Limits:      appreliability.ScanLimits{MaxItems: 50},
		OutboxList:  spy,
		OutboxMark:  &markTracker{marked: true},
		OutboxPub:   okPublisher{},
		OutboxLease: &appreliability.OutboxLeaseParams{WorkerID: "worker-lease-1", LockTTL: 30 * time.Second},
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.EqualValues(t, 1, spy.leaseCalls.Load())
}

func TestOutboxDispatchTick_TwoTicksMarkNoopDuplicatesPublishExpectJetStreamDedupe(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:          202,
		Topic:       "payments.test",
		CreatedAt:   time.Now().UTC().Add(-time.Hour),
		AggregateID: uuid.New(),
	}
	stub := &stubOutboxRepo{
		list:  []domainreliability.OutboxEvent{ev},
		stats: domainreliability.OutboxPipelineStats{PendingDueNow: 1},
	}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	pub := &countingPublisher{}
	mark := &markTracker{marked: false}
	deps := appbackground.WorkerDeps{
		Log:         zap.NewNop(),
		Reliability: rel,
		Policy:      policy,
		Limits:      appreliability.ScanLimits{MaxItems: 50},
		OutboxList:  stub,
		OutboxMark:  mark,
		OutboxPub:   pub,
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.EqualValues(t, 2, pub.n.Load())
}

func TestOutboxDispatchTick_PublishSuccess(t *testing.T) {
	ev := domainreliability.OutboxEvent{
		ID:                  203,
		Topic:               "payments.test",
		CreatedAt:           time.Now().UTC().Add(-time.Hour),
		AggregateID:         uuid.New(),
		PublishAttemptCount: 0,
	}
	stub := &stubOutboxRepo{
		list:  []domainreliability.OutboxEvent{ev},
		stats: domainreliability.OutboxPipelineStats{PendingDueNow: 1},
	}
	rel := appreliability.NewService(appreliability.Deps{Outbox: stub})
	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{OutboxMinAge: time.Millisecond})
	mark := &markTracker{marked: true}
	deps := appbackground.WorkerDeps{
		Log:         zap.NewNop(),
		Reliability: rel,
		Policy:      policy,
		Limits:      appreliability.ScanLimits{MaxItems: 50},
		OutboxList:  stub,
		OutboxMark:  mark,
		OutboxPub:   okPublisher{},
	}
	require.NoError(t, appbackground.OutboxDispatchTick(context.Background(), deps))
	require.Equal(t, ev.ID, mark.lastID)
	require.Empty(t, stub.recorded)
}
