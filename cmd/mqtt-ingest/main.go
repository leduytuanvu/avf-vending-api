package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/avf/avf-vending-api/internal/app/telemetryapp"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/observability"
	platformdb "github.com/avf/avf-vending-api/internal/platform/db"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	"github.com/joho/godotenv"
	natssrv "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	cfg.ProcessName = "mqtt-ingest"

	log, err := observability.NewLogger(cfg)
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()
	if role := strings.TrimSpace(cfg.Runtime.RuntimeRole); role != "" && role != cfg.ProcessName {
		log.Fatal("runtime role mismatch", zap.String("configured_role", role), zap.String("process", cfg.ProcessName))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracer, err := observability.InitTracer(ctx, cfg)
	if err != nil {
		log.Fatal("tracer init", zap.Error(err))
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), cfg.Ops.TracerShutdownTimeout)
		defer cancel()
		if err := shutdownTracer(sctx); err != nil {
			log.Warn("tracer shutdown error", zap.Error(err))
		}
	}()

	if cfg.Postgres.URL == "" {
		log.Fatal("mqtt-ingest requires DATABASE_URL for persistence")
	}

	dbCtx, dbCancel := context.WithTimeout(ctx, 30*time.Second)
	pool, err := platformdb.NewPool(dbCtx, &cfg.Postgres)
	dbCancel()
	if err != nil {
		log.Fatal("postgres pool", zap.Error(err))
	}
	defer pool.Close()

	store := postgres.NewStore(pool)

	mqttCfg := platformmqtt.BrokerConfig{
		BrokerURL:   cfg.MQTT.BrokerURL,
		ClientID:    cfg.MQTT.ClientIDForProcess(cfg.ProcessName),
		Username:    cfg.MQTT.Username,
		Password:    cfg.MQTT.Password,
		TopicPrefix: cfg.MQTT.TopicPrefix,
	}
	if err := mqttCfg.Validate(); err != nil {
		log.Fatal("mqtt config", zap.Error(err))
	}

	var ingestHooks *platformmqtt.IngestHooks
	addr := strings.TrimSpace(cfg.MQTTIngestMetricsListen)
	if addr == "" {
		addr = "127.0.0.1:9093"
	}
	var js natssrv.JetStreamContext
	if cfg.MetricsEnabled {
		ingestHooks = telemetryapp.NewIngestHooks()
	}
	opsSrv := &http.Server{
		Addr: addr,
		Handler: observability.NewOperationsMux(cfg, log, cfg.MetricsEnabled, func(ctx context.Context) error {
			pctx, cancel := context.WithTimeout(ctx, cfg.Ops.ReadinessTimeout)
			defer cancel()
			if err := pool.Ping(pctx); err != nil {
				return err
			}
			if strings.TrimSpace(cfg.NATS.URL) != "" && js == nil {
				return errors.New("jetstream not configured")
			}
			return nil
		}),
	}
	go func() {
		paths := []string{"/health/live", "/health/ready"}
		if cfg.MetricsEnabled {
			paths = append([]string{"/metrics"}, paths...)
		}
		log.Info("mqtt_ingest_ops_listen", zap.String("addr", addr), zap.Strings("paths", paths))
		if err := opsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("mqtt-ingest ops server exited", zap.Error(err))
		}
	}()
	defer func() {
		observability.ShutdownHTTPServer(log, opsSrv, cfg.Ops.ShutdownTimeout, "mqtt-ingest ops shutdown")
	}()

	natsURL := strings.TrimSpace(cfg.NATS.URL)
	if natsURL != "" {
		nc, err := platformnats.ConnectJetStream(ctx, natsURL, "avf-mqtt-ingest-telemetry")
		if err != nil {
			log.Fatal("nats connect", zap.Error(err))
		}
		defer func() { _ = nc.Conn.Drain() }()
		js = nc.JS
		jsLim := cfg.TelemetryJetStream.NATSBrokerLimits()
		platformnats.LogTelemetryJetStreamRetention(log, "mqtt-ingest", string(cfg.AppEnv), jsLim)
		if err := platformnats.EnsureTelemetryStreams(js, jsLim); err != nil {
			log.Fatal("nats telemetry streams", zap.Error(err))
		}
	} else if cfg.AppEnv == config.AppEnvProduction {
		log.Fatal("mqtt-ingest production requires NATS_URL (validated at config load; set NATS_URL in the process environment)")
	}

	ing, err := telemetryapp.SelectIngest(log, store, js, cfg.AppEnv, cfg.MQTTDeviceTelemetry.LegacyPostgresIngest)
	if err != nil {
		log.Fatal("telemetry ingest mode", zap.Error(err))
	}

	pipeline := telemetryapp.NewBoundedDeviceIngest(log, ing, cfg.MQTTDeviceTelemetry)
	defer pipeline.Close()

	limits := &platformmqtt.TelemetryIngressLimits{
		MaxPayloadBytes: cfg.MQTTDeviceTelemetry.MaxPayloadBytes,
		MaxPoints:       cfg.MQTTDeviceTelemetry.MaxPointsPerMessage,
		MaxTags:         cfg.MQTTDeviceTelemetry.MaxTagsPerMessage,
	}

	sub, err := platformmqtt.NewSubscriber(mqttCfg, log, ingestHooks, limits)
	if err != nil {
		log.Fatal("mqtt subscriber", zap.Error(err))
	}

	if err := sub.Run(ctx, pipeline); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal("mqtt-ingest stopped with error", zap.Error(err))
	}

	log.Info("mqtt-ingest stopped cleanly")
}
