package db

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

type slowQueryTraceKey struct{}

type slowQueryTraceVal struct {
	start time.Time
	sql   string
}

// SlowQueryTracer logs executed SQL slower than threshold (implements pgx.QueryTracer).
type SlowQueryTracer struct {
	threshold time.Duration
	log       *slog.Logger
}

// NewSlowQueryTracer returns nil when ms<=0 or log is nil.
func NewSlowQueryTracer(ms int, log *slog.Logger) pgx.QueryTracer {
	if ms <= 0 || log == nil {
		return nil
	}
	return &SlowQueryTracer{
		threshold: time.Duration(ms) * time.Millisecond,
		log:       log,
	}
}

// TraceQueryStart implements pgx.QueryTracer.
func (t *SlowQueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	sqlText := data.SQL
	if len(sqlText) > 2048 {
		sqlText = sqlText[:2048]
	}
	return context.WithValue(ctx, slowQueryTraceKey{}, slowQueryTraceVal{start: time.Now(), sql: sqlText})
}

// TraceQueryEnd implements pgx.QueryTracer.
func (t *SlowQueryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	v, ok := ctx.Value(slowQueryTraceKey{}).(slowQueryTraceVal)
	if !ok {
		return
	}
	d := time.Since(v.start)
	if d < t.threshold {
		return
	}
	t.log.Warn("postgres_slow_query",
		"duration_ms", d.Milliseconds(),
		"threshold_ms", int64(t.threshold/time.Millisecond),
		"sql", v.sql,
		"command_tag", data.CommandTag.String(),
		"err", data.Err,
	)
}
