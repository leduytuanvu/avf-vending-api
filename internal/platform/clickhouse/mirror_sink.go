package clickhouse

import (
	"context"
	"fmt"
	"sync"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"go.uber.org/zap"
)

// AsyncOutboxMirrorSink mirrors successfully published outbox rows to ClickHouse without blocking OLTP.
// EnqueuePublished is non-blocking: it may drop under backpressure (counter + warn log).
type AsyncOutboxMirrorSink struct {
	log *zap.Logger
	ch  Client

	table       string
	maxInflight int
	timeout     time.Duration
	maxAttempts int

	sem chan struct{}
	wg  sync.WaitGroup
}

// NewAsyncOutboxMirrorSink constructs a sink. maxInflight bounds concurrent HTTP inserts (each with retries).
func NewAsyncOutboxMirrorSink(log *zap.Logger, hc Client, table string, maxInflight int, insertTimeout time.Duration, maxAttempts int) (*AsyncOutboxMirrorSink, error) {
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
	return &AsyncOutboxMirrorSink{
		log:         log,
		ch:          hc,
		table:       table,
		maxInflight: maxInflight,
		timeout:     insertTimeout,
		maxAttempts: maxAttempts,
		sem:         make(chan struct{}, maxInflight),
	}, nil
}

// EnqueuePublished schedules an async insert; never blocks the outbox dispatch loop beyond a semaphore acquire.
func (s *AsyncOutboxMirrorSink) EnqueuePublished(ev domaincommerce.OutboxEvent) {
	if s == nil {
		return
	}
	select {
	case s.sem <- struct{}{}:
		s.wg.Add(1)
		go s.runInsert(ev)
	default:
		mirrorEnqueueDropped.Inc()
		if s.log != nil {
			s.log.Warn("analytics_mirror_enqueue_dropped",
				zap.Int64("outbox_id", ev.ID),
				zap.String("topic", ev.Topic),
				zap.String("note", "max concurrent analytics inserts; increase ANALYTICS_MIRROR_MAX_CONCURRENT or capacity"),
			)
		}
	}
}

func (s *AsyncOutboxMirrorSink) runInsert(ev domaincommerce.OutboxEvent) {
	defer s.wg.Done()
	defer func() { <-s.sem }()

	line, err := mirrorRowFromOutboxEvent(ev)
	if err != nil {
		mirrorMarshalFailed.Inc()
		if s.log != nil {
			s.log.Warn("analytics_mirror_marshal_failed", zap.Error(err), zap.Int64("outbox_id", ev.ID))
		}
		return
	}

	var lastErr error
	for attempt := 1; attempt <= s.maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		err := s.ch.InsertJSONEachRow(ctx, s.table, line)
		cancel()
		if err == nil {
			mirrorInsertOK.Inc()
			return
		}
		lastErr = err
		if attempt < s.maxAttempts {
			backoff := time.Duration(50*attempt) * time.Millisecond
			time.Sleep(backoff)
		}
	}
	mirrorInsertFailed.Inc()
	if s.log != nil {
		s.log.Warn("analytics_mirror_insert_exhausted",
			zap.Error(lastErr),
			zap.Int64("outbox_id", ev.ID),
			zap.String("topic", ev.Topic),
			zap.Int("max_attempts", s.maxAttempts),
			zap.String("note", "cold-path only; Postgres outbox state is authoritative"),
		)
	}
}

// Shutdown waits for in-flight mirror inserts (bounded by caller ctx patience via WaitGroup only).
func (s *AsyncOutboxMirrorSink) Shutdown() {
	if s == nil {
		return
	}
	s.wg.Wait()
}
