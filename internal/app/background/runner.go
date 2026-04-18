package background

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EffectivePeriodicCycleTimeout returns the wall-clock budget for one pass of a periodic job
// (list + per-row work) when CycleTimeout on deps is zero.
//
// Defaults are conservative: long enough for batched I/O under load, bounded so shutdown and
// stuck work surface within minutes rather than hanging until the next manual kill.
func EffectivePeriodicCycleTimeout(tickInterval, configured time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}
	if tickInterval <= 0 {
		return 2 * time.Minute
	}
	// ~1.5× tick as a floor hint, clamped for predictable ops behavior across mixed tick rates.
	t := tickInterval + tickInterval/2
	if t < 45*time.Second {
		t = 45 * time.Second
	}
	if t > 12*time.Minute {
		t = 12 * time.Minute
	}
	return t
}

// CycleEndMetricsHook runs after each periodic cycle (optional). result matches background_cycle_end:
// ok, canceled, cycle_deadline_exceeded, error.
type CycleEndMetricsHook func(job string, duration time.Duration, err error, result string)

// runTickerLoop runs fn on a fixed interval until ctx is cancelled. Each invocation uses a
// derived context capped by cycleTimeout so one slow pass cannot block observing ctx cancellation
// indefinitely (callers should still pass ctx into I/O so cancellation propagates).
func runTickerLoop(ctx context.Context, log *zap.Logger, name string, every, cycleTimeout time.Duration, cycleHook CycleEndMetricsHook, fn func(context.Context) error) {
	if log == nil {
		log = zap.NewNop()
	}
	ct := EffectivePeriodicCycleTimeout(every, cycleTimeout)
	t := time.NewTicker(every)
	defer t.Stop()

	runOnce := func() {
		cycleCtx, cancel := context.WithTimeout(ctx, ct)
		defer cancel()
		start := time.Now()
		log.Info("background_cycle_start",
			zap.String("job", name),
			zap.Duration("tick_interval", every),
			zap.Duration("cycle_timeout", ct),
		)
		err := fn(cycleCtx)
		dur := time.Since(start)
		var result string
		switch {
		case err == nil:
			result = "ok"
			log.Info("background_cycle_end",
				zap.String("job", name),
				zap.Duration("duration", dur),
			)
		case errors.Is(err, context.Canceled):
			result = "canceled"
			log.Info("background_cycle_end",
				zap.String("job", name),
				zap.Duration("duration", dur),
				zap.String("result", "canceled"),
			)
		case errors.Is(err, context.DeadlineExceeded):
			result = "cycle_deadline_exceeded"
			log.Warn("background_cycle_end",
				zap.String("job", name),
				zap.Duration("duration", dur),
				zap.String("result", "cycle_deadline_exceeded"),
				zap.Error(err),
			)
		default:
			result = "error"
			// P0 ops signal: alert on repeated failures; the next tick will run again unless the process exits.
			log.Error("background_cycle_end",
				zap.String("job", name),
				zap.Duration("duration", dur),
				zap.String("result", "error"),
				zap.Error(err),
			)
		}
		if cycleHook != nil {
			cycleHook(name, dur, err, result)
		}
	}

	runOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			runOnce()
		}
	}
}

// startTickerGoroutine launches runTickerLoop in a goroutine and registers it with wg.
func startTickerGoroutine(wg *sync.WaitGroup, ctx context.Context, log *zap.Logger, name string, every, cycleTimeout time.Duration, cycleHook CycleEndMetricsHook, fn func(context.Context) error) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		runTickerLoop(ctx, log, name, every, cycleTimeout, cycleHook, fn)
	}()
}
