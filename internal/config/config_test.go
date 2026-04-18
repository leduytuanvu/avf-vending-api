package config

import (
	"os"
	"strings"
	"testing"
)

func setMinimalValidLoadEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("HTTP_ADDR", ":0")
	t.Setenv("OTEL_SERVICE_NAME", "test")
}

func TestLoad_Defaults(t *testing.T) {
	setMinimalValidLoadEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Addr != ":0" {
		t.Fatalf("unexpected addr: %q", cfg.HTTP.Addr)
	}
}

func TestLoad_MissingLogLevelRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("LOG_LEVEL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for empty LOG_LEVEL")
	}
	if !strings.Contains(err.Error(), "LOG_LEVEL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_InvalidAppEnvRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("APP_ENV", "local-dev")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "APP_ENV") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_InvalidLogFormatRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("LOG_FORMAT", "yaml")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "LOG_FORMAT") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_InvalidHTTPAddrRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_ADDR", "not-a-host:port")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP_ADDR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_GRPCEnabledRequiresAddr(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("GRPC_ENABLED", "true")
	t.Setenv("GRPC_ADDR", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GRPC_ADDR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_RedisPasswordWithoutAddrRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_ADDR", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "REDIS_PASSWORD") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_PostgresPoolMinGreaterThanMaxRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("DATABASE_MAX_CONNS", "2")
	t.Setenv("DATABASE_MIN_CONNS", "5")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "DATABASE_MIN_CONNS") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_ProductionAppEnvStillValidatedStrictly(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("OTEL_SERVICE_NAME", "avf-api-prod")
	t.Setenv("HTTP_AUTH_JWT_SECRET", "unit-test-production-hs256-secret-key-material")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppEnv != AppEnvProduction {
		t.Fatalf("app env: %q", cfg.AppEnv)
	}
}

func TestValidate_OTELConflict(t *testing.T) {
	cfg := &Config{
		AppEnv:    AppEnvDevelopment,
		LogLevel:  "info",
		LogFormat: "json",
		HTTP: HTTPConfig{
			Addr:              ":8080",
			ShutdownTimeout:   1,
			ReadHeaderTimeout: 1,
			ReadTimeout:       1,
			WriteTimeout:      1,
			IdleTimeout:       1,
		},
		Telemetry: TelemetryConfig{
			ServiceName:  "svc",
			OTLPEndpoint: "localhost:4317",
			SDKDisabled:  true,
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidate_PostgresRequiresPositiveMaxConnsWhenURLSet(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("DATABASE_MAX_CONNS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidate_ReadinessStrictWithNoDepsUsesSeparateFlow(t *testing.T) {
	// READINESS_STRICT is runtime behavior; config should still load.
	setMinimalValidLoadEnv(t)
	t.Setenv("READINESS_STRICT", "true")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ReadinessStrict {
		t.Fatal("expected strict readiness")
	}
}

func TestLoad_HTTPAuth_rs256PEM_requiresKeyMaterial(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_AUTH_MODE", "rs256_pem")
	t.Setenv("HTTP_AUTH_JWT_RSA_PUBLIC_KEY", "")
	t.Setenv("HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rs256_pem") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_HTTPAuth_invalidMode(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_AUTH_MODE", "not_a_mode")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP_AUTH_MODE") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_HTTPRateLimit_enabledInvalidBurst(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_RATE_LIMIT_SENSITIVE_WRITES_ENABLED", "true")
	t.Setenv("HTTP_RATE_LIMIT_SENSITIVE_WRITES_RPS", "10")
	t.Setenv("HTTP_RATE_LIMIT_SENSITIVE_WRITES_BURST", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_TemporalDisabledByDefault(t *testing.T) {
	setMinimalValidLoadEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Temporal.Enabled {
		t.Fatal("expected TEMPORAL_ENABLED=false by default")
	}
}

func TestLoad_AnalyticsClickHouseDisabledByDefault(t *testing.T) {
	setMinimalValidLoadEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Analytics.ClickHouseEnabled {
		t.Fatal("expected ANALYTICS_CLICKHOUSE_ENABLED=false by default")
	}
	if cfg.Analytics.MirrorOutboxPublished {
		t.Fatal("expected ANALYTICS_MIRROR_OUTBOX_PUBLISHED=false by default")
	}
}

func TestLoad_AnalyticsMirrorRequiresClickHouse(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("ANALYTICS_MIRROR_OUTBOX_PUBLISHED", "true")
	t.Setenv("ANALYTICS_CLICKHOUSE_ENABLED", "false")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ANALYTICS_MIRROR_OUTBOX_PUBLISHED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_AnalyticsClickHouseEnabledRequiresHTTPURL(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("ANALYTICS_CLICKHOUSE_ENABLED", "true")
	t.Setenv("ANALYTICS_CLICKHOUSE_HTTP_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ANALYTICS_CLICKHOUSE_HTTP_URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_AnalyticsClickHouseBadTableName(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("ANALYTICS_CLICKHOUSE_ENABLED", "true")
	t.Setenv("ANALYTICS_CLICKHOUSE_HTTP_URL", "http://localhost:8123/avf")
	t.Setenv("ANALYTICS_CLICKHOUSE_TABLE", "bad-name")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ANALYTICS_CLICKHOUSE_TABLE") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_TemporalEnabled_requiresHostPort(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("TEMPORAL_ENABLED", "true")
	t.Setenv("TEMPORAL_TASK_QUEUE", "avf-workflows")
	t.Setenv("TEMPORAL_HOST_PORT", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "TEMPORAL_HOST_PORT") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_TemporalEnabled_requiresTaskQueue(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("TEMPORAL_ENABLED", "true")
	t.Setenv("TEMPORAL_HOST_PORT", "127.0.0.1:7233")
	t.Setenv("TEMPORAL_TASK_QUEUE", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "TEMPORAL_TASK_QUEUE") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_ArtifactsEnabled_invalidMaxUpload(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("API_ARTIFACTS_ENABLED", "true")
	t.Setenv("ARTIFACTS_MAX_UPLOAD_BYTES", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Production_requiresHS256Secret(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_AUTH_JWT_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP_AUTH_JWT_SECRET") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMain(m *testing.M) {
	// Isolate tests from developer machine env.
	for _, k := range []string{
		"APP_ENV", "LOG_LEVEL", "LOG_FORMAT",
		"HTTP_ADDR", "HTTP_SHUTDOWN_TIMEOUT", "HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT",
		"HTTP_SWAGGER_UI_ENABLED",
		"GRPC_ENABLED", "GRPC_ADDR", "GRPC_SHUTDOWN_TIMEOUT",
		"DATABASE_URL", "DATABASE_MAX_CONNS", "DATABASE_MIN_CONNS", "DATABASE_MAX_CONN_IDLE_TIME", "DATABASE_MAX_CONN_LIFETIME",
		"REDIS_ADDR", "REDIS_PASSWORD", "REDIS_DB",
		"READINESS_STRICT", "METRICS_ENABLED", "WORKER_METRICS_LISTEN", "RECONCILER_METRICS_LISTEN", "MQTT_INGEST_METRICS_LISTEN",
		"OTEL_SERVICE_NAME", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_INSECURE", "OTEL_SDK_DISABLED",
		"HTTP_AUTH_MODE", "HTTP_AUTH_JWT_SECRET", "HTTP_AUTH_JWT_SECRET_PREVIOUS",
		"HTTP_AUTH_JWT_LEEWAY", "HTTP_AUTH_JWT_RSA_PUBLIC_KEY", "HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE",
		"HTTP_AUTH_JWT_JWKS_URL", "HTTP_AUTH_JWT_JWKS_CACHE_TTL", "HTTP_AUTH_JWT_JWKS_SKIP_STARTUP_WARM",
		"HTTP_AUTH_JWT_ISSUER", "HTTP_AUTH_JWT_AUDIENCE",
		"HTTP_RATE_LIMIT_SENSITIVE_WRITES_ENABLED", "HTTP_RATE_LIMIT_SENSITIVE_WRITES_RPS", "HTTP_RATE_LIMIT_SENSITIVE_WRITES_BURST",
		"API_ARTIFACTS_ENABLED", "ARTIFACTS_MAX_UPLOAD_BYTES", "ARTIFACTS_DOWNLOAD_PRESIGN_TTL", "ARTIFACTS_LIST_MAX_KEYS",
		"TEMPORAL_ENABLED", "TEMPORAL_HOST_PORT", "TEMPORAL_NAMESPACE", "TEMPORAL_TASK_QUEUE",
		"ANALYTICS_CLICKHOUSE_ENABLED", "ANALYTICS_CLICKHOUSE_HTTP_URL", "ANALYTICS_MIRROR_OUTBOX_PUBLISHED",
		"ANALYTICS_CLICKHOUSE_TABLE", "ANALYTICS_MIRROR_MAX_CONCURRENT", "ANALYTICS_INSERT_TIMEOUT", "ANALYTICS_INSERT_MAX_ATTEMPTS",
	} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
