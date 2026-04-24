package observability

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// ReadyFunc is a lightweight readiness probe for auxiliary process HTTP servers.
type ReadyFunc func(context.Context) error

// NewOperationsMux builds a conservative loopback-safe ops surface: liveness,
// readiness, and optional metrics.
func NewOperationsMux(cfg *config.Config, log *zap.Logger, metricsEnabled bool, ready ReadyFunc) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		WriteVersionJSON(w, cfg)
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if ready == nil {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		timeout := 2 * time.Second
		if cfg != nil && cfg.Ops.ReadinessTimeout > 0 {
			timeout = cfg.Ops.ReadinessTimeout
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		if err := ready(ctx); err != nil {
			EnrichLogger(log, r.Context()).Warn("readiness failed", zap.Error(err))
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	if metricsEnabled {
		h := promhttp.Handler()
		if cfg != nil {
			if tok := strings.TrimSpace(cfg.MetricsScrapeToken); tok != "" {
				h = ScrapeBearerGate(tok, h)
			}
		}
		mux.Handle("/metrics", h)
	}
	return mux
}

// ShutdownHTTPServer gracefully shuts down an auxiliary HTTP server.
func ShutdownHTTPServer(log *zap.Logger, server *http.Server, timeout time.Duration, message string) {
	if server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		EnrichLogger(log, ctx).Warn(message, zap.Error(err))
	}
}
