// @title AVF Vending HTTP API
// @version 1.0
// @description HTTP API for the AVF vending platform (cmd/api). All `/v1/*` routes require `Authorization: Bearer <JWT>` unless noted (health, optional metrics, Swagger when enabled). Bearer middleware failures use minimal JSON `{"error":{"message":"..."}}` without `error.code`. Most `/v1` handlers use `{"error":{"code":"...","message":"..."}}` (see writeAPIError). **501** list stubs return `not_implemented` plus `capability` and `implemented:false`. **503** `capability_not_configured` is used when optional wiring (MQTT dispatch, commerce persistence, outbox defaults) is missing. Request tracing: responses echo `X-Request-ID` and `X-Correlation-ID` when middleware is enabled.
// @termsOfService https://github.com/avf/avf-vending-system/tree/main/avf-vending-api

// @contact.name AVF Engineering
// @contact.url https://github.com/avf/avf-vending-system

// @license.name License terms apply per your deployment agreement; see repository NOTICE if present.

// @host localhost:8080
// @BasePath /
// @schemes http https
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/avf/avf-vending-api/internal/bootstrap"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Best-effort local developer ergonomics; production should inject env via orchestrator.
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

	if err := bootstrap.RunAPI(ctx, cfg, log); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal("api stopped with error", zap.Error(err))
	}

	log.Info("api stopped cleanly")
}
