package telemetryapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	tel "github.com/avf/avf-vending-api/internal/platform/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

var ErrRetryableTelemetryBackpressure = errors.New("telemetryapp: retryable backpressure")

type pipelineJob struct {
	kind        string
	criticality tel.Criticality
	compactKey  string
	started     atomic.Bool
	mu          sync.Mutex
	fn          func(context.Context) error
}

func (j *pipelineJob) replaceFn(fn func(context.Context) error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.fn = fn
}

func (j *pipelineJob) run(ctx context.Context) error {
	j.mu.Lock()
	fn := j.fn
	j.mu.Unlock()
	return fn(ctx)
}

// BoundedDeviceIngest wraps a DeviceIngest with a bounded work queue, worker pool, and optional
// per-machine token-bucket limits for telemetry only.
type BoundedDeviceIngest struct {
	log   *zap.Logger
	inner platformmqtt.DeviceIngest
	cfg   config.MQTTDeviceTelemetryConfig

	ch        chan *pipelineJob
	wg        sync.WaitGroup
	closeOnce sync.Once
	limMu     sync.Mutex
	limits    map[uuid.UUID]*rate.Limiter
	compactMu sync.Mutex
	compact   map[string]*pipelineJob

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
		log:     log,
		inner:   inner,
		cfg:     tel,
		ch:      make(chan *pipelineJob, capacity),
		limits:  make(map[uuid.UUID]*rate.Limiter),
		compact: make(map[string]*pipelineJob),
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

func (p *BoundedDeviceIngest) recordAccepted(kind string, criticality tel.Criticality) {
	RecordTelemetryReceived(kind)
	if criticality == tel.CriticalityCriticalNoDrop {
		RecordTelemetryReceived(string(tel.CriticalityCriticalNoDrop))
		if p.log != nil {
			p.log.Debug("mqtt_ingest_critical_accepted", zap.String("kind", kind))
		}
	}
}

func (p *BoundedDeviceIngest) tryCompact(job *pipelineJob) bool {
	if job.compactKey == "" {
		return false
	}
	p.compactMu.Lock()
	defer p.compactMu.Unlock()
	existing, ok := p.compact[job.compactKey]
	if !ok {
		return false
	}
	existing.replaceFn(job.fn)
	return true
}

func (p *BoundedDeviceIngest) retryableBackpressure(reason string, kind string, criticality tel.Criticality) error {
	RecordTelemetryRejected(reason)
	if p.log != nil {
		p.log.Warn("mqtt_ingest_retryable_backpressure",
			zap.String("kind", kind),
			zap.String("criticality", string(criticality)),
			zap.String("reason", reason))
	}
	return fmt.Errorf("%w: %s", ErrRetryableTelemetryBackpressure, reason)
}

// ingestCriticalSync runs critical_no_drop work inline so Dispatch never returns success before
// durable downstream handling (e.g. JetStream publish) completes. Bounded memory is preserved by
// not enqueueing critical telemetry; droppable/compactable traffic still uses the bounded queue.
func (p *BoundedDeviceIngest) ingestCriticalSync(ctx context.Context, kind string, fn func(context.Context) error) error {
	err := fn(ctx)
	if err != nil {
		// Validation-style failures may already increment dedicated reject counters in the inner ingest (e.g. NATS bridge).
		if !errors.Is(err, tel.ErrCriticalTelemetryMissingIdentity) {
			RecordTelemetryRejected("handler_error")
		}
		if p.log != nil {
			p.log.Warn("mqtt_ingest_critical_failed", zap.String("kind", kind), zap.Error(err))
		}
		return err
	}
	RecordTelemetryReceived(kind)
	RecordTelemetryReceived(string(tel.CriticalityCriticalNoDrop))
	if p.log != nil {
		p.log.Debug("mqtt_ingest_critical_accepted", zap.String("kind", kind))
	}
	return nil
}

func (p *BoundedDeviceIngest) submit(ctx context.Context, kind string, criticality tel.Criticality, compactKey string, fn func(context.Context) error) error {
	job := &pipelineJob{kind: kind, criticality: criticality, compactKey: compactKey, fn: fn}
	if criticality == tel.CriticalityCompactableLatest && p.tryCompact(job) {
		p.recordAccepted(kind, criticality)
		return nil
	}
	if p.cfg.DropOnBackpressure {
		select {
		case p.ch <- job:
			if job.compactKey != "" {
				p.compactMu.Lock()
				if !job.started.Load() {
					p.compact[job.compactKey] = job
				}
				p.compactMu.Unlock()
			}
			p.queued.Add(1)
			p.updateQueueGauge()
			p.recordAccepted(kind, criticality)
			return nil
		default:
			switch criticality {
			case tel.CriticalityDroppableMetrics:
				RecordTelemetryDropped("droppable_queue_full")
				if p.log != nil {
					p.log.Warn("mqtt_ingest_dropped_droppable",
						zap.String("kind", kind),
						zap.String("criticality", string(criticality)),
						zap.String("reason", "droppable_queue_full"))
				}
				return nil
			case tel.CriticalityCompactableLatest:
				if p.tryCompact(job) {
					p.recordAccepted(kind, criticality)
					return nil
				}
				return p.retryableBackpressure("compactable_queue_full", kind, criticality)
			default:
				return p.retryableBackpressure("critical_queue_full", kind, criticality)
			}
		}
	}
	wait := p.cfg.SubmitWaitMs
	if wait < 1 {
		wait = 1
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(wait)*time.Millisecond)
	defer cancel()
	select {
	case p.ch <- job:
		if job.compactKey != "" {
			p.compactMu.Lock()
			if !job.started.Load() {
				p.compact[job.compactKey] = job
			}
			p.compactMu.Unlock()
		}
		p.queued.Add(1)
		p.updateQueueGauge()
		p.recordAccepted(kind, criticality)
		return nil
	case <-tctx.Done():
		switch criticality {
		case tel.CriticalityDroppableMetrics:
			RecordTelemetryRejected("droppable_queue_full_timeout")
		case tel.CriticalityCompactableLatest:
			RecordTelemetryRejected("compactable_queue_full_timeout")
		default:
			RecordTelemetryRejected("critical_queue_full_timeout")
		}
		return fmt.Errorf("%w: bounded queue full (timeout %dms)", ErrRetryableTelemetryBackpressure, p.cfg.SubmitWaitMs)
	}
}

func (p *BoundedDeviceIngest) worker() {
	defer p.wg.Done()
	for job := range p.ch {
		p.queued.Add(-1)
		p.updateQueueGauge()
		job.started.Store(true)
		if job.compactKey != "" {
			p.compactMu.Lock()
			current, ok := p.compact[job.compactKey]
			if ok && current == job {
				delete(p.compact, job.compactKey)
			}
			p.compactMu.Unlock()
		}
		exec := context.Background()
		if err := job.run(exec); err != nil && p.log != nil {
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
		RecordTelemetryRejected("rate_limited")
		return fmt.Errorf("%w: per-machine rate limit exceeded", ErrRetryableTelemetryBackpressure)
	}
	criticality := tel.CriticalityForEventType(in.EventType)
	if criticality == tel.CriticalityCriticalNoDrop {
		return p.ingestCriticalSync(ctx, "telemetry", func(exec context.Context) error {
			return p.inner.IngestTelemetry(exec, in)
		})
	}
	compactKey := ""
	if criticality == tel.CriticalityCompactableLatest {
		compactKey = fmt.Sprintf("%s:%s", in.MachineID.String(), strings.TrimSpace(strings.ToLower(in.EventType)))
	}
	return p.submit(ctx, "telemetry", criticality, compactKey, func(exec context.Context) error {
		return p.inner.IngestTelemetry(exec, in)
	})
}

func (p *BoundedDeviceIngest) IngestShadowReported(ctx context.Context, in platformmqtt.ShadowReportedIngest) error {
	return p.submit(ctx, "shadow_reported", tel.CriticalityCompactableLatest, fmt.Sprintf("%s:shadow_reported", in.MachineID.String()), func(exec context.Context) error {
		return p.inner.IngestShadowReported(exec, in)
	})
}

func (p *BoundedDeviceIngest) IngestShadowDesired(ctx context.Context, in platformmqtt.ShadowDesiredIngest) error {
	return p.submit(ctx, "shadow_desired", tel.CriticalityCompactableLatest, fmt.Sprintf("%s:shadow_desired", in.MachineID.String()), func(exec context.Context) error {
		return p.inner.IngestShadowDesired(exec, in)
	})
}

func (p *BoundedDeviceIngest) IngestCommandReceipt(ctx context.Context, in platformmqtt.CommandReceiptIngest) error {
	return p.ingestCriticalSync(ctx, "command_receipt", func(exec context.Context) error {
		return p.inner.IngestCommandReceipt(exec, in)
	})
}
