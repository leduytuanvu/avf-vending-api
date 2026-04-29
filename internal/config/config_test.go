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
	if !cfg.MetricsExposeOnPublicHTTP {
		t.Fatal("development should default METRICS_EXPOSE_ON_PUBLIC_HTTP to true when unset")
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
	if cfg.Runtime.Region != "" {
		t.Fatalf("development should not invent a region, got %q", cfg.Runtime.Region)
	}
}

func TestLoad_RedisRuntimeFeatureFlags(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("CACHE_ENABLED", "true")
	t.Setenv("SALE_CATALOG_CACHE_TTL", "2m")
	t.Setenv("AUTH_ACCESS_JTI_REVOCATION_ENABLED", "true")
	t.Setenv("AUTH_REVOCATION_REDIS_FAIL_OPEN", "true")
	t.Setenv("RATE_LIMIT_GRPC_MACHINE_HOT_PER_MIN", "123")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.RedisRuntime.CacheEnabled {
		t.Fatal("expected cache enabled")
	}
	if cfg.RedisRuntime.SaleCatalogCacheTTL != 2*time.Minute {
		t.Fatalf("unexpected sale catalog ttl: %s", cfg.RedisRuntime.SaleCatalogCacheTTL)
	}
	if !cfg.RedisRuntime.AuthAccessJTIRevocationEnabled {
		t.Fatal("expected auth JTI revocation enabled")
	}
	if !cfg.RedisRuntime.AuthRevocationRedisFailOpen {
		t.Fatal("development should allow explicit auth revocation fail-open")
	}
	if cfg.RedisRuntime.GRPCMachineHotPerMinute != 123 {
		t.Fatalf("unexpected grpc hot limit: %d", cfg.RedisRuntime.GRPCMachineHotPerMinute)
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

func TestLoad_GRPCRequireMachineJWTDefaultsEnabled(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("GRPC_ENABLED", "true")
	t.Setenv("GRPC_ADDR", "127.0.0.1:0")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.GRPC.RequireMachineJWT {
		t.Fatal("GRPC_REQUIRE_MACHINE_JWT should default to true")
	}
	if !cfg.GRPC.RequireGRPCIdempotency {
		t.Fatal("GRPC_REQUIRE_IDEMPOTENCY should default to true")
	}
}

func TestLoad_MachineJWTPolicyAliases(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("AUTH_ISSUER", "https://auth.example.test")
	t.Setenv("AUTH_ADMIN_AUDIENCE", "avf-admin")
	t.Setenv("AUTH_MACHINE_AUDIENCE", "avf-machine")
	t.Setenv("MACHINE_ACCESS_TTL", "7m")
	t.Setenv("MACHINE_REFRESH_TTL", "168h")
	t.Setenv("MACHINE_TOKEN_CLOCK_SKEW", "20s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAuth.ExpectedIssuer != "https://auth.example.test" || cfg.MachineJWT.ExpectedIssuer != "https://auth.example.test" {
		t.Fatalf("issuer aliases not applied: http=%q machine=%q", cfg.HTTPAuth.ExpectedIssuer, cfg.MachineJWT.ExpectedIssuer)
	}
	if cfg.HTTPAuth.ExpectedAudience != "avf-admin" {
		t.Fatalf("admin audience=%q", cfg.HTTPAuth.ExpectedAudience)
	}
	if cfg.MachineJWT.ExpectedAudience != "avf-machine" {
		t.Fatalf("machine audience=%q", cfg.MachineJWT.ExpectedAudience)
	}
	if cfg.MachineJWT.AccessTokenTTL != 7*time.Minute || cfg.MachineJWT.RefreshTokenTTL != 168*time.Hour {
		t.Fatalf("machine ttls access=%s refresh=%s", cfg.MachineJWT.AccessTokenTTL, cfg.MachineJWT.RefreshTokenTTL)
	}
	if cfg.MachineJWT.JWTLeeway != 20*time.Second {
		t.Fatalf("machine clock skew=%s", cfg.MachineJWT.JWTLeeway)
	}
	if cfg.MachineJWT.RequireAudience {
		t.Fatal("development should default MACHINE_AUTH_REQUIRE_AUDIENCE to false")
	}
}

func TestValidateGRPCProductionReflection_RejectsWhenEnabled(t *testing.T) {
	cfg := &Config{
		AppEnv: AppEnvProduction,
		GRPC: GRPCConfig{
			Enabled:           true,
			ReflectionEnabled: true,
		},
	}
	if err := cfg.validateGRPCProductionReflection(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateGRPCProductionReflection_AllowsDevelopment(t *testing.T) {
	cfg := &Config{
		AppEnv: AppEnvDevelopment,
		GRPC: GRPCConfig{
			Enabled:           true,
			ReflectionEnabled: true,
		},
	}
	if err := cfg.validateGRPCProductionReflection(); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_RedisPasswordWithoutAddrRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("REDIS_PASSWORD", "fixture")
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
	t.Setenv("DATABASE_URL", "postgres://user:"+"pass@localhost:5432/db?sslmode=disable")
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

func TestLoad_PostgresPoolDefaults(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:"+"pass@localhost:5432/db?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Postgres.MaxConns != 3 {
		t.Fatalf("max conns: got %d want 3", cfg.Postgres.MaxConns)
	}
	if cfg.Postgres.MinConns != 0 {
		t.Fatalf("min conns: got %d want 0", cfg.Postgres.MinConns)
	}
	if cfg.Postgres.MaxConnIdleTime != 5*time.Minute {
		t.Fatalf("idle time: got %s want 5m", cfg.Postgres.MaxConnIdleTime)
	}
	if cfg.Postgres.MaxConnLifetime != 30*time.Minute {
		t.Fatalf("lifetime: got %s want 30m", cfg.Postgres.MaxConnLifetime)
	}
}

func TestLoad_PostgresPoolPerProcessOverride(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:"+"pass@localhost:5432/db?sslmode=disable")
	t.Setenv("DATABASE_MAX_CONNS", "5")
	t.Setenv("WORKER_DATABASE_MAX_CONNS", "2")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Postgres.MaxConnsForProcess("worker"); got != 2 {
		t.Fatalf("worker max conns: got %d want 2", got)
	}
	if got := cfg.Postgres.MaxConnsForProcess("api"); got != 5 {
		t.Fatalf("api max conns: got %d want 5", got)
	}
}

func TestPostgresPoolSummaryForProcess(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:"+"pass@localhost:5432/db?sslmode=disable")
	t.Setenv("DATABASE_MAX_CONNS", "10")
	t.Setenv("API_DATABASE_MAX_CONNS", "4")
	t.Setenv("MQTT_INGEST_DATABASE_MAX_CONNS", "2")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	api := cfg.Postgres.PoolSummaryForProcess("api")
	if api.MaxConns != 4 || api.ProcessName != "api" {
		t.Fatalf("api summary: %+v", api)
	}
	mqtt := cfg.Postgres.PoolSummaryForProcess("mqtt-ingest")
	if mqtt.MaxConns != 2 {
		t.Fatalf("mqtt-ingest summary: %+v", mqtt)
	}
	worker := cfg.Postgres.PoolSummaryForProcess("worker")
	if worker.MaxConns != 10 {
		t.Fatalf("worker summary: %+v", worker)
	}
}

func TestLoad_PostgresInvalidMaxConnIdleDuration(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:"+"pass@localhost:5432/db?sslmode=disable")
	t.Setenv("DATABASE_MAX_CONN_IDLE_TIME", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "DATABASE_MAX_CONN_IDLE_TIME") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_PostgresPoolInvalidOverrideRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:"+"pass@localhost:5432/db?sslmode=disable")
	t.Setenv("WORKER_DATABASE_MAX_CONNS", "nope")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "WORKER_DATABASE_MAX_CONNS") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_PostgresPoolSettingsRequireDatabaseURL(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("WORKER_DATABASE_MAX_CONNS", "1")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "DATABASE_*") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_PostgresPoolOverrideBelowMinRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:"+"pass@localhost:5432/db?sslmode=disable")
	t.Setenv("DATABASE_MIN_CONNS", "2")
	t.Setenv("API_DATABASE_MAX_CONNS", "1")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API_DATABASE_MAX_CONNS") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_ProductionAppEnvStillValidatedStrictly(t *testing.T) {
	setMinimalProductionLoadEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppEnv != AppEnvProduction {
		t.Fatalf("app env: %q", cfg.AppEnv)
	}
}

func TestLoad_AuditCriticalFailOpenForbiddenOutsideDevTest(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("AUDIT_CRITICAL_FAIL_OPEN", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "AUDIT_CRITICAL_FAIL_OPEN") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_InvalidPlatformAuditOrganizationID(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("PLATFORM_AUDIT_ORGANIZATION_ID", "not-a-uuid")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "PLATFORM_AUDIT_ORGANIZATION_ID") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func setMinimalProductionLoadEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("OTEL_SERVICE_NAME", "avf-api-prod")
	t.Setenv("HTTP_AUTH_MODE", "hs256")
	t.Setenv("HTTP_AUTH_JWT_SECRET", "test-prod-user-jwt-secret-needs-32b+")
	t.Setenv("MACHINE_JWT_MODE", "hs256")
	t.Setenv("MACHINE_JWT_SECRET", "test-prod-machine-jwt-secret-32bytes!!")
	t.Setenv("NATS_URL", "nats://nats.prod.internal:4222")
	t.Setenv("APP_REGION", "ap-southeast-1")
	t.Setenv("APP_NODE_NAME", "prod-node-a")
	t.Setenv("APP_INSTANCE_ID", "prod-node-a-api-1")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db.prod.internal:5432/prod?sslmode=require")
	t.Setenv("PAYMENT_ENV", "live")
	t.Setenv("READINESS_STRICT", "true")
	t.Setenv("PUBLIC_BASE_URL", "https://api.ldtv.dev")
	t.Setenv("MQTT_BROKER_URL", "tls://emqx.prod.internal:8883")
	t.Setenv("MQTT_USERNAME", "avf-api-prod")
	t.Setenv("MQTT_PASSWORD", "test-fixture-mqtt-password-not-real")
	t.Setenv("MQTT_CLIENT_ID_INGEST", "avf-prod-test-ingest")
	t.Setenv("MQTT_TOPIC_PREFIX", "avf/devices")
	t.Setenv("COMMERCE_PAYMENT_PROVIDER", "vnpay")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET", "production-test-commerce-webhook-secret-at-least-32chars-xx")
	t.Setenv("HTTP_CORS_ALLOWED_ORIGINS", "")
	t.Setenv("AUTH_ISSUER", "https://api.ldtv.dev")
	t.Setenv("MACHINE_GRPC_ENABLED", "true")
	t.Setenv("GRPC_ADDR", ":9090")
	t.Setenv("GRPC_PUBLIC_BASE_URL", "grpcs://machine-api.ldtv.dev:443")
	t.Setenv("GRPC_BEHIND_TLS_PROXY", "true")
}

func TestLoad_MetricsExposeOnPublicInvalidRejected(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("METRICS_EXPOSE_ON_PUBLIC_HTTP", "maybe")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "METRICS_EXPOSE_ON_PUBLIC_HTTP") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_ProductionMetricsPublicRequiresScrapeToken(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("METRICS_ENABLED", "true")
	t.Setenv("METRICS_EXPOSE_ON_PUBLIC_HTTP", "true")
	_ = os.Unsetenv("METRICS_SCRAPE_TOKEN")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "METRICS_SCRAPE_TOKEN") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_ProductionMetricsPublicRequiresOperatorAllow(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("METRICS_ENABLED", "true")
	t.Setenv("METRICS_EXPOSE_ON_PUBLIC_HTTP", "true")
	t.Setenv("METRICS_SCRAPE_TOKEN", "long-enough-token-abcdef12")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "PRODUCTION_PUBLIC_METRICS_ENDPOINT_ALLOWED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_ProductionMetricsPublicAcceptsWithAllowAndToken(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("METRICS_ENABLED", "true")
	t.Setenv("METRICS_EXPOSE_ON_PUBLIC_HTTP", "true")
	t.Setenv("METRICS_SCRAPE_TOKEN", "long-enough-token-abcdef12")
	t.Setenv("PRODUCTION_PUBLIC_METRICS_ENDPOINT_ALLOWED", "true")
	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_ProductionMetricsPrivateDefaultNoTokenRequired(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("METRICS_ENABLED", "true")
	_ = os.Unsetenv("METRICS_EXPOSE_ON_PUBLIC_HTTP")
	_ = os.Unsetenv("METRICS_SCRAPE_TOKEN")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MetricsExposeOnPublicHTTP {
		t.Fatal("expected METRICS_EXPOSE_ON_PUBLIC_HTTP false by default in production")
	}
}

func TestLoad_OpenAPIJSONDefaultEnabled(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(t *testing.T)
		wantOpen bool
	}{
		{"development", setMinimalValidLoadEnv, true},
		{"staging", minimalStaging, true},
		{"production", setMinimalProductionLoadEnv, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			_ = os.Unsetenv("HTTP_OPENAPI_JSON_ENABLED")
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.OpenAPIJSONEnabled != tc.wantOpen {
				t.Fatalf("OpenAPIJSONEnabled: got %v want %v in %s", cfg.OpenAPIJSONEnabled, tc.wantOpen, tc.name)
			}
		})
	}
}

func TestLoad_SwaggerUIWithoutOpenAPIRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_SWAGGER_UI_ENABLED", "true")
	t.Setenv("HTTP_OPENAPI_JSON_ENABLED", "false")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when Swagger UI enabled but OpenAPI JSON disabled")
	}
}

func TestLoad_SwaggerUIEnabled_production(t *testing.T) {
	t.Run("HTTP_SWAGGER_UI_ENABLED_true", func(t *testing.T) {
		setMinimalProductionLoadEnv(t)
		t.Setenv("HTTP_SWAGGER_UI_ENABLED", "true")
		t.Setenv("HTTP_OPENAPI_JSON_ENABLED", "true")
		t.Setenv("PRODUCTION_SWAGGER_UI_ALLOWED", "true")
		t.Setenv("PRODUCTION_OPENAPI_JSON_ALLOWED", "true")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if !cfg.SwaggerUIEnabled {
			t.Fatal("expected SwaggerUIEnabled true")
		}
	})
	t.Run("HTTP_SWAGGER_UI_ENABLED_false", func(t *testing.T) {
		setMinimalProductionLoadEnv(t)
		t.Setenv("HTTP_SWAGGER_UI_ENABLED", "false")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.SwaggerUIEnabled {
			t.Fatal("expected SwaggerUIEnabled false")
		}
	})
	t.Run("HTTP_SWAGGER_UI_ENABLED_unset", func(t *testing.T) {
		setMinimalProductionLoadEnv(t)
		_ = os.Unsetenv("HTTP_SWAGGER_UI_ENABLED")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.SwaggerUIEnabled {
			t.Fatal("expected SwaggerUIEnabled false when unset in production")
		}
	})
}

func TestLoad_RuntimeMetadataAndURLs(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("APP_REGION", "ap-southeast-1")
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
	if cfg.Runtime.Region != "ap-southeast-1" {
		t.Fatalf("region: %q", cfg.Runtime.Region)
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

func TestLoad_RuntimeRegionAlias(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("REGION", "local-dr")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtime.Region != "local-dr" {
		t.Fatalf("region alias: %q", cfg.Runtime.Region)
	}
}

func TestLoad_ProductionRequiresRegion(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("APP_REGION", "")
	t.Setenv("REGION", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected production region error")
	}
	if !strings.Contains(err.Error(), "APP_REGION") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_RuntimeIdentityRejectsPlaceholdersAndUnsafeChars(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("APP_REGION", "CHANGE_ME_REGION")
	_, err := Load()
	if err == nil {
		t.Fatal("expected placeholder error")
	}
	if !strings.Contains(err.Error(), "APP_REGION") {
		t.Fatalf("unexpected error: %v", err)
	}

	setMinimalValidLoadEnv(t)
	t.Setenv("APP_REGION", "ap-southeast-1")
	t.Setenv("APP_INSTANCE_ID", "node a/api")
	_, err = Load()
	if err == nil {
		t.Fatal("expected invalid instance id error")
	}
	if !strings.Contains(err.Error(), "APP_INSTANCE_ID") {
		t.Fatalf("unexpected error: %v", err)
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
	t.Setenv("REDIS_URL", "rediss://default:"+"fixture@127.0.0.1:6380/2")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Redis.Addr != "127.0.0.1:6380" {
		t.Fatalf("redis addr: %q", cfg.Redis.Addr)
	}
	if cfg.Redis.Username != "default" || cfg.Redis.Password != "fixture" {
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
		Capacity: loadCapacityLimitsConfig(),
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidate_PostgresRequiresPositiveMaxConnsWhenURLSet(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:"+"pass@localhost:5432/db?sslmode=disable")
	t.Setenv("DATABASE_MAX_CONNS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_PostgresNegativeMinConnsRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:"+"pass@localhost:5432/db?sslmode=disable")
	t.Setenv("DATABASE_MIN_CONNS", "-1")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must be >= 0") {
		t.Fatalf("unexpected: %v", err)
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

func TestLoad_AbuseRateLimit_enabledRequiresLimits(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_LOGIN_PER_MIN", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "RATE_LIMIT_LOGIN_PER_MIN") {
		t.Fatalf("unexpected error: %v", err)
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
	if cfg.Analytics.ProjectOutboxEvents {
		t.Fatal("expected ANALYTICS_PROJECT_OUTBOX_EVENTS=false by default")
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

func TestLoad_AnalyticsProjectionRequiresClickHouse(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("ANALYTICS_PROJECT_OUTBOX_EVENTS", "true")
	t.Setenv("ANALYTICS_CLICKHOUSE_ENABLED", "false")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ANALYTICS_PROJECT_OUTBOX_EVENTS") {
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

func TestLoad_AnalyticsProjectionBadTableName(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("ANALYTICS_CLICKHOUSE_ENABLED", "true")
	t.Setenv("ANALYTICS_CLICKHOUSE_HTTP_URL", "http://localhost:8123/avf")
	t.Setenv("ANALYTICS_PROJECT_OUTBOX_EVENTS", "true")
	t.Setenv("ANALYTICS_PROJECTION_TABLE", "bad-name")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ANALYTICS_PROJECTION_TABLE") {
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
	setMinimalProductionLoadEnv(t)
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
	t.Setenv("PAYMENT_WEBHOOK_SECRET", "fixture")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commerce.PaymentWebhookHMACSecret != "fixture" {
		t.Fatalf("payment webhook secret: %q", cfg.Commerce.PaymentWebhookHMACSecret)
	}
}

func TestLoad_PaymentWebhookAllowUnsignedRejectedInProduction(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_PaymentWebhookAllowUnsignedRejectedInStaging(t *testing.T) {
	minimalStaging(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_Production_requiresCommercePaymentWebhookSecret(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("COMMERCE_PAYMENT_WEBHOOK_SECRET")
	_ = os.Unsetenv("PAYMENT_WEBHOOK_SECRET")
	_ = os.Unsetenv("COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COMMERCE_PAYMENT_WEBHOOK_SECRET") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_Development_allowUnsignedTrueLoads(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Commerce.PaymentWebhookAllowUnsigned {
		t.Fatal("expected PaymentWebhookAllowUnsigned true")
	}
}

func TestLoad_PaymentWebhookReplayWindowOverridesLegacySkew(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_REPLAY_WINDOW", "120")
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS", "600")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commerce.PaymentWebhookTimestampSkew != 120*time.Second {
		t.Fatalf("replay window: %s", cfg.Commerce.PaymentWebhookTimestampSkew)
	}
}

func TestLoad_PaymentWebhookSecretPrefersCOMMERCE_PAYMENT_WEBHOOK_SECRET(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_SECRET", "preferred-secret")
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET", "legacy-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commerce.PaymentWebhookHMACSecret != "preferred-secret" {
		t.Fatalf("unexpected secret: %q", cfg.Commerce.PaymentWebhookHMACSecret)
	}
}

func TestLoad_PaymentWebhookVerificationUnsupported(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_VERIFICATION", "stripe")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COMMERCE_PAYMENT_WEBHOOK_VERIFICATION") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_PaymentWebhookTimestampSkewOutOfRange(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS", "10")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "TIMESTAMP_SKEW") && !strings.Contains(err.Error(), "REPLAY_WINDOW") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_SMTPConfig(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USER", "mailer")
	t.Setenv("SMTP_PASSWORD", "fixture")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SMTP.Host != "smtp.example.com" || cfg.SMTP.Port != 587 {
		t.Fatalf("unexpected smtp config: %+v", cfg.SMTP)
	}
	if cfg.SMTP.Username != "mailer" || cfg.SMTP.Password != "fixture" {
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

func TestMQTTConfig_validateProductionRequiresTLS(t *testing.T) {
	m := MQTTConfig{
		BrokerURL:   "tcp://mqtt:1883",
		TopicPrefix: "avf/devices",
		ClientID:    "c",
	}
	if err := m.validate(AppEnvProduction); err == nil {
		t.Fatal("expected error when MQTT is plain TCP in production")
	}
	m = MQTTConfig{
		BrokerURL:   "tcp://mqtt:1883",
		TopicPrefix: "avf/devices",
		ClientID:    "c",
		TLSEnabled:  true,
	}
	if err := m.validate(AppEnvProduction); err == nil {
		t.Fatal("expected error when production MQTT uses tcp:// to non-localhost even if MQTT_TLS_ENABLED=true")
	}
	m = MQTTConfig{
		BrokerURL:   "tcp://localhost:1883",
		TopicPrefix: "avf/devices",
		ClientID:    "c",
	}
	if err := m.validate(AppEnvProduction); err != nil {
		t.Fatalf("localhost tcp should be allowed in production: %v", err)
	}
	m = MQTTConfig{
		BrokerURL:          "ssl://mqtt:8883",
		TopicPrefix:        "avf/devices",
		ClientID:           "c",
		InsecureSkipVerify: true,
	}
	if err := m.validate(AppEnvProduction); err == nil {
		t.Fatal("expected MQTT_INSECURE_SKIP_VERIFY rejected in production")
	}
}

func TestLoad_Staging_RequiresExplicitCORSVar(t *testing.T) {
	minimalStaging(t)
	_ = os.Unsetenv("HTTP_CORS_ALLOWED_ORIGINS")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when HTTP_CORS_ALLOWED_ORIGINS unset in staging")
	}
	if !strings.Contains(err.Error(), "HTTP_CORS_ALLOWED_ORIGINS") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RequiresExplicitCORSVar(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("HTTP_CORS_ALLOWED_ORIGINS")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when HTTP_CORS_ALLOWED_ORIGINS unset in production")
	}
	if !strings.Contains(err.Error(), "HTTP_CORS_ALLOWED_ORIGINS") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsShortHS256JWTSecret(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("HTTP_AUTH_JWT_SECRET", strings.Repeat("a", 31))
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for short JWT secret in production")
	}
	if !strings.Contains(err.Error(), "HTTP_AUTH_JWT_SECRET") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsCORSWildcard(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("HTTP_CORS_ALLOWED_ORIGINS", "https://app.example.com,*")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for * in CORS origins")
	}
	if !strings.Contains(err.Error(), "HTTP_CORS_ALLOWED_ORIGINS") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_GRPCEnabledRequiresProcessHealthReadiness(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("GRPC_ENABLED", "true")
	t.Setenv("GRPC_ADDR", ":9090")
	t.Setenv("GRPC_HEALTH_USE_PROCESS_READINESS", "false")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when GRPC_HEALTH_USE_PROCESS_READINESS=false in production with gRPC on")
	}
	if !strings.Contains(err.Error(), "GRPC_HEALTH_USE_PROCESS_READINESS") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsCommerceUnsafeUnsignedProduction(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsCommerceAllowUnsigned(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Staging_RejectsCommerceAllowUnsigned(t *testing.T) {
	minimalStaging(t)
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_HTTPAuthJWTAlgConflictsWithMode(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_AUTH_JWT_ALG", "RS256")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for HS256 mode with RS256 alg label")
	}
	if !strings.Contains(err.Error(), "HTTP_AUTH_JWT_ALG") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_HTTPAuthJWTAlgAcceptsJWKSMode(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_AUTH_MODE", "jwt_jwks")
	t.Setenv("HTTP_AUTH_JWT_JWKS_URL", "https://issuer.example.com/.well-known/jwks.json")
	t.Setenv("HTTP_AUTH_LOGIN_JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("HTTP_AUTH_JWT_ALG", "RS256")
	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_UserMachineServiceJWTAliases(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("USER_JWT_MODE", "hs256")
	t.Setenv("USER_JWT_SECRET", "user-secret-012345678901234567890123")
	t.Setenv("USER_JWT_LOGIN_SECRET", "login-secret-012345678901234567890")
	t.Setenv("MACHINE_JWT_MODE", "hs256")
	t.Setenv("MACHINE_JWT_SECRET", "machine-secret-012345678901234567890")
	t.Setenv("SERVICE_JWT_SECRET", "service-secret-012345678901234567890")
	t.Setenv("INTERNAL_GRPC_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(cfg.HTTPAuth.JWTSecret); got != "user-secret-012345678901234567890123" {
		t.Fatalf("user jwt secret=%q", got)
	}
	if got := string(cfg.MachineJWT.JWTSecret); got != "machine-secret-012345678901234567890" {
		t.Fatalf("machine jwt secret=%q", got)
	}
	if got := string(cfg.InternalGRPC.ServiceTokenSecret); got != "service-secret-012345678901234567890" {
		t.Fatalf("service jwt secret=%q", got)
	}
}

func TestLoad_MachineJWTAlgCrossCheck(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_AUTH_JWT_SECRET", "dev-secret-012345678901234567890123")
	t.Setenv("MACHINE_JWT_MODE", "hs256")
	t.Setenv("MACHINE_JWT_ALG", "RS256")

	_, err := Load()
	if err == nil {
		t.Fatal("expected MACHINE_JWT_ALG cross-check error")
	}
	if !strings.Contains(err.Error(), "MACHINE_JWT_ALG") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_GRPC_TLSRequiresServerCertFiles(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("GRPC_ENABLED", "true")
	t.Setenv("GRPC_ADDR", ":9090")
	t.Setenv("GRPC_TLS_ENABLED", "true")
	t.Setenv("GRPC_TLS_CERT_FILE", "")
	t.Setenv("GRPC_TLS_KEY_FILE", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GRPC_TLS_CERT_FILE") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_GRPC_ClientAuthRequireNeedsCA(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("GRPC_ENABLED", "true")
	t.Setenv("GRPC_ADDR", ":9090")
	t.Setenv("GRPC_TLS_ENABLED", "true")
	t.Setenv("GRPC_TLS_CERT_FILE", "server.crt")
	t.Setenv("GRPC_TLS_KEY_FILE", "server.key")
	t.Setenv("GRPC_TLS_CLIENT_AUTH", "require")
	_ = os.Unsetenv("GRPC_TLS_CLIENT_CA_FILE")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GRPC_TLS_CLIENT_CA_FILE") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_GRPC_CertOnlyRequiresTLS(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("GRPC_ENABLED", "true")
	t.Setenv("GRPC_ADDR", ":9090")
	t.Setenv("GRPC_MACHINE_AUTH_CERT_ONLY_ALLOWED", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GRPC_MACHINE_AUTH_CERT_ONLY_ALLOWED") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RequiresExplicitMachineGRPCEnv(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("MACHINE_GRPC_ENABLED")
	t.Setenv("GRPC_ENABLED", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "MACHINE_GRPC_ENABLED=true") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsLegacyRESTWithoutProductionAllow(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("MACHINE_REST_LEGACY_ENABLED", "true")
	t.Setenv("MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION", "false")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Production_AllowsLegacyRESTWithExplicitAllow(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("MACHINE_REST_LEGACY_ENABLED", "true")
	t.Setenv("MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TransportBoundary.MachineRESTLegacyEnabled || !cfg.TransportBoundary.MachineRESTLegacyAllowInProduction {
		t.Fatalf("unexpected boundary: %+v", cfg.TransportBoundary)
	}
}

func TestLoad_Development_DefaultMachineLegacyRESTEnabled(t *testing.T) {
	setMinimalValidLoadEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TransportBoundary.MachineRESTLegacyEnabled {
		t.Fatal("expected MACHINE_REST_LEGACY_ENABLED default true outside production")
	}
}

func TestLoad_Development_EnableLegacyMachineHTTPExplicitFalse(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("ENABLE_LEGACY_MACHINE_HTTP", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TransportBoundary.MachineRESTLegacyEnabled {
		t.Fatal("expected legacy machine HTTP off when ENABLE_LEGACY_MACHINE_HTTP=false")
	}
}

func TestLoad_EnableLegacyMachineHTTPOverridesMachineRestEnv(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("ENABLE_LEGACY_MACHINE_HTTP", "false")
	t.Setenv("MACHINE_REST_LEGACY_ENABLED", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TransportBoundary.MachineRESTLegacyEnabled {
		t.Fatal("ENABLE_LEGACY_MACHINE_HTTP=false must take precedence over MACHINE_REST_LEGACY_ENABLED=true")
	}
}

func TestLoad_Production_RejectsLegacyRESTWithoutProductionAllow_EnableLegacyHTTP(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("ENABLE_LEGACY_MACHINE_HTTP", "true")
	t.Setenv("MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION", "false")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Production_AllowsLegacyRESTWithExplicitAllow_EnableLegacyHTTP(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("ENABLE_LEGACY_MACHINE_HTTP", "true")
	t.Setenv("MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TransportBoundary.MachineRESTLegacyEnabled || !cfg.TransportBoundary.MachineRESTLegacyAllowInProduction {
		t.Fatalf("unexpected boundary: %+v", cfg.TransportBoundary)
	}
}

func TestValidateGRPCProductionExposure_RejectsPlaintext(t *testing.T) {
	cfg := &Config{
		AppEnv: AppEnvProduction,
		GRPC: GRPCConfig{
			Enabled:        true,
			BehindTLSProxy: false,
			TLS:            GRPCServerTLSConfig{Enabled: false},
			PublicBaseURL:  "grpcs://machine-api.example.com:443",
		},
	}
	if err := cfg.validateGRPCProductionExposure(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateGRPCProductionExposure_RejectsMissingPublicURL(t *testing.T) {
	cfg := &Config{
		AppEnv: AppEnvProduction,
		GRPC: GRPCConfig{
			Enabled:        true,
			BehindTLSProxy: true,
			TLS:            GRPCServerTLSConfig{Enabled: false},
			PublicBaseURL:  "",
		},
	}
	if err := cfg.validateGRPCProductionExposure(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateGRPCProductionExposure_RejectsBadScheme(t *testing.T) {
	cfg := &Config{
		AppEnv: AppEnvProduction,
		GRPC: GRPCConfig{
			Enabled:        true,
			BehindTLSProxy: true,
			TLS:            GRPCServerTLSConfig{Enabled: false},
			PublicBaseURL:  "https://machine-api.example.com/",
		},
	}
	if err := cfg.validateGRPCProductionExposure(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateGRPCProductionExposure_RejectsInsecureGrpcScheme(t *testing.T) {
	cfg := &Config{
		AppEnv: AppEnvProduction,
		GRPC: GRPCConfig{
			Enabled:        true,
			BehindTLSProxy: true,
			TLS:            GRPCServerTLSConfig{Enabled: false},
			PublicBaseURL:  "grpc://machine-api.example.com:443",
		},
	}
	err := cfg.validateGRPCProductionExposure()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "grpcs://") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateGRPCProductionExposure_AllowsBehindProxy(t *testing.T) {
	cfg := &Config{
		AppEnv: AppEnvProduction,
		GRPC: GRPCConfig{
			Enabled:        true,
			BehindTLSProxy: true,
			TLS:            GRPCServerTLSConfig{Enabled: false},
			PublicBaseURL:  "grpcs://machine-api.example.com:443",
		},
	}
	if err := cfg.validateGRPCProductionExposure(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateGRPCProductionExposure_AllowsDirectTLS(t *testing.T) {
	cfg := &Config{
		AppEnv: AppEnvProduction,
		GRPC: GRPCConfig{
			Enabled:       true,
			TLS:           GRPCServerTLSConfig{Enabled: true},
			PublicBaseURL: "grpcs://machine-api.example.com:443",
		},
	}
	if err := cfg.validateGRPCProductionExposure(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateGRPCProductionExposure_NoOpOutsideProduction(t *testing.T) {
	cfg := &Config{
		AppEnv: AppEnvStaging,
		GRPC: GRPCConfig{
			Enabled:       true,
			TLS:           GRPCServerTLSConfig{Enabled: false},
			PublicBaseURL: "",
		},
	}
	if err := cfg.validateGRPCProductionExposure(); err != nil {
		t.Fatal(err)
	}
}

func TestGRPC_validate_RejectsTLSAndBehindProxyTogether(t *testing.T) {
	cfg := GRPCConfig{
		Enabled:             true,
		Addr:                ":9090",
		ShutdownTimeout:     time.Second,
		UnaryHandlerTimeout: time.Minute,
		BehindTLSProxy:      true,
		TLS:                 GRPCServerTLSConfig{Enabled: true},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestGRPC_validate_MaxMsgNegativeRejected(t *testing.T) {
	cfg := GRPCConfig{
		Enabled:             true,
		Addr:                ":9090",
		ShutdownTimeout:     time.Second,
		UnaryHandlerTimeout: time.Minute,
		MaxRecvMsgSize:      -1,
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_GRPC_TLSAndBehindProxyRejected(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("GRPC_ENABLED", "true")
	t.Setenv("GRPC_ADDR", ":9090")
	t.Setenv("GRPC_BEHIND_TLS_PROXY", "true")
	t.Setenv("GRPC_TLS_ENABLED", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Production_RejectsGRPCWithoutTLSExposureGuard(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("GRPC_BEHIND_TLS_PROXY")
	_ = os.Unsetenv("GRPC_PUBLIC_BASE_URL")
	t.Setenv("GRPC_TLS_ENABLED", "false")
	t.Setenv("GRPC_BEHIND_TLS_PROXY", "false")
	t.Setenv("GRPC_PUBLIC_BASE_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "plaintext public exposure") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestMain(m *testing.M) {
	// Isolate tests from developer machine env.
	for _, k := range []string{
		"APP_ENV", "LOG_LEVEL", "LOG_FORMAT",
		"APP_REGION", "REGION", "APP_NODE_NAME", "APP_INSTANCE_ID", "APP_RUNTIME_ROLE",
		"APP_VERSION", "APP_GIT_SHA", "APP_BUILD_TIME",
		"APP_BASE_URL", "PUBLIC_BASE_URL", "MACHINE_PUBLIC_BASE_URL",
		"HTTP_ADDR", "HTTP_SHUTDOWN_TIMEOUT", "HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT",
		"HTTP_SWAGGER_UI_ENABLED", "HTTP_OPENAPI_JSON_ENABLED",
		"HTTP_OPS_ADDR", "OPS_READINESS_TIMEOUT", "OPS_SHUTDOWN_TIMEOUT", "TRACER_SHUTDOWN_TIMEOUT",
		"GRPC_ENABLED", "MACHINE_GRPC_ENABLED", "GRPC_ADDR", "GRPC_SHUTDOWN_TIMEOUT",
		"GRPC_PUBLIC_BASE_URL", "GRPC_BEHIND_TLS_PROXY", "GRPC_MAX_RECV_MSG_SIZE", "GRPC_MAX_SEND_MSG_SIZE",
		"GRPC_REFLECTION_ENABLED", "GRPC_HEALTH_ENABLED", "GRPC_HEALTH_USE_PROCESS_READINESS",
		"GRPC_REQUIRE_MACHINE_AUTH", "GRPC_REQUIRE_MACHINE_JWT", "GRPC_REQUIRE_IDEMPOTENCY", "GRPC_UNARY_HANDLER_TIMEOUT",
		"GRPC_TLS_ENABLED", "GRPC_TLS_CERT_FILE", "GRPC_TLS_KEY_FILE", "GRPC_TLS_CLIENT_CA_FILE", "GRPC_TLS_CLIENT_AUTH",
		"GRPC_MTLS_MACHINE_ID_URI_PREFIX", "GRPC_MACHINE_AUTH_CERT_ONLY_ALLOWED",
		"DATABASE_URL", "DATABASE_MAX_CONNS", "DATABASE_MIN_CONNS", "DATABASE_MAX_CONN_IDLE_TIME", "DATABASE_MAX_CONN_LIFETIME",
		"COLOCATE_APP_WITH_DATA_NODE", "ALLOW_APP_NODE_ON_DATA_NODE", "ENABLE_APP_NODE_B",
		"API_DATABASE_MAX_CONNS", "WORKER_DATABASE_MAX_CONNS", "MQTT_INGEST_DATABASE_MAX_CONNS", "RECONCILER_DATABASE_MAX_CONNS", "TEMPORAL_WORKER_DATABASE_MAX_CONNS",
		"REDIS_ADDR", "REDIS_URL", "REDIS_USERNAME", "REDIS_PASSWORD", "REDIS_DB", "REDIS_TLS_ENABLED", "REDIS_TLS_INSECURE_SKIP_VERIFY",
		"READINESS_STRICT", "METRICS_ENABLED", "WORKER_METRICS_LISTEN", "RECONCILER_METRICS_LISTEN", "MQTT_INGEST_METRICS_LISTEN",
		"MQTT_BROKER_URL", "MQTT_CLIENT_ID", "MQTT_CLIENT_ID_API", "MQTT_CLIENT_ID_INGEST", "MQTT_USERNAME", "MQTT_PASSWORD", "MQTT_TOPIC_PREFIX",
		"MQTT_TOPIC_LAYOUT", "MQTT_TLS_ENABLED", "MQTT_CA_FILE", "MQTT_CERT_FILE", "MQTT_KEY_FILE", "MQTT_INSECURE_SKIP_VERIFY",
		"ENABLE_LEGACY_MACHINE_HTTP", "MACHINE_REST_LEGACY_ENABLED", "MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION", "MQTT_COMMAND_TRANSPORT",
		"PAYMENT_WEBHOOK_SECRET", "COMMERCE_PAYMENT_WEBHOOK_SECRET", "COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET",
		"COMMERCE_PAYMENT_WEBHOOK_VERIFICATION", "COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS", "COMMERCE_PAYMENT_WEBHOOK_REPLAY_WINDOW",
		"COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED", "COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION",
		"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD",
		"OTEL_SERVICE_NAME", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_INSECURE", "OTEL_SDK_DISABLED",
		"HTTP_AUTH_MODE", "HTTP_AUTH_JWT_ALG", "HTTP_AUTH_JWT_SECRET", "HTTP_AUTH_JWT_SECRET_PREVIOUS",
		"HTTP_AUTH_JWT_LEEWAY", "HTTP_AUTH_JWT_RSA_PUBLIC_KEY", "HTTP_AUTH_JWT_RSA_PUBLIC_KEY_FILE",
		"HTTP_AUTH_JWT_ED25519_PUBLIC_KEY", "HTTP_AUTH_JWT_ED25519_PUBLIC_KEY_FILE",
		"HTTP_AUTH_JWT_JWKS_URL", "HTTP_AUTH_JWT_JWKS_CACHE_TTL", "HTTP_AUTH_JWT_JWKS_SKIP_STARTUP_WARM",
		"HTTP_AUTH_JWT_ISSUER", "HTTP_AUTH_JWT_AUDIENCE",
		"HTTP_RATE_LIMIT_SENSITIVE_WRITES_ENABLED", "HTTP_RATE_LIMIT_SENSITIVE_WRITES_RPS", "HTTP_RATE_LIMIT_SENSITIVE_WRITES_BURST",
		"RATE_LIMIT_ENABLED", "RATE_LIMIT_LOGIN_PER_MIN", "RATE_LIMIT_REFRESH_PER_MIN", "RATE_LIMIT_ADMIN_MUTATION_PER_MIN",
		"RATE_LIMIT_MACHINE_PER_MIN", "RATE_LIMIT_WEBHOOK_PER_MIN", "RATE_LIMIT_PUBLIC_PER_MIN",
		"RATE_LIMIT_COMMAND_DISPATCH_PER_MIN", "RATE_LIMIT_REPORTS_READ_PER_MIN", "RATE_LIMIT_LOCKOUT_WINDOW",
		"API_ARTIFACTS_ENABLED", "ARTIFACTS_MAX_UPLOAD_BYTES", "ARTIFACTS_DOWNLOAD_PRESIGN_TTL", "ARTIFACTS_LIST_MAX_KEYS",
		"TEMPORAL_ENABLED", "TEMPORAL_HOST_PORT", "TEMPORAL_NAMESPACE", "TEMPORAL_TASK_QUEUE",
		"ANALYTICS_CLICKHOUSE_ENABLED", "ANALYTICS_CLICKHOUSE_HTTP_URL", "ANALYTICS_MIRROR_OUTBOX_PUBLISHED", "ANALYTICS_PROJECT_OUTBOX_EVENTS",
		"ANALYTICS_CLICKHOUSE_TABLE", "ANALYTICS_PROJECTION_TABLE", "ANALYTICS_MIRROR_MAX_CONCURRENT", "ANALYTICS_INSERT_TIMEOUT", "ANALYTICS_INSERT_MAX_ATTEMPTS",
		"NATS_URL", "NATS_REQUIRED",
		"OUTBOX_PUBLISHER_REQUIRED", "OUTBOX_MAX_ATTEMPTS", "OUTBOX_BACKOFF_MIN", "OUTBOX_BACKOFF_MAX", "OUTBOX_DLQ_ENABLED",
		"PAYMENT_ENV",
		"PRODUCTION_DATABASE_URL", "STAGING_DATABASE_URL", "PRODUCTION_DATABASE_HOST", "STAGING_DATABASE_HOST",
		"STAGING_ALLOW_LOCAL_DATABASE", "DEVELOPMENT_ALLOW_LIVE_PAYMENT",
		"PRODUCTION_SWAGGER_UI_ALLOWED", "PRODUCTION_ALLOW_NONSTANDARD_MQTT_TOPIC_PREFIX",
		"TELEMETRY_MAX_PAYLOAD_BYTES", "TELEMETRY_MAX_POINTS_PER_MESSAGE", "TELEMETRY_MAX_TAGS_PER_MESSAGE",
		"TELEMETRY_PER_MACHINE_MSGS_PER_SEC", "TELEMETRY_PER_MACHINE_BURST", "TELEMETRY_GLOBAL_MAX_INFLIGHT",
		"TELEMETRY_WORKER_CONCURRENCY", "TELEMETRY_DROP_ON_BACKPRESSURE", "TELEMETRY_LEGACY_POSTGRES_INGEST", "TELEMETRY_SUBMIT_WAIT_MS",
		"TELEMETRY_STREAM_MAX_BYTES", "TELEMETRY_STREAM_MAX_AGE", "TELEMETRY_CONSUMER_MAX_ACK_PENDING", "TELEMETRY_CONSUMER_ACK_WAIT",
		"TELEMETRY_CONSUMER_MAX_DELIVER", "TELEMETRY_CONSUMER_BATCH_SIZE", "TELEMETRY_CONSUMER_PULL_TIMEOUT",
		"TELEMETRY_PROJECTION_MAX_CONCURRENCY", "TELEMETRY_PROJECTION_DEDUPE_LRU_SIZE", "TELEMETRY_READINESS_MAX_PENDING",
		"TELEMETRY_READINESS_MAX_PROJECTION_FAIL_STREAK", "TELEMETRY_CONSUMER_LAG_POLL_INTERVAL",
		"TELEMETRY_CLEANUP_ENABLED", "TELEMETRY_CLEANUP_DRY_RUN", "TELEMETRY_RETENTION_DAYS", "TELEMETRY_CRITICAL_RETENTION_DAYS", "TELEMETRY_CLEANUP_BATCH_SIZE",
		"ENABLE_RETENTION_WORKER", "RETENTION_DRY_RUN",
		"ENTERPRISE_RETENTION_CLEANUP_ENABLED", "ENTERPRISE_RETENTION_CLEANUP_DRY_RUN", "ENTERPRISE_RETENTION_CLEANUP_BATCH_SIZE",
		"COMMAND_RETENTION_DAYS", "COMMAND_RECEIPT_RETENTION_DAYS",
		"PAYMENT_WEBHOOK_EVENT_RETENTION_DAYS", "PAYMENT_EVENT_RETENTION_DAYS",
		"OUTBOX_PUBLISHED_RETENTION_DAYS", "OUTBOX_RETENTION_DAYS",
		"OFFLINE_EVENT_RETENTION_DAYS", "INVENTORY_EVENT_RETENTION_DAYS",
		"AUDIT_RETENTION_DAYS", "PROCESSED_MESSAGE_RETENTION_DAYS", "REFRESH_TOKEN_RETENTION_DAYS", "PASSWORD_RESET_TOKEN_RETENTION_DAYS",
		"RETENTION_ALLOW_DESTRUCTIVE_LOCAL",
		"CAPACITY_MAX_TELEMETRY_GRPC_BATCH_EVENTS", "CAPACITY_MAX_TELEMETRY_GRPC_BATCH_BYTES",
		"CAPACITY_MAX_OFFLINE_EVENTS_PER_REQUEST", "CAPACITY_MAX_MEDIA_MANIFEST_ENTRIES",
		"REPORTING_SYNC_MAX_SPAN_DAYS", "REPORTING_EXPORT_MAX_SPAN_DAYS",
		"WORKER_RECOVERY_SCAN_MAX_ITEMS", "WORKER_OUTBOX_DISPATCH_MAX_ITEMS",
		"WORKER_TICK_OUTBOX_DISPATCH", "WORKER_TICK_PAYMENT_TIMEOUT_SCAN", "WORKER_TICK_STUCK_COMMAND_SCAN", "WORKER_CYCLE_BACKOFF_AFTER_FAILURE",
	} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
