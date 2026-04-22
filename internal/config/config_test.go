package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func setMinimalValidLoadEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("HTTP_ADDR", ":0")
	t.Setenv("OTEL_SERVICE_NAME", "test")
	t.Setenv("APP_NODE_NAME", "node-a")
	t.Setenv("APP_INSTANCE_ID", "node-a-api-1")
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
	if cfg.Build.Version == "" {
		t.Fatal("expected non-empty build version")
	}
	if cfg.Runtime.NodeName != "node-a" {
		t.Fatalf("unexpected node name: %q", cfg.Runtime.NodeName)
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
	t.Setenv("NATS_URL", "nats://127.0.0.1:4222")
	t.Setenv("APP_NODE_NAME", "prod-node-a")
	t.Setenv("APP_INSTANCE_ID", "prod-node-a-api-1")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppEnv != AppEnvProduction {
		t.Fatalf("app env: %q", cfg.AppEnv)
	}
}

func TestLoad_RuntimeMetadataAndURLs(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("PUBLIC_BASE_URL", "https://api.example.com")
	t.Setenv("MACHINE_PUBLIC_BASE_URL", "https://machines.example.com")
	t.Setenv("APP_RUNTIME_ROLE", "api")
	t.Setenv("APP_VERSION", "1.2.3")
	t.Setenv("APP_GIT_SHA", "abc123")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtime.PublicBaseURL != "https://api.example.com" {
		t.Fatalf("public base url: %q", cfg.Runtime.PublicBaseURL)
	}
	if cfg.Runtime.MachinePublicBaseURL != "https://machines.example.com" {
		t.Fatalf("machine public base url: %q", cfg.Runtime.MachinePublicBaseURL)
	}
	if cfg.Runtime.EffectiveRuntimeRole("api") != "api" {
		t.Fatalf("runtime role: %q", cfg.Runtime.EffectiveRuntimeRole("api"))
	}
	if cfg.Build.Version != "1.2.3" || cfg.Build.GitSHA != "abc123" {
		t.Fatalf("unexpected build info: %+v", cfg.Build)
	}
}

func TestLoad_APPBaseURLAliasPreferred(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("APP_BASE_URL", "https://api.ldtv.dev")
	t.Setenv("PUBLIC_BASE_URL", "https://legacy.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtime.PublicBaseURL != "https://api.ldtv.dev" {
		t.Fatalf("public base url: %q", cfg.Runtime.PublicBaseURL)
	}
	if cfg.Runtime.MachinePublicBaseURL != "https://api.ldtv.dev" {
		t.Fatalf("machine public base url: %q", cfg.Runtime.MachinePublicBaseURL)
	}
}

func TestLoad_RedisTLSRequiresAddr(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("REDIS_TLS_ENABLED", "true")
	t.Setenv("REDIS_ADDR", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "REDIS_TLS") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_RedisURLAlias(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_URL", "rediss://default:redis-secret@127.0.0.1:6380/2")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Redis.Addr != "127.0.0.1:6380" {
		t.Fatalf("redis addr: %q", cfg.Redis.Addr)
	}
	if cfg.Redis.Username != "default" || cfg.Redis.Password != "redis-secret" {
		t.Fatalf("unexpected redis auth: %+v", cfg.Redis)
	}
	if cfg.Redis.DB != 2 || !cfg.Redis.TLSEnabled {
		t.Fatalf("unexpected redis config: %+v", cfg.Redis)
	}
}

func TestLoad_InvalidRedisURLRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_URL", "://bad")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_MQTTBrokerRequiresClientIDFamily(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("MQTT_BROKER_URL", "tcp://broker.example.com:1883")
	t.Setenv("MQTT_CLIENT_ID", "")
	t.Setenv("MQTT_CLIENT_ID_API", "")
	t.Setenv("MQTT_CLIENT_ID_INGEST", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "MQTT_BROKER_URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_OpsTimeoutsAndAddr(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_OPS_ADDR", "127.0.0.1:9099")
	t.Setenv("OPS_READINESS_TIMEOUT", "3s")
	t.Setenv("OPS_SHUTDOWN_TIMEOUT", "7s")
	t.Setenv("TRACER_SHUTDOWN_TIMEOUT", "9s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ops.HTTPAddr != "127.0.0.1:9099" {
		t.Fatalf("ops addr: %q", cfg.Ops.HTTPAddr)
	}
	if cfg.Ops.ReadinessTimeout != 3*time.Second || cfg.Ops.ShutdownTimeout != 7*time.Second || cfg.Ops.TracerShutdownTimeout != 9*time.Second {
		t.Fatalf("unexpected ops config: %+v", cfg.Ops)
	}
}

func TestValidate_OTELConflict(t *testing.T) {
	cfg := &Config{
		AppEnv:    AppEnvDevelopment,
		LogLevel:  "info",
		LogFormat: "json",
		Runtime: RuntimeConfig{
			NodeName:   "node-a",
			InstanceID: "node-a-api-1",
		},
		Build: BuildConfig{
			Version: "dev",
		},
		HTTP: HTTPConfig{
			Addr:              ":8080",
			ShutdownTimeout:   1,
			ReadHeaderTimeout: 1,
			ReadTimeout:       1,
			WriteTimeout:      1,
			IdleTimeout:       1,
		},
		Ops: OperationsConfig{
			ReadinessTimeout:      1,
			ShutdownTimeout:       1,
			TracerShutdownTimeout: 1,
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

func TestLoad_TemporalSchedulingFlagsAndWorkerMetricsListen(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("TEMPORAL_SCHEDULE_PAYMENT_PENDING_TIMEOUT", "true")
	t.Setenv("TEMPORAL_SCHEDULE_REFUND_ORCHESTRATION", "true")
	t.Setenv("TEMPORAL_WORKER_METRICS_LISTEN", "127.0.0.1:9094")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Temporal.SchedulePaymentPendingTimeout || !cfg.Temporal.ScheduleRefundOrchestration {
		t.Fatalf("unexpected temporal flags: %+v", cfg.Temporal)
	}
	if cfg.TemporalWorkerMetricsListen != "127.0.0.1:9094" {
		t.Fatalf("temporal worker listen: %q", cfg.TemporalWorkerMetricsListen)
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

func TestLoad_PaymentWebhookSecretAlias(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("PAYMENT_WEBHOOK_SECRET", "webhook-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commerce.PaymentWebhookHMACSecret != "webhook-secret" {
		t.Fatalf("payment webhook secret: %q", cfg.Commerce.PaymentWebhookHMACSecret)
	}
}

func TestLoad_SMTPConfig(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USER", "mailer")
	t.Setenv("SMTP_PASSWORD", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SMTP.Host != "smtp.example.com" || cfg.SMTP.Port != 587 {
		t.Fatalf("unexpected smtp config: %+v", cfg.SMTP)
	}
	if cfg.SMTP.Username != "mailer" || cfg.SMTP.Password != "secret" {
		t.Fatalf("unexpected smtp credentials: %+v", cfg.SMTP)
	}
}

func TestLoad_SMTPHostRequiresPositivePort(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "SMTP_PORT") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMain(m *testing.M) {
	// Isolate tests from developer machine env.
	for _, k := range []string{
		"APP_ENV", "LOG_LEVEL", "LOG_FORMAT",
		"APP_NODE_NAME", "APP_INSTANCE_ID", "APP_RUNTIME_ROLE",
		"APP_VERSION", "APP_GIT_SHA", "APP_BUILD_TIME",
		"APP_BASE_URL", "PUBLIC_BASE_URL", "MACHINE_PUBLIC_BASE_URL",
		"HTTP_ADDR", "HTTP_SHUTDOWN_TIMEOUT", "HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT",
		"HTTP_SWAGGER_UI_ENABLED",
		"HTTP_OPS_ADDR", "OPS_READINESS_TIMEOUT", "OPS_SHUTDOWN_TIMEOUT", "TRACER_SHUTDOWN_TIMEOUT",
		"GRPC_ENABLED", "GRPC_ADDR", "GRPC_SHUTDOWN_TIMEOUT",
		"DATABASE_URL", "DATABASE_MAX_CONNS", "DATABASE_MIN_CONNS", "DATABASE_MAX_CONN_IDLE_TIME", "DATABASE_MAX_CONN_LIFETIME",
		"REDIS_ADDR", "REDIS_URL", "REDIS_USERNAME", "REDIS_PASSWORD", "REDIS_DB", "REDIS_TLS_ENABLED", "REDIS_TLS_INSECURE_SKIP_VERIFY",
		"READINESS_STRICT", "METRICS_ENABLED", "WORKER_METRICS_LISTEN", "RECONCILER_METRICS_LISTEN", "MQTT_INGEST_METRICS_LISTEN",
		"MQTT_BROKER_URL", "MQTT_CLIENT_ID", "MQTT_CLIENT_ID_API", "MQTT_CLIENT_ID_INGEST", "MQTT_USERNAME", "MQTT_PASSWORD", "MQTT_TOPIC_PREFIX",
		"PAYMENT_WEBHOOK_SECRET", "COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET",
		"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD",
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
		"NATS_URL",
		"TELEMETRY_MAX_PAYLOAD_BYTES", "TELEMETRY_MAX_POINTS_PER_MESSAGE", "TELEMETRY_MAX_TAGS_PER_MESSAGE",
		"TELEMETRY_PER_MACHINE_MSGS_PER_SEC", "TELEMETRY_PER_MACHINE_BURST", "TELEMETRY_GLOBAL_MAX_INFLIGHT",
		"TELEMETRY_WORKER_CONCURRENCY", "TELEMETRY_DROP_ON_BACKPRESSURE", "TELEMETRY_LEGACY_POSTGRES_INGEST", "TELEMETRY_SUBMIT_WAIT_MS",
		"TELEMETRY_STREAM_MAX_BYTES", "TELEMETRY_STREAM_MAX_AGE", "TELEMETRY_CONSUMER_MAX_ACK_PENDING", "TELEMETRY_CONSUMER_ACK_WAIT",
		"TELEMETRY_CONSUMER_MAX_DELIVER", "TELEMETRY_CONSUMER_BATCH_SIZE", "TELEMETRY_CONSUMER_PULL_TIMEOUT",
		"TELEMETRY_PROJECTION_MAX_CONCURRENCY", "TELEMETRY_PROJECTION_DEDUPE_LRU_SIZE", "TELEMETRY_READINESS_MAX_PENDING",
		"TELEMETRY_READINESS_MAX_PROJECTION_FAIL_STREAK", "TELEMETRY_CONSUMER_LAG_POLL_INTERVAL",
	} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
