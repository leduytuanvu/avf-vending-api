package telemetryapp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type pipelineJob struct {
	kind string
	fn   func(context.Context) error
}

// BoundedDeviceIngest wraps a DeviceIngest with a bounded work queue, worker pool, and optional
// per-machine token-bucket limits for telemetry only.
type BoundedDeviceIngest struct {
	log   *zap.Logger
	inner platformmqtt.DeviceIngest
	cfg   config.MQTTDeviceTelemetryConfig

	ch        chan pipelineJob
	wg        sync.WaitGroup
	closeOnce sync.Once
	limMu     sync.Mutex
	limits    map[uuid.UUID]*rate.Limiter

	queued atomic.Int32
}

// NewBoundedDeviceIngest starts worker goroutines; call Close after the MQTT subscriber exits.
func NewBoundedDeviceIngest(log *zap.Logger, inner platformmqtt.DeviceIngest, tel config.MQTTDeviceTelemetryConfig) *BoundedDeviceIngest {
	if inner == nil {
		panic("telemetryapp.NewBoundedDeviceIngest: nil inner")
	}
	capacity := tel.GlobalMaxInflight
	if capacity < 1 {
		capacity = 8
	}
	workers := tel.WorkerConcurrency
	if workers < 1 {
		workers = 1
	}
	p := &BoundedDeviceIngest{
		log:    log,
		inner:  inner,
		cfg:    tel,
		ch:     make(chan pipelineJob, capacity),
		limits: make(map[uuid.UUID]*rate.Limiter),
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

func (p *BoundedDeviceIngest) updateQueueGauge() {
	SetTelemetryQueueDepth(float64(p.queued.Load()))
}

func (p *BoundedDeviceIngest) telemetryLimiter(id uuid.UUID) *rate.Limiter {
	p.limMu.Lock()
	defer p.limMu.Unlock()
	if lim, ok := p.limits[id]; ok {
		return lim
	}
	burst := p.cfg.PerMachineBurst
	if burst < 1 {
		burst = 1
	}
	lim := rate.NewLimiter(rate.Limit(p.cfg.PerMachineMsgsPerSec), burst)
	p.limits[id] = lim
	return lim
}

func (p *BoundedDeviceIngest) submit(ctx context.Context, kind string, fn func(context.Context) error) error {
	if p.cfg.DropOnBackpressure {
		select {
		case p.ch <- pipelineJob{kind: kind, fn: fn}:
			p.queued.Add(1)
			p.updateQueueGauge()
			RecordTelemetryReceived(kind)
			return nil
		default:
			RecordTelemetryDropped("queue_full")
			return nil
		}
	}
	wait := p.cfg.SubmitWaitMs
	if wait < 1 {
		wait = 1
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(wait)*time.Millisecond)
	defer cancel()
	select {
	case p.ch <- pipelineJob{kind: kind, fn: fn}:
		p.queued.Add(1)
		p.updateQueueGauge()
		RecordTelemetryReceived(kind)
		return nil
	case <-tctx.Done():
		RecordTelemetryRejected("queue_full_timeout")
		return fmt.Errorf("telemetryapp: bounded queue full (timeout %dms)", p.cfg.SubmitWaitMs)
	}
}

func (p *BoundedDeviceIngest) worker() {
	defer p.wg.Done()
	for job := range p.ch {
		p.queued.Add(-1)
		p.updateQueueGauge()
		exec := context.Background()
		if err := job.fn(exec); err != nil && p.log != nil {
			p.log.Warn("mqtt_ingest_pipeline_job_failed", zap.String("kind", job.kind), zap.Error(err))
		}
	}
}

// Close releases workers after draining the queue.
func (p *BoundedDeviceIngest) Close() {
	p.closeOnce.Do(func() {
		close(p.ch)
	})
	p.wg.Wait()
	SetTelemetryQueueDepth(0)
}

var _ platformmqtt.DeviceIngest = (*BoundedDeviceIngest)(nil)

func (p *BoundedDeviceIngest) IngestTelemetry(ctx context.Context, in platformmqtt.TelemetryIngest) error {
	if !p.telemetryLimiter(in.MachineID).Allow() {
		RecordTelemetryRateLimited()
		return fmt.Errorf("telemetryapp: per-machine rate limit exceeded")
	}
	return p.submit(ctx, "telemetry", func(exec context.Context) error {
		return p.inner.IngestTelemetry(exec, in)
	})
}

func (p *BoundedDeviceIngest) IngestShadowReported(ctx context.Context, in platformmqtt.ShadowReportedIngest) error {
	return p.submit(ctx, "shadow_reported", func(exec context.Context) error {
		return p.inner.IngestShadowReported(exec, in)
	})
}

func (p *BoundedDeviceIngest) IngestShadowDesired(ctx context.Context, in platformmqtt.ShadowDesiredIngest) error {
	return p.submit(ctx, "shadow_desired", func(exec context.Context) error {
		return p.inner.IngestShadowDesired(exec, in)
	})
}

func (p *BoundedDeviceIngest) IngestCommandReceipt(ctx context.Context, in platformmqtt.CommandReceiptIngest) error {
	return p.submit(ctx, "command_receipt", func(exec context.Context) error {
		return p.inner.IngestCommandReceipt(exec, in)
	})
}
