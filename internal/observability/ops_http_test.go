package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"go.uber.org/zap"
)

func TestNewOperationsMux_LiveAndReady(t *testing.T) {
	t.Parallel()

	mux := NewOperationsMux(&config.Config{}, zap.NewNop(), false, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("live status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status=%d", rec.Code)
	}
}

func TestNewOperationsMux_ReadinessFailure(t *testing.T) {
	t.Parallel()

	mux := NewOperationsMux(&config.Config{}, zap.NewNop(), false, func(_ context.Context) error {
		return errors.New("boom")
	})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status=%d", rec.Code)
	}
}

func TestNewOperationsMux_MetricsOptional(t *testing.T) {
	t.Parallel()

	disabled := NewOperationsMux(&config.Config{}, zap.NewNop(), false, nil)
	rec := httptest.NewRecorder()
	disabled.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected metrics 404 when disabled, got %d", rec.Code)
	}

	enabled := NewOperationsMux(&config.Config{}, zap.NewNop(), true, nil)
	rec = httptest.NewRecorder()
	enabled.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected metrics 200 when enabled, got %d", rec.Code)
	}
}

func TestNewOperationsMux_Version(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		AppEnv:      config.AppEnvProduction,
		ProcessName: "worker",
		Runtime: config.RuntimeConfig{
			Region:      "ap-southeast-1",
			NodeName:    "node-a",
			InstanceID:  "node-a-worker-1",
			RuntimeRole: "worker",
		},
		Build: config.BuildConfig{
			Version: "1.2.3",
			GitSHA:  "abc123",
		},
	}
	mux := NewOperationsMux(cfg, zap.NewNop(), false, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("version status=%d", rec.Code)
	}
	if body := rec.Body.String(); body == "" || !containsAll(body, "1.2.3", "abc123", "worker", "ap-southeast-1") {
		t.Fatalf("unexpected version body: %q", body)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
