package telemetryapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	tel "github.com/avf/avf-vending-api/internal/platform/telemetry"
	natssrv "github.com/nats-io/nats.go"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// JetStreamWorkersConfig wires durable telemetry consumers into Postgres projections.
type JetStreamWorkersConfig struct {
	Log         *zap.Logger
	NC          *natssrv.Conn
	JS          natssrv.JetStreamContext
	Store       *postgres.Store
	Telemetry   config.TelemetryJetStreamConfig
	Limits      platformnats.TelemetryBrokerLimits
}

// JetStreamWorkers runs durable pull consumers that project telemetry into Postgres.
type JetStreamWorkers struct {
	log   *zap.Logger
	nc    *natssrv.Conn
	js    natssrv.JetStreamContext
	store *postgres.Store
	cfg   config.TelemetryJetStreamConfig
	lim   platformnats.TelemetryBrokerLimits

	seqDedupe *boundedSeenSet
	idemGuard *idempotencyPayloadGuard
	sem       *semaphore.Weighted

	inFlight atomic.Int32

	failStreak sync.Map // durable name -> *int32
}

// NewJetStreamWorkers validates config and returns a worker handle.
func NewJetStreamWorkers(c JetStreamWorkersConfig) *JetStreamWorkers {
	if c.Log == nil || c.NC == nil || c.JS == nil || c.Store == nil {
		panic("telemetryapp.NewJetStreamWorkers: nil dependency")
	}
	n := int64(c.Telemetry.ProjectionMaxConcurrency)
	if n < 1 {
		n = 1
	}
	idemN := c.Telemetry.ProjectionDedupeLRUSize / 4
	if idemN < 4096 {
		idemN = 4096
	}
	return &JetStreamWorkers{
		log:       c.Log,
		nc:        c.NC,
		js:        c.JS,
		store:     c.Store,
		cfg:       c.Telemetry,
		lim:       platformnats.NormalizeTelemetryBrokerLimits(c.Limits),
		seqDedupe: newBoundedSeenSet(c.Telemetry.ProjectionDedupeLRUSize),
		idemGuard: newIdempotencyPayloadGuard(idemN),
		sem:       semaphore.NewWeighted(n),
	}
}

// Start blocks until ctx is cancelled.
func (w *JetStreamWorkers) Start(ctx context.Context) error {
	if w == nil || w.nc == nil || w.store == nil {
		return errors.New("telemetryapp: nil JetStreamWorkers")
	}
	js := w.js
	if js == nil {
		var err error
		js, err = w.nc.JetStream()
		if err != nil {
			return err
		}
		w.js = js
	}
	if err := platformnats.EnsureTelemetryStreams(js, w.lim); err != nil {
		return err
	}
	if err := platformnats.EnsureTelemetryDurableConsumers(js, w.lim); err != nil {
		return err
	}

	pollCtx, cancelPoll := context.WithCancel(ctx)
	defer cancelPoll()
	go w.lagPollLoop(pollCtx)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryHeartbeat, "avf-w-telemetry-heartbeat", platformnats.SubjectTelemetryPrefix+"heartbeat.>", false, w.handleHeartbeat) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryState, "avf-w-telemetry-state", platformnats.SubjectTelemetryPrefix+"state.>", false, w.handleState) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryMetrics, "avf-w-telemetry-metrics", platformnats.SubjectTelemetryPrefix+"metrics.>", true, w.handleMetrics) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryIncidents, "avf-w-telemetry-incidents", platformnats.SubjectTelemetryPrefix+"incident.>", false, w.handleIncident) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryCommandReceipts, "avf-w-telemetry-command-receipts", platformnats.SubjectTelemetryPrefix+"command_receipt.>", false, w.handleCommandReceipt) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryDiagnosticBundleReady, "avf-w-telemetry-diagnostic", platformnats.SubjectTelemetryPrefix+"diagnostic_bundle_ready.>", false, w.handleDiagnostic) })
	if w.log != nil {
		w.log.Info("telemetry_jetstream_workers_started")
	}
	return g.Wait()
}

type msgHandler func(ctx context.Context, env tel.Envelope) error

func (w *JetStreamWorkers) lagPollLoop(ctx context.Context) {
	interval := w.cfg.ConsumerLagPollInterval
	if interval < time.Second {
		interval = 15 * time.Second
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			js := w.js
			if js == nil {
				continue
			}
			for _, p := range platformnats.TelemetryDurablePairs() {
				info, err := js.ConsumerInfo(p.Stream, p.Durable)
				if err != nil {
					continue
				}
				telemetryConsumerLag.WithLabelValues(p.Stream, p.Durable).Set(float64(info.NumPending))
			}
		}
	}
}

func (w *JetStreamWorkers) pullLoop(ctx context.Context, js natssrv.JetStreamContext, stream, durable, filter string, metricsStream bool, h msgHandler) error {
	sub, err := js.PullSubscribe(filter, durable, natssrv.BindStream(stream))
	if err != nil {
		return fmt.Errorf("telemetry pull subscribe %s: %w", durable, err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	batch := w.lim.ConsumerFetchBatch
	wait := w.lim.ConsumerFetchMaxWait
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		batchStart := time.Now()
		msgs, err := sub.Fetch(batch, natssrv.MaxWait(wait))
		if err != nil {
			if errors.Is(err, natssrv.ErrTimeout) {
				continue
			}
			telemetryProjectionFailures.WithLabelValues("fetch_err").Inc()
			if w.log != nil {
				w.log.Debug("telemetry_fetch_idle", zap.String("durable", durable), zap.Error(err))
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		telemetryProjectionBatchSize.Observe(float64(len(msgs)))
		for _, m := range msgs {
			if err := w.sem.Acquire(ctx, 1); err != nil {
				return err
			}
			w.inFlight.Add(1)
			telemetryProjectionBacklog.Set(float64(w.inFlight.Load()))
			func() {
				defer w.sem.Release(1)
				defer func() {
					w.inFlight.Add(-1)
					telemetryProjectionBacklog.Set(float64(w.inFlight.Load()))
				}()

				var env tel.Envelope
				if err := json.Unmarshal(m.Data, &env); err != nil {
					if w.log != nil {
						w.log.Warn("telemetry_malformed_envelope", zap.Error(err), zap.String("subject", m.Subject))
					}
					telemetryProjectionFailures.WithLabelValues("malformed_json").Inc()
					_ = m.Term()
					w.markOK(durable)
					return
				}

				if metricsStream {
					if skip, reason := w.metricsDedupeAction(m, durable, stream, &env); skip {
						recordTelemetryDuplicate(reason)
						_ = m.Ack()
						w.markOK(durable)
						return
					}
				}

				if err := h(ctx, env); err != nil {
					if w.log != nil {
						w.log.Warn("telemetry_handler_error", zap.Error(err), zap.String("subject", m.Subject))
					}
					telemetryProjectionFailures.WithLabelValues("handler_err").Inc()
					w.markFail(durable)
					_ = m.Nak()
					return
				}
				w.markOK(durable)
				if metricsStream {
					w.rememberMetricsDedupe(m, durable, stream, &env)
				}
				_ = m.Ack()
			}()
		}
		telemetryProjectionFlushSeconds.Observe(time.Since(batchStart).Seconds())
	}
}

func (w *JetStreamWorkers) metricsDedupeAction(m *natssrv.Msg, durable, stream string, env *tel.Envelope) (skip bool, reason string) {
	md, err := m.Metadata()
	if err == nil && md != nil {
		seqKey := fmt.Sprintf("%s|%s|%d", stream, durable, md.Sequence.Stream)
		if w.seqDedupe.contains(seqKey) {
			return true, "stream_seq"
		}
	}
	idem := strings.TrimSpace(env.Idempotency)
	if idem != "" {
		ph := hashPayload(env.Payload)
		dup, conflict := w.idemGuard.check(idem, ph)
		if conflict {
			telemetryIdempotencyConflictTotal.Inc()
			return true, "idempotency_conflict"
		}
		if dup {
			return true, "idempotency_replay"
		}
	}
	return false, ""
}

func (w *JetStreamWorkers) rememberMetricsDedupe(m *natssrv.Msg, durable, stream string, env *tel.Envelope) {
	md, err := m.Metadata()
	if err == nil && md != nil {
		seqKey := fmt.Sprintf("%s|%s|%d", stream, durable, md.Sequence.Stream)
		w.seqDedupe.remember(seqKey)
	}
	if idem := strings.TrimSpace(env.Idempotency); idem != "" {
		w.idemGuard.remember(idem, hashPayload(env.Payload))
	}
}

func (w *JetStreamWorkers) markFail(durable string) {
	v, _ := w.failStreak.LoadOrStore(durable, new(int32))
	p := v.(*int32)
	atomic.AddInt32(p, 1)
	w.refreshMaxFailGauge()
}

func (w *JetStreamWorkers) markOK(durable string) {
	if v, ok := w.failStreak.Load(durable); ok {
		atomic.StoreInt32(v.(*int32), 0)
	}
	w.refreshMaxFailGauge()
}

func (w *JetStreamWorkers) refreshMaxFailGauge() {
	max := 0
	w.failStreak.Range(func(_, v interface{}) bool {
		n := int(atomic.LoadInt32(v.(*int32)))
		if n > max {
			max = n
		}
		return true
	})
	telemetryProjectionFailConsecutiveMax.Set(float64(max))
}

func (w *JetStreamWorkers) maxFailStreak() int {
	max := 0
	w.failStreak.Range(func(_, v interface{}) bool {
		n := int(atomic.LoadInt32(v.(*int32)))
		if n > max {
			max = n
		}
		return true
	})
	return max
}

// Ready evaluates worker-side telemetry overload gates for HTTP readiness.
func (w *JetStreamWorkers) Ready(ctx context.Context, js natssrv.JetStreamContext) error {
	if w == nil {
		return fmt.Errorf("telemetryapp: nil JetStreamWorkers")
	}
	_ = ctx
	if w.cfg.ReadinessMaxPending > 0 && js != nil {
		if m, err := w.maxConsumerPending(js); err == nil && m > w.cfg.ReadinessMaxPending {
			return fmt.Errorf("telemetry readiness: max consumer NumPending=%d exceeds TELEMETRY_READINESS_MAX_PENDING=%d", m, w.cfg.ReadinessMaxPending)
		}
	}
	if w.cfg.ReadinessMaxProjectionFailStreak > 0 {
		if m := w.maxFailStreak(); m >= w.cfg.ReadinessMaxProjectionFailStreak {
			return fmt.Errorf("telemetry readiness: projection fail streak=%d exceeds TELEMETRY_READINESS_MAX_PROJECTION_FAIL_STREAK=%d", m, w.cfg.ReadinessMaxProjectionFailStreak)
		}
	}
	return nil
}

func (w *JetStreamWorkers) maxConsumerPending(js natssrv.JetStreamContext) (int64, error) {
	var max uint64
	for _, p := range platformnats.TelemetryDurablePairs() {
		info, err := js.ConsumerInfo(p.Stream, p.Durable)
		if err != nil {
			return 0, err
		}
		if info.NumPending > max {
			max = info.NumPending
		}
	}
	return int64(max), nil
}

func (w *JetStreamWorkers) handleHeartbeat(ctx context.Context, env tel.Envelope) error {
	return w.store.UpsertHeartbeatSnapshot(ctx, env.MachineID, env.ReceivedAt.UTC())
}

func (w *JetStreamWorkers) handleState(ctx context.Context, env tel.Envelope) error {
	return w.store.ApplyShadowReportedProjection(ctx, env.MachineID, env.Payload, ptrStr(env.AppVersion), ptrStr(env.FirmwareVer))
}

func (w *JetStreamWorkers) handleMetrics(ctx context.Context, env tel.Envelope) error {
	data := unwrapTelemetryData(env.Payload)
	samples := postgres.ParseMetricsPayload(data)
	if len(samples) == 0 {
		return nil
	}
	ts := env.ReceivedAt.UTC()
	for k, v := range samples {
		vv := v
		if err := w.store.MergeTelemetryRollupMinute(ctx, env.MachineID, ts, k, 1, &vv, &vv, &vv, &vv, nil); err != nil {
			return err
		}
	}
	return nil
}

func (w *JetStreamWorkers) handleIncident(ctx context.Context, env tel.Envelope) error {
	data := unwrapTelemetryData(env.Payload)
	sev, code, title, dedupe, err := postgres.ParseIncidentPayload(data)
	if err != nil {
		return nil
	}
	detail := env.Payload
	if len(detail) == 0 {
		detail = []byte("{}")
	}
	return w.store.UpsertMachineIncidentDeduped(ctx, env.MachineID, sev, code, title, detail, dedupe)
}

func (w *JetStreamWorkers) handleCommandReceipt(ctx context.Context, env tel.Envelope) error {
	var inner struct {
		Sequence      int64           `json:"sequence"`
		Status        string          `json:"status"`
		CorrelationID *uuid.UUID      `json:"correlation_id"`
		Payload       json.RawMessage `json:"payload"`
		DedupeKey     string          `json:"dedupe_key"`
	}
	if err := json.Unmarshal(env.Payload, &inner); err != nil {
		return err
	}
	if inner.DedupeKey == "" {
		return fmt.Errorf("telemetryapp: command receipt missing dedupe_key")
	}
	raw := []byte(inner.Payload)
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	_, err := w.store.ApplyCommandReceiptTransition(ctx, postgres.CommandReceiptTransitionParams{
		MachineID:     env.MachineID,
		Sequence:      inner.Sequence,
		Status:        strings.ToLower(strings.TrimSpace(inner.Status)),
		CorrelationID: inner.CorrelationID,
		Payload:       raw,
		DedupeKey:     inner.DedupeKey,
	})
	return err
}

func (w *JetStreamWorkers) handleDiagnostic(ctx context.Context, env tel.Envelope) error {
	data := unwrapTelemetryData(env.Payload)
	sk, prov, ct, size, sha, err := postgres.ParseDiagnosticManifestPayload(data)
	if err != nil {
		return nil
	}
	meta := map[string]any{"envelope_event": env.SourceEvent}
	mb, _ := json.Marshal(meta)
	return w.store.InsertDiagnosticBundleManifestRow(ctx, env.MachineID, sk, prov, ct, size, sha, mb, nil)
}

func ptrStr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func unwrapTelemetryData(payload []byte) []byte {
	var w struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &w); err == nil && len(w.Data) > 0 {
		return w.Data
	}
	return payload
}
