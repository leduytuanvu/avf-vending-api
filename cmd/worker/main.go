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

	appbackground "github.com/avf/avf-vending-api/internal/app/background"
	"github.com/avf/avf-vending-api/internal/app/telemetryapp"
	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/observability"
	platformclickhouse "github.com/avf/avf-vending-api/internal/platform/clickhouse"
	platformdb "github.com/avf/avf-vending-api/internal/platform/db"
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
		log.Fatal("worker requires DATABASE_URL for persistence")
	}

	dbCtx, cancelDB := context.WithTimeout(ctx, 30*time.Second)
	pool, err := platformdb.NewPool(dbCtx, &cfg.Postgres)
	cancelDB()
	if err != nil {
		log.Fatal("postgres pool", zap.Error(err))
	}
	defer pool.Close()

	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{})
	if err := appreliability.ValidateRecoveryPolicy(policy); err != nil {
		log.Fatal("recovery policy", zap.Error(err))
	}

	outboxRepo := postgres.NewOutboxRepository(pool)
	scanRepo := postgres.NewRecoveryScanRepository(pool)
	store := postgres.NewStore(pool)
	relSvc := appreliability.NewService(appreliability.Deps{
		Payments: scanRepo,
		Commands: scanRepo,
		Vends:    scanRepo,
		Outbox:   outboxRepo,
	})

	var outboxPub domaincommerce.OutboxPublisher
	var outboxDLQ appbackground.OutboxDeadLetterPublisher
	var telemetryWorkers *telemetryapp.JetStreamWorkers
	var telemetryJS natssrv.JetStreamContext
	if natsURL := strings.TrimSpace(os.Getenv(platformnats.EnvNATSURL)); natsURL != "" {
		nc, err := platformnats.ConnectJetStream(ctx, natsURL, "avf-worker-outbox")
		if err != nil {
			log.Fatal("nats connect", zap.Error(err), zap.String("url", natsURL))
		}
		defer func() { _ = nc.Conn.Drain() }()
		if err := platformnats.EnsureInternalStreams(nc.JS); err != nil {
			log.Fatal("nats streams", zap.Error(err))
		}
		jsLim := cfg.TelemetryJetStream.NATSBrokerLimits()
		if err := platformnats.EnsureTelemetryStreams(nc.JS, jsLim); err != nil {
			log.Fatal("nats telemetry streams", zap.Error(err))
		}
		if err := platformnats.EnsureTelemetryDurableConsumers(nc.JS, jsLim); err != nil {
			log.Fatal("nats telemetry consumers", zap.Error(err))
		}
		outboxPub = platformnats.NewJetStreamOutboxPublisher(nc.JS)
		outboxDLQ = platformnats.NewOutboxDeadLetterJetStream(nc.JS)
		log.Info("outbox jetstream publisher enabled",
			zap.String("stream_outbox", platformnats.StreamOutbox),
			zap.String("stream_dlq", platformnats.StreamDLQ),
		)
		tw := telemetryapp.NewJetStreamWorkers(telemetryapp.JetStreamWorkersConfig{
			Log:         log,
			NC:          nc.Conn,
			JS:          nc.JS,
			Store:       store,
			Telemetry:   cfg.TelemetryJetStream,
			Limits:      jsLim,
		})
		telemetryWorkers = tw
		telemetryJS = nc.JS
		go func() {
			if err := tw.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("telemetry jetstream workers exited", zap.Error(err))
			}
		}()
	}

	var metricsSrv *http.Server
	if cfg.MetricsEnabled {
		addr := strings.TrimSpace(cfg.WorkerMetricsListen)
		if addr == "" {
			addr = "127.0.0.1:9091"
		}
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if telemetryWorkers == nil || telemetryJS == nil {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
				return
			}
			if err := telemetryWorkers.Ready(r.Context(), telemetryJS); err != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		metricsSrv = &http.Server{
			Addr:    addr,
			Handler: mux,
		}
		go func() {
			log.Info("worker_metrics_listen", zap.String("addr", addr), zap.Strings("paths", []string{"/metrics", "/health/live", "/health/ready"}))
			if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("worker metrics server exited", zap.Error(err))
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
			log.Warn("worker metrics shutdown", zap.Error(err))
		}
	}()

	ob, pay, cmd, _ := appbackground.DefaultWorkerTickSchedule()
	retention := 24 * time.Hour
	deps := appbackground.WorkerDeps{
		Log:                    log,
		Reliability:            relSvc,
		Policy:                 policy,
		Limits:                 appreliability.ScanLimits{MaxItems: 200},
		OutboxList:             outboxRepo,
		OutboxMark:             outboxRepo,
		OutboxPub:              outboxPub,
		OutboxDeadLetter:       outboxDLQ,
		OutboxTick:             ob,
		PaymentTimeoutTick:     pay,
		StuckCommandTick:       cmd,
		RetentionTick:          retention,
		TelemetryRetention: func(c context.Context) error {
			return postgres.RunTelemetryRetention(c, pool, time.Now().UTC())
		},
		MQTTCommandAckTimeouts: store.ApplyMQTTCommandAckTimeouts,
	}

	var mirrorSink *platformclickhouse.AsyncOutboxMirrorSink
	if cfg.Analytics.ClickHouseEnabled {
		pingCtx, pingCancel := context.WithTimeout(ctx, 15*time.Second)
		chc, chErr := platformclickhouse.Open(pingCtx, platformclickhouse.Config{
			Enabled:      true,
			HTTPEndpoint: cfg.Analytics.ClickHouseHTTPURL,
		})
		pingCancel()
		if chErr != nil {
			log.Fatal("analytics clickhouse", zap.Error(chErr))
		}
		defer func() { _ = chc.Close() }()
		if cfg.Analytics.MirrorOutboxPublished {
			var sErr error
			mirrorSink, sErr = platformclickhouse.NewAsyncOutboxMirrorSink(
				log,
				chc,
				cfg.Analytics.MirrorTable,
				cfg.Analytics.MirrorMaxConcurrent,
				cfg.Analytics.InsertTimeout,
				cfg.Analytics.InsertMaxAttempts,
			)
			if sErr != nil {
				log.Fatal("analytics mirror sink", zap.Error(sErr))
			}
			deps.OnOutboxPublishedMirror = mirrorSink.EnqueuePublished
		}
	}
	defer func() {
		if mirrorSink != nil {
			mirrorSink.Shutdown()
		}
	}()

	log.Info("worker_process_bootstrap",
		zap.String("signal", "SIGINT/SIGTERM for graceful shutdown"),
		zap.Int32("recovery_scan_max_items", deps.Limits.MaxItems),
	)

	if err := appbackground.RunWorker(ctx, deps); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal("worker stopped with error", zap.Error(err))
	}

	log.Info("worker stopped cleanly")
}
