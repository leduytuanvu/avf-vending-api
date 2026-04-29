package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	appbackground "github.com/avf/avf-vending-api/internal/app/background"
	appreliability "github.com/avf/avf-vending-api/internal/app/reliability"
	"github.com/avf/avf-vending-api/internal/app/telemetryapp"
	"github.com/avf/avf-vending-api/internal/bootstrap"
	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/observability"
	platformclickhouse "github.com/avf/avf-vending-api/internal/platform/clickhouse"
	platformdb "github.com/avf/avf-vending-api/internal/platform/db"
	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	platformredis "github.com/avf/avf-vending-api/internal/platform/redis"
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
	if pn := strings.TrimSpace(os.Getenv("PROCESS_NAME")); pn != "" {
		cfg.ProcessName = pn
	} else {
		cfg.ProcessName = "worker"
	}

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
		log.Fatal("worker requires DATABASE_URL for persistence")
	}

	dbCtx, cancelDB := context.WithTimeout(ctx, 30*time.Second)
	pool, err := platformdb.NewPool(dbCtx, &cfg.Postgres)
	cancelDB()
	if err != nil {
		log.Fatal("postgres pool", zap.Error(err))
	}
	defer pool.Close()

	rdb, err := platformredis.NewClient(&cfg.Redis)
	if err != nil {
		log.Fatal("redis client", zap.Error(err))
	}
	var workerLocker platformredis.Locker
	if rdb != nil {
		defer func() { _ = rdb.Close() }()
		if cfg.RedisRuntime.LocksEnabled {
			workerLocker = platformredis.NewRedisLocker(rdb, cfg.Redis.KeyPrefix)
		}
	}

	policy := appreliability.NormalizeRecoveryPolicy(appreliability.RecoveryPolicy{
		OutboxMaxPublishAttempts: cfg.Outbox.MaxAttempts,
		OutboxPublishBackoffBase: cfg.Outbox.BackoffMin,
		OutboxPublishBackoffMax:  cfg.Outbox.BackoffMax,
	})
	if err := appreliability.ValidateRecoveryPolicy(policy); err != nil {
		log.Fatal("recovery policy", zap.Error(err))
	}

	workflowBoundary, workflowCleanup, err := bootstrap.BuildWorkflowOrchestration(ctx, cfg)
	if err != nil {
		log.Fatal("workflow orchestration bootstrap", zap.Error(err))
	}
	defer workflowCleanup()

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
	if natsURL := strings.TrimSpace(cfg.NATS.URL); natsURL != "" {
		nc, err := platformnats.ConnectJetStream(ctx, natsURL, "avf-worker-outbox")
		if err != nil {
			log.Fatal("nats connect", zap.Error(err), zap.String("url", natsURL))
		}
		defer func() { _ = nc.Conn.Drain() }()
		if err := platformnats.EnsureInternalStreams(nc.JS); err != nil {
			log.Fatal("nats streams", zap.Error(err))
		}
		jsLim := cfg.TelemetryJetStream.NATSBrokerLimits()
		platformnats.LogTelemetryJetStreamRetention(log, "worker", string(cfg.AppEnv), jsLim)
		if err := platformnats.EnsureTelemetryStreams(nc.JS, jsLim); err != nil {
			log.Fatal("nats telemetry streams", zap.Error(err))
		}
		if err := platformnats.EnsureTelemetryDurableConsumers(nc.JS, jsLim); err != nil {
			log.Fatal("nats telemetry consumers", zap.Error(err))
		}
		outboxPub = platformnats.NewJetStreamOutboxPublisher(nc.JS)
		if cfg.Outbox.DLQEnabled {
			outboxDLQ = platformnats.NewOutboxDeadLetterJetStream(nc.JS)
		}
		log.Info("outbox jetstream publisher enabled",
			zap.String("stream_outbox", platformnats.StreamOutbox),
			zap.String("stream_dlq", platformnats.StreamDLQ),
			zap.Bool("dlq_enabled", cfg.Outbox.DLQEnabled),
		)
		tw := telemetryapp.NewJetStreamWorkers(telemetryapp.JetStreamWorkersConfig{
			Log:       log,
			NC:        nc.Conn,
			JS:        nc.JS,
			Store:     store,
			Telemetry: cfg.TelemetryJetStream,
			Limits:    jsLim,
		})
		telemetryWorkers = tw
		telemetryJS = nc.JS
		go func() {
			if err := tw.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("telemetry jetstream workers exited", zap.Error(err))
			}
		}()
	}
	if cfg.Outbox.PublisherRequired && outboxPub == nil {
		log.Fatal("outbox publisher required but not configured",
			zap.Bool("outbox_publisher_required", cfg.Outbox.PublisherRequired),
			zap.Bool("nats_required", cfg.NATS.Required),
			zap.String("nats_url", strings.TrimSpace(cfg.NATS.URL)),
		)
	}

	addr := strings.TrimSpace(cfg.WorkerMetricsListen)
	if addr == "" {
		addr = "127.0.0.1:9091"
	}
	opsSrv := &http.Server{
		Addr: addr,
		Handler: observability.NewOperationsMux(cfg, log, cfg.MetricsEnabled, func(ctx context.Context) error {
			pctx, cancel := context.WithTimeout(ctx, cfg.Ops.ReadinessTimeout)
			defer cancel()
			if err := pool.Ping(pctx); err != nil {
				return err
			}
			if telemetryWorkers == nil || telemetryJS == nil {
				return nil
			}
			return telemetryWorkers.Ready(pctx, telemetryJS)
		}),
	}
	go func() {
		paths := []string{"/health/live", "/health/ready"}
		if cfg.MetricsEnabled {
			paths = append([]string{"/metrics"}, paths...)
		}
		log.Info("worker_ops_listen", zap.String("addr", addr), zap.Strings("paths", paths))
		if err := opsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("worker ops server exited", zap.Error(err))
		}
	}()
	defer func() {
		observability.ShutdownHTTPServer(log, opsSrv, cfg.Ops.ShutdownTimeout, "worker ops shutdown")
	}()

	ob := cfg.Capacity.WorkerTickOutbox
	pay := cfg.Capacity.WorkerTickPaymentTimeout
	cmd := cfg.Capacity.WorkerTickStuckCommand
	var retentionTick time.Duration
	var telemetryRetention func(context.Context) error
	var enterpriseRetention func(context.Context) error

	telCfg := cfg.TelemetryDataRetention
	telCfg.CleanupDryRun = config.EffectiveRetentionDryRun(cfg.RetentionWorker.GlobalDryRun, cfg.TelemetryDataRetention.CleanupDryRun)
	entCfg := cfg.EnterpriseRetention
	entCfg.CleanupDryRun = config.EffectiveRetentionDryRun(cfg.RetentionWorker.GlobalDryRun, cfg.EnterpriseRetention.CleanupDryRun)

	retentionWorkerOn := cfg.RetentionWorker.Enabled
	if retentionWorkerOn && cfg.TelemetryDataRetention.CleanupEnabled {
		retentionTick = 24 * time.Hour
		telemetryRetention = func(c context.Context) error {
			res, err := postgres.RunTelemetryRetention(c, pool, telCfg, time.Now().UTC())
			if err != nil {
				return err
			}
			log.Info("telemetry_retention_complete", zap.Bool("dry_run", res.DryRun), zap.Any("stages", res.Stages))
			return nil
		}
	}
	if retentionWorkerOn && cfg.EnterpriseRetention.CleanupEnabled {
		retentionTick = 24 * time.Hour
		enterpriseRetention = func(c context.Context) error {
			result, err := postgres.RunEnterpriseRetention(c, pool, entCfg, time.Now().UTC())
			if err != nil {
				return err
			}
			fields := []zap.Field{zap.Bool("dry_run", result.DryRun)}
			if result.DryRun {
				fields = append(fields, zap.Any("candidates", result.Candidates))
			} else {
				fields = append(fields, zap.Any("deleted", result.Deleted))
			}
			log.Info("enterprise_retention_complete", fields...)
			return nil
		}
	}
	outboxOnly := cfg.WorkerOutboxOnly || strings.EqualFold(strings.TrimSpace(cfg.ProcessName), "outbox")
	if outboxOnly {
		retentionTick = 0
		telemetryRetention = nil
		enterpriseRetention = nil
	}
	var obLease *appreliability.OutboxLeaseParams
	if !cfg.WorkerOutboxDisableLease {
		workerID := strings.TrimSpace(cfg.Runtime.InstanceID)
		if workerID == "" {
			workerID = strings.TrimSpace(cfg.Runtime.NodeName)
		}
		if workerID == "" {
			h, _ := os.Hostname()
			workerID = fmt.Sprintf("%s-%d", h, os.Getpid())
		}
		ltSec := cfg.WorkerOutboxLockTTLSeconds
		if ltSec <= 0 {
			ltSec = 45
		}
		obLease = &appreliability.OutboxLeaseParams{
			WorkerID: workerID,
			LockTTL:  time.Duration(ltSec) * time.Second,
		}
	}

	var outboxBatch int32
	if v := cfg.Capacity.WorkerOutboxDispatchMaxItems; v > 0 {
		outboxBatch = v
	}

	deps := appbackground.WorkerDeps{
		Log:                           log,
		Reliability:                   relSvc,
		Policy:                        policy,
		Limits:                        appreliability.ScanLimits{MaxItems: cfg.Capacity.WorkerRecoveryScanMaxItems},
		OutboxBatchMaxItems:           outboxBatch,
		OutboxList:                    outboxRepo,
		OutboxMark:                    outboxRepo,
		OutboxPub:                     outboxPub,
		OutboxDeadLetter:              outboxDLQ,
		OutboxLease:                   obLease,
		OutboxOnly:                    outboxOnly,
		OutboxTick:                    ob,
		PaymentTimeoutTick:            pay,
		StuckCommandTick:              cmd,
		CycleBackoffAfterFailure:      cfg.Capacity.WorkerCycleBackoffAfterFailure,
		RetentionTick:                 retentionTick,
		TelemetryRetention:            telemetryRetention,
		EnterpriseRetention:           enterpriseRetention,
		MQTTCommandAckTimeouts:        store.ApplyMQTTCommandAckTimeouts,
		WorkflowOrchestration:         workflowBoundary,
		SchedulePaymentPendingTimeout: cfg.Temporal.SchedulePaymentPendingTimeout,
		DistributedLocker:             workerLocker,
	}

	var mirrorSink *platformclickhouse.AsyncOutboxMirrorSink
	var projectionSink *platformclickhouse.AsyncProjectionSink
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
		}
		if cfg.Analytics.ProjectOutboxEvents {
			var sErr error
			projectionSink, sErr = platformclickhouse.NewAsyncProjectionSink(
				log,
				chc,
				cfg.Analytics.ProjectionTable,
				cfg.Analytics.MirrorMaxConcurrent,
				cfg.Analytics.InsertTimeout,
				cfg.Analytics.InsertMaxAttempts,
			)
			if sErr != nil {
				log.Fatal("analytics projection sink", zap.Error(sErr))
			}
		}
		if mirrorSink != nil || projectionSink != nil {
			deps.OnOutboxPublishedMirror = func(ev domaincommerce.OutboxEvent) {
				if mirrorSink != nil {
					mirrorSink.EnqueuePublished(ev)
				}
				if projectionSink != nil {
					projectionSink.EnqueuePublished(ev)
				}
			}
		}
	}
	defer func() {
		if mirrorSink != nil {
			mirrorSink.Shutdown()
		}
		if projectionSink != nil {
			projectionSink.Shutdown()
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
