package clickhouse

import (
	"context"
	"fmt"
	"sync"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"go.uber.org/zap"
)

// AsyncProjectionSink maps successfully published outbox events into typed ClickHouse analytics rows.
// It is intentionally best-effort and never authoritative for sale, vend, payment, inventory, or command state.
type AsyncProjectionSink struct {
	log *zap.Logger
	ch  Client

	table       string
	maxInflight int
	timeout     time.Duration
	maxAttempts int

	sem chan struct{}
	wg  sync.WaitGroup
}

func NewAsyncProjectionSink(log *zap.Logger, hc Client, table string, maxInflight int, insertTimeout time.Duration, maxAttempts int) (*AsyncProjectionSink, error) {
	if hc == nil {
		return nil, fmt.Errorf("clickhouse: nil client")
	}
	if maxInflight < 1 {
		maxInflight = 1
	}
	if insertTimeout <= 0 {
		insertTimeout = 5 * time.Second
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &AsyncProjectionSink{
		log:         log,
		ch:          hc,
		table:       table,
		maxInflight: maxInflight,
		timeout:     insertTimeout,
		maxAttempts: maxAttempts,
		sem:         make(chan struct{}, maxInflight),
	}, nil
}

// EnqueuePublished schedules analytics projection work after an outbox event is published and marked.
func (s *AsyncProjectionSink) EnqueuePublished(ev domaincommerce.OutboxEvent) {
	if s == nil {
		return
	}
	select {
	case s.sem <- struct{}{}:
		s.wg.Add(1)
		go s.runInsert(ev)
	default:
		projectionEnqueueDropped.Inc()
		if s.log != nil {
			s.log.Warn("analytics_projection_enqueue_dropped",
				zap.Int64("outbox_id", ev.ID),
				zap.String("topic", ev.Topic),
				zap.String("event_type", ev.EventType),
				zap.String("note", "max concurrent analytics projection inserts; source outbox state is authoritative"),
			)
		}
	}
}

func (s *AsyncProjectionSink) runInsert(ev domaincommerce.OutboxEvent) {
	defer s.wg.Done()
	defer func() { <-s.sem }()

	lines, err := projectionRowsFromOutboxEvent(ev)
	if err != nil {
		projectionMarshalFailed.Inc()
		if s.log != nil {
			s.log.Warn("analytics_projection_marshal_failed", zap.Error(err), zap.Int64("outbox_id", ev.ID), zap.String("event_type", ev.EventType))
		}
		return
	}
	if len(lines) == 0 {
		return
	}
	for _, line := range lines {
		s.insertLine(ev, line)
	}
}

func (s *AsyncProjectionSink) insertLine(ev domaincommerce.OutboxEvent, line []byte) {
	var lastErr error
	for attempt := 1; attempt <= s.maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		err := s.ch.InsertJSONEachRow(ctx, s.table, line)
		cancel()
		if err == nil {
			projectionInsertOK.Inc()
			if !ev.CreatedAt.IsZero() {
				projectionLagSeconds.Observe(time.Since(ev.CreatedAt).Seconds())
			}
			return
		}
		lastErr = err
		if attempt < s.maxAttempts {
			time.Sleep(time.Duration(50*attempt) * time.Millisecond)
		}
	}
	projectionInsertFailed.Inc()
	if s.log != nil {
		s.log.Warn("analytics_projection_insert_exhausted",
			zap.Error(lastErr),
			zap.Int64("outbox_id", ev.ID),
			zap.String("topic", ev.Topic),
			zap.String("event_type", ev.EventType),
			zap.Int("max_attempts", s.maxAttempts),
			zap.String("note", "cold-path only; PostgreSQL and outbox remain authoritative"),
		)
	}
}

func (s *AsyncProjectionSink) Shutdown() {
	if s == nil {
		return
	}
	s.wg.Wait()
}
