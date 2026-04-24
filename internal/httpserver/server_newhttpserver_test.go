package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type stubReadinessProbe struct{}

func (stubReadinessProbe) Ready(context.Context) error { return nil }

func testHTTPServerConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		AppEnv:           config.AppEnvDevelopment,
		ProcessName:      "api",
		LogLevel:         "info",
		LogFormat:        "json",
		SwaggerUIEnabled: true,
		MetricsEnabled:   false,
		Runtime: config.RuntimeConfig{
			NodeName:    "node-a",
			InstanceID:  "node-a-api-1",
			RuntimeRole: "api",
		},
		Build: config.BuildConfig{
			Version: "dev",
		},
		HTTPAuth: config.HTTPAuthConfig{
			Mode:            "hs256",
			JWTSecret:       bytes.Repeat([]byte("x"), 32),
			JWTLeeway:       45 * time.Second,
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 720 * time.Hour,
		},
		HTTP: config.HTTPConfig{
			Addr:              ":0",
			ShutdownTimeout:   15 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		Ops: config.OperationsConfig{
			ReadinessTimeout:      2 * time.Second,
			ShutdownTimeout:       5 * time.Second,
			TracerShutdownTimeout: 10 * time.Second,
		},
	}
}

func TestNewHTTPServer_noPanic_healthAndSwagger(t *testing.T) {
	t.Parallel()
	cfg := testHTTPServerConfig(t)
	hs, err := NewHTTPServer(cfg, zap.NewNop(), stubReadinessProbe{}, &api.HTTPApplication{})
	if err != nil {
		t.Fatal(err)
	}
	h := hs.srv.Handler

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("GET /health/live: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /swagger/index.html: status=%d", rec2.Code)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK || !strings.Contains(rec3.Body.String(), `"version":"dev"`) {
		t.Fatalf("GET /version: status=%d body=%q", rec3.Code, rec3.Body.String())
	}
}

func TestNewHTTPServer_production_swaggerDisabled_noSwaggerRoutes(t *testing.T) {
	t.Parallel()
	cfg := testHTTPServerConfig(t)
	cfg.AppEnv = config.AppEnvProduction
	cfg.SwaggerUIEnabled = false
	hs, err := NewHTTPServer(cfg, zap.NewNop(), stubReadinessProbe{}, &api.HTTPApplication{})
	if err != nil {
		t.Fatal(err)
	}
	h := hs.srv.Handler
	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /swagger/index.html: status=%d want 404", rec.Code)
	}
}

func TestNewHTTPServer_production_metrics_opsOnly_requiresHTTPOpsAddr(t *testing.T) {
	t.Parallel()
	cfg := testHTTPServerConfig(t)
	cfg.AppEnv = config.AppEnvProduction
	cfg.MetricsEnabled = true
	cfg.MetricsExposeOnPublicHTTP = false
	cfg.Ops.HTTPAddr = ""
	_, err := NewHTTPServer(cfg, zap.NewNop(), stubReadinessProbe{}, &api.HTTPApplication{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP_OPS_ADDR")
}

func TestNewHTTPServer_production_metrics_opsOnly_publicListenerNoMetrics(t *testing.T) {
	t.Parallel()
	cfg := testHTTPServerConfig(t)
	cfg.AppEnv = config.AppEnvProduction
	cfg.MetricsEnabled = true
	cfg.MetricsExposeOnPublicHTTP = false
	cfg.Ops.HTTPAddr = "127.0.0.1:8081"
	hs, err := NewHTTPServer(cfg, zap.NewNop(), stubReadinessProbe{}, &api.HTTPApplication{})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	hs.srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.NotNil(t, hs.ops)
	rec2 := httptest.NewRecorder()
	hs.ops.Handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec2.Code)
}

func TestNewHTTPServer_production_metrics_opsListenerRequiresBearerWhenTokenSet(t *testing.T) {
	t.Parallel()
	cfg := testHTTPServerConfig(t)
	cfg.AppEnv = config.AppEnvProduction
	cfg.MetricsEnabled = true
	cfg.MetricsExposeOnPublicHTTP = false
	cfg.MetricsScrapeToken = "ops-metrics-token-16b"
	cfg.Ops.HTTPAddr = "127.0.0.1:8081"
	hs, err := NewHTTPServer(cfg, zap.NewNop(), stubReadinessProbe{}, &api.HTTPApplication{})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	hs.ops.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer ops-metrics-token-16b")
	rec2 := httptest.NewRecorder()
	hs.ops.Handler.ServeHTTP(rec2, req)
	require.Equal(t, http.StatusOK, rec2.Code)

	reqLive := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec3 := httptest.NewRecorder()
	hs.ops.Handler.ServeHTTP(rec3, reqLive)
	require.Equal(t, http.StatusOK, rec3.Code)
}

func TestNewHTTPServer_production_metrics_publicRequiresBearer(t *testing.T) {
	t.Parallel()
	cfg := testHTTPServerConfig(t)
	cfg.AppEnv = config.AppEnvProduction
	cfg.MetricsEnabled = true
	cfg.MetricsExposeOnPublicHTTP = true
	cfg.MetricsScrapeToken = "test-metrics-token-16b"
	cfg.Ops.HTTPAddr = "127.0.0.1:8081"
	hs, err := NewHTTPServer(cfg, zap.NewNop(), stubReadinessProbe{}, &api.HTTPApplication{})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	hs.srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer test-metrics-token-16b")
	rec2 := httptest.NewRecorder()
	hs.srv.Handler.ServeHTTP(rec2, req)
	require.Equal(t, http.StatusOK, rec2.Code)
}

func TestNewHTTPServer_development_metricsOnPublicWithoutToken(t *testing.T) {
	t.Parallel()
	cfg := testHTTPServerConfig(t)
	cfg.MetricsEnabled = true
	hs, err := NewHTTPServer(cfg, zap.NewNop(), stubReadinessProbe{}, &api.HTTPApplication{})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	hs.srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestNewHTTPServer_production_swaggerEnabled_servesSwagger(t *testing.T) {
	t.Parallel()
	cfg := testHTTPServerConfig(t)
	cfg.AppEnv = config.AppEnvProduction
	cfg.SwaggerUIEnabled = true
	hs, err := NewHTTPServer(cfg, zap.NewNop(), stubReadinessProbe{}, &api.HTTPApplication{})
	if err != nil {
		t.Fatal(err)
	}
	h := hs.srv.Handler
	for _, path := range []string{"/swagger/index.html", "/swagger/doc.json"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status=%d", path, rec.Code)
		}
	}
}

func TestMountV1_v1AuthRoutesNotDuplicated_chiWalk(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	app := &api.HTTPApplication{}
	cfg := &config.Config{}
	writeRL := func(h http.Handler) http.Handler { return h }
	mountV1(r, app, zap.NewNop(), cfg, stubAccessTokenValidator{}, writeRL)

	// With nil Auth, mountAuthRoutes registers nothing; duplicate /auth would still panic at mountV1.
	seen := map[string]int{}
	_ = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, "/v1/auth") {
			k := method + " " + route
			seen[k]++
		}
		return nil
	})
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("expected exactly one registration for %q, got %d", k, n)
		}
	}
}
