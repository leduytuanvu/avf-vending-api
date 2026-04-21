package telemetryapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	tel "github.com/avf/avf-vending-api/internal/platform/telemetry"
	natssrv "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// NATSBridge republishes device MQTT ingest to JetStream by class and applies light OLTP touches only.
// Raw heartbeat/metrics are not written to device_telemetry_events.
type NATSBridge struct {
	Log   *zap.Logger
	JS    natssrv.JetStreamContext
	Store *postgres.Store
}

var _ platformmqtt.DeviceIngest = (*NATSBridge)(nil)

func (b *NATSBridge) IngestTelemetry(ctx context.Context, in platformmqtt.TelemetryIngest) error {
	if b == nil || b.Store == nil || b.JS == nil {
		return errors.New("telemetryapp: nil NATSBridge")
	}
	maxN := tel.MaxIngestPayloadBytes()
	if len(in.Payload) > maxN {
		return fmt.Errorf("telemetryapp: payload exceeds TELEMETRY_MAX_INGEST_BYTES (%d)", maxN)
	}
	cls := tel.ClassifyEventType(in.EventType)
	loc, err := b.Store.GetMachineOrgSite(ctx, in.MachineID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	sv := in.SchemaVersion
	if sv == 0 {
		sv = tel.DefaultSchemaVersion
	}
	env := tel.Envelope{
		SchemaVersion:     sv,
		Class:             cls,
		MachineID:         in.MachineID,
		TenantID:          &loc.OrganizationID,
		SiteID:            &loc.SiteID,
		ReceivedAt:        now,
		SourceEvent:       in.EventType,
		Payload:           wrapTelemetryPayload(in.EventType, in.Payload),
		EventID:           in.EventID,
		BootID:            in.BootID,
		SeqNo:             in.SeqNo,
		CorrelationID:     in.CorrelationID,
		OperatorSessionID: in.OperatorSessionID,
	}
	if in.OccurredAt != nil {
		t := in.OccurredAt.UTC()
		env.EmittedAt = &t
	}
	if idem := platformmqtt.TelemetryIdempotencyKey(in.MachineID, in); idem != "" {
		env.Idempotency = idem
	}
	body, err := env.Marshal()
	if err != nil {
		return err
	}
	dedupe := env.Idempotency
	if dedupe == "" {
		dedupe = fmt.Sprintf("%s:%s:%d", in.MachineID.String(), in.EventType, now.UnixNano())
	}
	if err := platformnats.PublishTelemetry(b.JS, cls, in.MachineID, body, dedupe); err != nil {
		RecordTelemetryPublishFailure()
		return err
	}
	if err := b.Store.TouchMachineConnectivityFast(ctx, in.MachineID); err != nil {
		if b.Log != nil {
			b.Log.Warn("telemetry_bridge_touch_connectivity", zap.Error(err), zap.String("machine_id", in.MachineID.String()))
		}
	}
	return nil
}

func wrapTelemetryPayload(eventType string, payload []byte) json.RawMessage {
	w := map[string]json.RawMessage{}
	et, _ := json.Marshal(eventType)
	w["event_type"] = et
	w["data"] = json.RawMessage(payload)
	b, _ := json.Marshal(w)
	return b
}

func (b *NATSBridge) IngestShadowReported(ctx context.Context, in platformmqtt.ShadowReportedIngest) error {
	if b == nil || b.Store == nil || b.JS == nil {
		return errors.New("telemetryapp: nil NATSBridge")
	}
	if len(in.ReportedJSON) > tel.MaxIngestPayloadBytes() {
		return fmt.Errorf("telemetryapp: shadow payload too large")
	}
	loc, err := b.Store.GetMachineOrgSite(ctx, in.MachineID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	env := tel.Envelope{
		SchemaVersion: tel.DefaultSchemaVersion,
		Class:         tel.ClassState,
		MachineID:     in.MachineID,
		TenantID:      &loc.OrganizationID,
		SiteID:        &loc.SiteID,
		ReceivedAt:    now,
		SourceEvent:   "shadow.reported",
		Payload:       in.ReportedJSON,
	}
	body, err := env.Marshal()
	if err != nil {
		return err
	}
	dedupe := fmt.Sprintf("shadow:%s:%d", in.MachineID.String(), now.UnixNano())
	if err := platformnats.PublishTelemetry(b.JS, tel.ClassState, in.MachineID, body, dedupe); err != nil {
		RecordTelemetryPublishFailure()
		return err
	}
	if err := b.Store.TouchMachineConnectivityFast(ctx, in.MachineID); err != nil && b.Log != nil {
		b.Log.Warn("telemetry_bridge_touch_connectivity", zap.Error(err), zap.String("machine_id", in.MachineID.String()))
	}
	return nil
}

func (b *NATSBridge) IngestShadowDesired(ctx context.Context, in platformmqtt.ShadowDesiredIngest) error {
	if b == nil || b.Store == nil || b.JS == nil {
		return errors.New("telemetryapp: nil NATSBridge")
	}
	if len(in.DesiredJSON) > tel.MaxIngestPayloadBytes() {
		return fmt.Errorf("telemetryapp: shadow desired payload too large")
	}
	loc, err := b.Store.GetMachineOrgSite(ctx, in.MachineID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	env := tel.Envelope{
		SchemaVersion: tel.DefaultSchemaVersion,
		Class:         tel.ClassState,
		MachineID:     in.MachineID,
		TenantID:      &loc.OrganizationID,
		SiteID:        &loc.SiteID,
		ReceivedAt:    now,
		SourceEvent:   "shadow.desired",
		Payload:       in.DesiredJSON,
	}
	body, err := env.Marshal()
	if err != nil {
		return err
	}
	dedupe := fmt.Sprintf("shadow-desired:%s:%d", in.MachineID.String(), now.UnixNano())
	if err := platformnats.PublishTelemetry(b.JS, tel.ClassState, in.MachineID, body, dedupe); err != nil {
		RecordTelemetryPublishFailure()
		return err
	}
	if err := b.Store.TouchMachineConnectivityFast(ctx, in.MachineID); err != nil && b.Log != nil {
		b.Log.Warn("telemetry_bridge_touch_connectivity", zap.Error(err), zap.String("machine_id", in.MachineID.String()))
	}
	return nil
}

func (b *NATSBridge) IngestCommandReceipt(ctx context.Context, in platformmqtt.CommandReceiptIngest) error {
	if b == nil || b.Store == nil || b.JS == nil {
		return errors.New("telemetryapp: nil NATSBridge")
	}
	if len(in.Payload) > tel.MaxIngestPayloadBytes() {
		return fmt.Errorf("telemetryapp: command receipt payload too large")
	}
	loc, err := b.Store.GetMachineOrgSite(ctx, in.MachineID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	wire, _ := json.Marshal(map[string]any{
		"sequence":       in.Sequence,
		"status":         in.Status,
		"correlation_id": in.CorrelationID,
		"payload":        json.RawMessage(in.Payload),
		"dedupe_key":     in.DedupeKey,
	})
	env := tel.Envelope{
		SchemaVersion: tel.DefaultSchemaVersion,
		Class:         tel.ClassCommandReceipt,
		MachineID:     in.MachineID,
		TenantID:      &loc.OrganizationID,
		SiteID:        &loc.SiteID,
		ReceivedAt:    now,
		SourceEvent:   "commands.receipt",
		Payload:       wire,
		Idempotency:   in.DedupeKey,
	}
	body, err := env.Marshal()
	if err != nil {
		return err
	}
	if err := platformnats.PublishTelemetry(b.JS, tel.ClassCommandReceipt, in.MachineID, body, in.DedupeKey); err != nil {
		RecordTelemetryPublishFailure()
		return err
	}
	if err := b.Store.TouchMachineConnectivityFast(ctx, in.MachineID); err != nil && b.Log != nil {
		b.Log.Warn("telemetry_bridge_touch_connectivity", zap.Error(err), zap.String("machine_id", in.MachineID.String()))
	}
	return nil
}

// LegacyStoreIngest wraps the Postgres store for direct-to-OLTP ingest when NATS is unavailable.
type LegacyStoreIngest struct {
	Store *postgres.Store
}

var _ platformmqtt.DeviceIngest = (*LegacyStoreIngest)(nil)

func (l *LegacyStoreIngest) IngestTelemetry(ctx context.Context, in platformmqtt.TelemetryIngest) error {
	return l.Store.IngestTelemetry(ctx, in)
}

func (l *LegacyStoreIngest) IngestShadowReported(ctx context.Context, in platformmqtt.ShadowReportedIngest) error {
	return l.Store.IngestShadowReported(ctx, in)
}

func (l *LegacyStoreIngest) IngestShadowDesired(ctx context.Context, in platformmqtt.ShadowDesiredIngest) error {
	return l.Store.IngestShadowDesired(ctx, in)
}

func (l *LegacyStoreIngest) IngestCommandReceipt(ctx context.Context, in platformmqtt.CommandReceiptIngest) error {
	return l.Store.IngestCommandReceipt(ctx, in)
}

// SelectIngest returns JetStream bridge when JS is configured. In production, JetStream is mandatory
// and TELEMETRY_LEGACY_POSTGRES_INGEST must be false (also enforced in config.Validate).
func SelectIngest(log *zap.Logger, store *postgres.Store, js natssrv.JetStreamContext, appEnv config.AppEnvironment, legacyPostgres bool) (platformmqtt.DeviceIngest, error) {
	if store == nil {
		panic("telemetryapp.SelectIngest: nil store")
	}
	if appEnv == config.AppEnvProduction {
		if js == nil {
			return nil, fmt.Errorf("telemetryapp: APP_ENV=production requires NATS JetStream for device telemetry (set %s and ensure mqtt-ingest can connect)", platformnats.EnvNATSURL)
		}
		if legacyPostgres {
			return nil, fmt.Errorf("telemetryapp: APP_ENV=production forbids legacy Postgres telemetry hot path (TELEMETRY_LEGACY_POSTGRES_INGEST)")
		}
		if log != nil {
			log.Info("mqtt_ingest_telemetry_mode", zap.String("mode", "nats_jetstream_bridge"))
		}
		return &NATSBridge{Log: log, JS: js, Store: store}, nil
	}
	if js != nil {
		if log != nil {
			log.Info("mqtt_ingest_telemetry_mode", zap.String("mode", "nats_jetstream_bridge"))
		}
		return &NATSBridge{Log: log, JS: js, Store: store}, nil
	}
	if legacyPostgres {
		if log != nil {
			log.Info("mqtt_ingest_telemetry_mode", zap.String("mode", "legacy_postgres_hot_path"),
				zap.String("reason", "TELEMETRY_LEGACY_POSTGRES_INGEST=true"))
		}
		return &LegacyStoreIngest{Store: store}, nil
	}
	if log != nil {
		log.Warn("mqtt_ingest_telemetry_mode", zap.String("mode", "legacy_postgres_hot_path"),
			zap.String("note", "set NATS_URL for JetStream-backed telemetry; TELEMETRY_LEGACY_POSTGRES_INGEST=false by default"))
	}
	return &LegacyStoreIngest{Store: store}, nil
}
