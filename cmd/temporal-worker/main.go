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

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/bootstrap"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/observability"
	platformdb "github.com/avf/avf-vending-api/internal/platform/db"
	platformtemporal "github.com/avf/avf-vending-api/internal/platform/temporal"
	"github.com/joho/godotenv"
	sdkworker "go.temporal.io/sdk/worker"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	cfg.ProcessName = "temporal-worker"

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

	if !cfg.Temporal.Enabled {
		log.Fatal("temporal worker requires TEMPORAL_ENABLED=true")
	}
	if cfg.Postgres.URL == "" {
		log.Fatal("temporal worker requires DATABASE_URL for activity state checks")
	}

	dbCtx, cancelDB := context.WithTimeout(ctx, 30*time.Second)
	pool, err := platformdb.NewPool(dbCtx, &cfg.Postgres)
	cancelDB()
	if err != nil {
		log.Fatal("postgres pool", zap.Error(err))
	}
	defer pool.Close()

	activityDeps, activityCleanup, err := bootstrap.BuildTemporalWorkerActivityDeps(ctx, cfg, pool, log)
	if err != nil {
		log.Fatal("temporal worker activity bootstrap", zap.Error(err))
	}
	defer activityCleanup()

	tc, err := platformtemporal.Dial(platformtemporal.DialOptions{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
	})
	if err != nil {
		log.Fatal("temporal dial", zap.Error(err))
	}
	defer tc.Close()

	var registerErr error
	w, err := platformtemporal.NewWorker(tc, cfg.Temporal.TaskQueue, sdkworker.Options{}, func(w sdkworker.Worker) {
		registerErr = workfloworch.RegisterAll(w, activityDeps)
	})
	if err != nil {
		log.Fatal("temporal worker construct", zap.Error(err))
	}
	if registerErr != nil {
		log.Fatal("temporal worker register", zap.Error(registerErr))
	}

	addr := strings.TrimSpace(cfg.TemporalWorkerMetricsListen)
	if addr == "" {
		addr = "127.0.0.1:9094"
	}
	opsSrv := &http.Server{
		Addr: addr,
		Handler: observability.NewOperationsMux(cfg, log, cfg.MetricsEnabled, func(ctx context.Context) error {
			pctx, cancel := context.WithTimeout(ctx, cfg.Ops.ReadinessTimeout)
			defer cancel()
			return pool.Ping(pctx)
		}),
	}
	go func() {
		paths := []string{"/health/live", "/health/ready"}
		if cfg.MetricsEnabled {
			paths = append([]string{"/metrics"}, paths...)
		}
		log.Info("temporal_worker_ops_listen", zap.String("addr", addr), zap.Strings("paths", paths))
		if err := opsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("temporal worker ops server exited", zap.Error(err))
		}
	}()
	defer func() {
		observability.ShutdownHTTPServer(log, opsSrv, cfg.Ops.ShutdownTimeout, "temporal worker ops shutdown")
	}()

	log.Info("temporal_worker_bootstrap",
		zap.String("task_queue", cfg.Temporal.TaskQueue),
		zap.String("namespace", cfg.Temporal.Namespace),
	)

	go func() {
		<-ctx.Done()
		w.Stop()
	}()
	if err := platformtemporal.RunInteractive(w); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal("temporal worker stopped with error", zap.Error(err))
	}
	log.Info("temporal worker stopped cleanly")
}
