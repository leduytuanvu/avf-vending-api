package background

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestEffectivePeriodicCycleTimeout(t *testing.T) {
	require.Equal(t, 45*time.Second, EffectivePeriodicCycleTimeout(20*time.Second, 0))
	require.Equal(t, 45*time.Second, EffectivePeriodicCycleTimeout(25*time.Second, 0))
	require.Equal(t, 45*time.Second, EffectivePeriodicCycleTimeout(29*time.Second, 0))
	require.Equal(t, 90*time.Second, EffectivePeriodicCycleTimeout(1*time.Minute, 0))
	require.Equal(t, 12*time.Minute, EffectivePeriodicCycleTimeout(10*time.Minute, 0))
	require.Equal(t, 99*time.Second, EffectivePeriodicCycleTimeout(20*time.Second, 99*time.Second))
}

func TestRunTickerLoop_CancelStopsPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var runs int32
	done := make(chan struct{})
	go func() {
		runTickerLoop(ctx, zap.NewNop(), "test_job", 50*time.Millisecond, 200*time.Millisecond, nil, func(c context.Context) error {
			atomic.AddInt32(&runs, 1)
			<-c.Done()
			return c.Err()
		})
		close(done)
	}()
	time.Sleep(60 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runTickerLoop did not exit after cancel")
	}
	require.GreaterOrEqual(t, atomic.LoadInt32(&runs), int32(1))
}

func TestRunTickerLoop_LogsErrorOnFailedCycle(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runTickerLoop(ctx, log, "failing_job", time.Hour, 5*time.Second, nil, func(context.Context) error {
			return errors.New("simulated cycle failure")
		})
		close(done)
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runTickerLoop did not exit")
	}
	found := false
	for _, e := range logs.All() {
		if e.Message == "background_cycle_end" && e.Level == zap.ErrorLevel {
			found = true
			break
		}
	}
	require.True(t, found, "expected Error-level background_cycle_end log")
}
