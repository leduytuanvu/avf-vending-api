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
	"github.com/avf/avf-vending-api/internal/bootstrap"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/avf/avf-vending-api/internal/observability/reconcilerprom"
	platformdb "github.com/avf/avf-vending-api/internal/platform/db"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	cfg.ProcessName = "reconciler"

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
		log.Fatal("reconciler requires DATABASE_URL for persistence")
	}

	if err := config.ValidateReconciler(&cfg.Reconciler); err != nil {
		log.Fatal("reconciler config invalid", zap.Error(err))
	}

	dbCtx, cancelDB := context.WithTimeout(ctx, 30*time.Second)
	pool, err := platformdb.NewPool(dbCtx, &cfg.Postgres)
	cancelDB()
	if err != nil {
		log.Fatal("postgres pool", zap.Error(err))
	}
	defer pool.Close()

	workflowBoundary, workflowCleanup, err := bootstrap.BuildWorkflowOrchestration(ctx, cfg)
	if err != nil {
		log.Fatal("workflow orchestration bootstrap", zap.Error(err))
	}
	defer workflowCleanup()

	deps, cleanup, err := bootstrap.BuildReconcilerDeps(ctx, cfg, pool, log)
	if err != nil {
		log.Fatal("reconciler bootstrap", zap.Error(err))
	}
	defer cleanup()
	deps.WorkflowOrchestration = workflowBoundary
	deps.ScheduleRefundOrchestration = cfg.Temporal.ScheduleRefundOrchestration
	deps.ScheduleManualReviewEscalation = cfg.Temporal.ScheduleManualReviewEscalation

	if cfg.MetricsEnabled {
		deps.Telemetry = reconcilerprom.New()
	}

	addr := strings.TrimSpace(cfg.ReconcilerMetricsListen)
	if addr == "" {
		addr = "127.0.0.1:9092"
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
		log.Info("reconciler_ops_listen", zap.String("addr", addr), zap.Strings("paths", paths))
		if err := opsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("reconciler ops server exited", zap.Error(err))
		}
	}()
	defer func() {
		observability.ShutdownHTTPServer(log, opsSrv, cfg.Ops.ShutdownTimeout, "reconciler ops shutdown")
	}()

	if cfg.Reconciler.ActionsEnabled {
		log.Info("reconciler_actions_enabled",
			zap.Bool("dry_run", cfg.Reconciler.DryRun),
			zap.String("note", "PSP probe and refund routing are active; dry_run skips payment mutations and refund/duplicate NATS publishes"),
		)
	} else {
		log.Info("reconciler_actions_disabled",
			zap.String("note", "list-only reconciliation ticks; set RECONCILER_ACTIONS_ENABLED=true after configuring probe URL and NATS"),
		)
	}

	log.Info("reconciler_process_bootstrap",
		zap.String("signal", "SIGINT/SIGTERM for graceful shutdown"),
		zap.Int32("batch_limits", deps.Limits),
		zap.Duration("stable_age", deps.StableAge),
	)

	if err := appbackground.RunReconciler(ctx, deps); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal("reconciler stopped with error", zap.Error(err))
	}

	log.Info("reconciler stopped cleanly")
}
