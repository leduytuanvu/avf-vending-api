package telemetryapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/modules/postgres"
	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	tel "github.com/avf/avf-vending-api/internal/platform/telemetry"
	natssrv "github.com/nats-io/nats.go"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// JetStreamWorkers runs durable pull consumers that project telemetry into Postgres.
type JetStreamWorkers struct {
	Log   *zap.Logger
	NC    *natssrv.Conn
	Store *postgres.Store
}

// Start blocks until ctx is cancelled.
func (w *JetStreamWorkers) Start(ctx context.Context) error {
	if w == nil || w.NC == nil || w.Store == nil {
		return errors.New("telemetryapp: nil JetStreamWorkers")
	}
	js, err := w.NC.JetStream()
	if err != nil {
		return err
	}
	if err := platformnats.EnsureTelemetryStreams(js); err != nil {
		return err
	}
	if err := platformnats.EnsureTelemetryDurableConsumers(js); err != nil {
		return err
	}
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryHeartbeat, "avf-w-telemetry-heartbeat", platformnats.SubjectTelemetryPrefix+"heartbeat.>", w.handleHeartbeat) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryState, "avf-w-telemetry-state", platformnats.SubjectTelemetryPrefix+"state.>", w.handleState) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryMetrics, "avf-w-telemetry-metrics", platformnats.SubjectTelemetryPrefix+"metrics.>", w.handleMetrics) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryIncidents, "avf-w-telemetry-incidents", platformnats.SubjectTelemetryPrefix+"incident.>", w.handleIncident) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryCommandReceipts, "avf-w-telemetry-command-receipts", platformnats.SubjectTelemetryPrefix+"command_receipt.>", w.handleCommandReceipt) })
	g.Go(func() error { return w.pullLoop(gctx, js, platformnats.StreamTelemetryDiagnosticBundleReady, "avf-w-telemetry-diagnostic", platformnats.SubjectTelemetryPrefix+"diagnostic_bundle_ready.>", w.handleDiagnostic) })
	if w.Log != nil {
		w.Log.Info("telemetry_jetstream_workers_started")
	}
	return g.Wait()
}

type msgHandler func(ctx context.Context, env tel.Envelope) error

func (w *JetStreamWorkers) pullLoop(ctx context.Context, js natssrv.JetStreamContext, stream, durable, filter string, h msgHandler) error {
	sub, err := js.PullSubscribe(filter, durable, natssrv.BindStream(stream))
	if err != nil {
		return fmt.Errorf("telemetry pull subscribe %s: %w", durable, err)
	}
	defer func() { _ = sub.Unsubscribe() }()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msgs, err := sub.Fetch(32, natssrv.MaxWait(2*time.Second))
		if err != nil {
			if errors.Is(err, natssrv.ErrTimeout) {
				continue
			}
			if w.Log != nil {
				w.Log.Debug("telemetry_fetch_idle", zap.String("durable", durable), zap.Error(err))
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		for _, m := range msgs {
			var env tel.Envelope
			if err := json.Unmarshal(m.Data, &env); err != nil {
				if w.Log != nil {
					w.Log.Warn("telemetry_malformed_envelope", zap.Error(err), zap.String("subject", m.Subject))
				}
				_ = m.Term() // poison — do not infinite redeliver malformed
				continue
			}
			if err := h(ctx, env); err != nil {
				if w.Log != nil {
					w.Log.Warn("telemetry_handler_error", zap.Error(err), zap.String("subject", m.Subject))
				}
				_ = m.Nak()
				continue
			}
			_ = m.Ack()
		}
	}
}

func (w *JetStreamWorkers) handleHeartbeat(ctx context.Context, env tel.Envelope) error {
	return w.Store.UpsertHeartbeatSnapshot(ctx, env.MachineID, env.ReceivedAt.UTC())
}

func (w *JetStreamWorkers) handleState(ctx context.Context, env tel.Envelope) error {
	return w.Store.ApplyShadowReportedProjection(ctx, env.MachineID, env.Payload, ptrStr(env.AppVersion), ptrStr(env.FirmwareVer))
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
		if err := w.Store.MergeTelemetryRollupMinute(ctx, env.MachineID, ts, k, 1, &vv, &vv, &vv, &vv, nil); err != nil {
			return err
		}
	}
	return nil
}

func (w *JetStreamWorkers) handleIncident(ctx context.Context, env tel.Envelope) error {
	data := unwrapTelemetryData(env.Payload)
	sev, code, title, dedupe, err := postgres.ParseIncidentPayload(data)
	if err != nil {
		return nil // ignore malformed incident payloads (bounded visibility)
	}
	detail := env.Payload
	if len(detail) == 0 {
		detail = []byte("{}")
	}
	return w.Store.UpsertMachineIncidentDeduped(ctx, env.MachineID, sev, code, title, detail, dedupe)
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
	_, err := w.Store.ApplyCommandReceiptTransition(ctx, postgres.CommandReceiptTransitionParams{
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
	return w.Store.InsertDiagnosticBundleManifestRow(ctx, env.MachineID, sk, prov, ct, size, sha, mb, nil)
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
