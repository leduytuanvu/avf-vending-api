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

	deps, cleanup, err := bootstrap.BuildReconcilerDeps(ctx, cfg, pool, log)
	if err != nil {
		log.Fatal("reconciler bootstrap", zap.Error(err))
	}
	defer cleanup()

	if cfg.MetricsEnabled {
		deps.Telemetry = reconcilerprom.New()
	}

	var metricsSrv *http.Server
	if cfg.MetricsEnabled {
		addr := strings.TrimSpace(cfg.ReconcilerMetricsListen)
		if addr == "" {
			addr = "127.0.0.1:9092"
		}
		metricsSrv = &http.Server{
			Addr:    addr,
			Handler: promhttp.Handler(),
		}
		go func() {
			log.Info("reconciler_metrics_listen", zap.String("addr", addr), zap.String("path", "/metrics"))
			if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("reconciler metrics server exited", zap.Error(err))
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
			log.Warn("reconciler metrics shutdown", zap.Error(err))
		}
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
