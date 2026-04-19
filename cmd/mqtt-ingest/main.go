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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log, err := observability.NewLogger(cfg)
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	mqttCfg := platformmqtt.LoadBrokerFromEnv()
	if err := mqttCfg.Validate(); err != nil {
		log.Fatal("mqtt config", zap.Error(err))
	}

	var ingestHooks *platformmqtt.IngestHooks
	var metricsSrv *http.Server
	if cfg.MetricsEnabled {
		ingestHooks = telemetryapp.NewIngestHooks()
		addr := strings.TrimSpace(cfg.MQTTIngestMetricsListen)
		if addr == "" {
			addr = "127.0.0.1:9093"
		}
		metricsSrv = &http.Server{
			Addr:    addr,
			Handler: promhttp.Handler(),
		}
		go func() {
			log.Info("mqtt_ingest_metrics_listen", zap.String("addr", addr), zap.String("path", "/metrics"))
			if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("mqtt-ingest metrics server exited", zap.Error(err))
			}
		}()
	}
	defer func() {
		if metricsSrv == nil {
			return
		}
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsSrv.Shutdown(sctx); err != nil {
			log.Warn("mqtt-ingest metrics shutdown", zap.Error(err))
		}
	}()

	natsURL := strings.TrimSpace(os.Getenv(platformnats.EnvNATSURL))
	var js natssrv.JetStreamContext
	if natsURL != "" {
		nc, err := platformnats.ConnectJetStream(ctx, natsURL, "avf-mqtt-ingest-telemetry")
		if err != nil {
			log.Fatal("nats connect", zap.Error(err))
		}
		defer func() { _ = nc.Conn.Drain() }()
		js = nc.JS
		if err := platformnats.EnsureTelemetryStreams(js, cfg.TelemetryJetStream.NATSBrokerLimits()); err != nil {
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
