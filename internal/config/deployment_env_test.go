package config

import (
	"os"
	"strings"
	"testing"
)

func minimalStaging(t *testing.T) {
	t.Helper()
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("HTTP_ADDR", ":0")
	t.Setenv("OTEL_SERVICE_NAME", "test")
	t.Setenv("HTTP_AUTH_JWT_SECRET", "staging-test-jwt-secret-not-repeated-32+chars")
	t.Setenv("APP_REGION", "ap-southeast-1")
	t.Setenv("APP_NODE_NAME", "stg-node")
	t.Setenv("APP_INSTANCE_ID", "stg-1")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("DATABASE_URL", "postgres://u:p@stgpool.example.com:5432/staging?sslmode=require")
	t.Setenv("NATS_URL", "nats://nats.stg:4222")
	t.Setenv("PAYMENT_ENV", "sandbox")
	t.Setenv("READINESS_STRICT", "true")
	t.Setenv("PUBLIC_BASE_URL", "https://staging-api.ldtv.dev")
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx.stg:1883")
	t.Setenv("MQTT_TLS_ENABLED", "true")
	t.Setenv("MQTT_CLIENT_ID", "stg")
	t.Setenv("MQTT_TOPIC_PREFIX", "avf-staging/devices")
	t.Setenv("COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET", "staging-test-commerce-webhook-secret-at-least-32chars-xx")
	t.Setenv("HTTP_CORS_ALLOWED_ORIGINS", "")
}

func TestLoad_Development_AllowsLocalhostDatabase(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://u:p@127.0.0.1:5432/dev?sslmode=disable")
	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_Staging_RejectsLocalhostDBByDefault(t *testing.T) {
	minimalStaging(t)
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/st?sslmode=disable")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "localhost") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Staging_AcceptNonlocalDB(t *testing.T) {
	minimalStaging(t)
	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_StagingRequiresNATSByDefault(t *testing.T) {
	minimalStaging(t)
	t.Setenv("NATS_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "NATS_URL") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_DevelopmentCanDisableNATSRuntime(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("NATS_URL", "")
	t.Setenv("NATS_REQUIRED", "false")
	t.Setenv("OUTBOX_PUBLISHER_REQUIRED", "false")
	t.Setenv("API_REQUIRE_NATS_RUNTIME", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NATS.Required || cfg.Outbox.PublisherRequired {
		t.Fatalf("expected optional local nats/outbox, got nats=%v outbox=%v", cfg.NATS.Required, cfg.Outbox.PublisherRequired)
	}
}

func TestLoad_OutboxExplicitConfig(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("NATS_URL", "nats://127.0.0.1:4222")
	t.Setenv("OUTBOX_PUBLISHER_REQUIRED", "true")
	t.Setenv("OUTBOX_MAX_ATTEMPTS", "5")
	t.Setenv("OUTBOX_BACKOFF_MIN", "2s")
	t.Setenv("OUTBOX_BACKOFF_MAX", "30s")
	t.Setenv("OUTBOX_DLQ_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Outbox.PublisherRequired || cfg.Outbox.MaxAttempts != 5 || cfg.Outbox.BackoffMin.String() != "2s" ||
		cfg.Outbox.BackoffMax.String() != "30s" || cfg.Outbox.DLQEnabled {
		t.Fatalf("unexpected outbox config: %+v", cfg.Outbox)
	}
}

// stagingProdHostSeparationErr is the message when staging DATABASE_URL hostname must not be treated as the production ref.
// It references PRODUCTION_* env vars in uppercase; do not use case-sensitive "production" substring.
func errorIsStagingProdHostSeparation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "staging DATABASE_URL host must not match") &&
		strings.Contains(s, "PRODUCTION_DATABASE_HOST")
}

func TestLoad_Staging_RejectsProdDatabaseHost(t *testing.T) {
	t.Run("PRODUCTION_DATABASE_HOST_matches_staging_DATABASE_URL_host", func(t *testing.T) {
		minimalStaging(t)
		t.Setenv("PRODUCTION_DATABASE_HOST", "stgpool.example.com")
		_, err := Load()
		if err == nil {
			t.Fatal("expected error")
		}
		if !errorIsStagingProdHostSeparation(err) {
			t.Fatalf("expected host-separation error, got: %v", err)
		}
	})
	t.Run("PRODUCTION_DATABASE_URL_same_host_different_path_not_equal_string", func(t *testing.T) {
		// Not caught by the exact-string equality check; host from PRODUCTION_DATABASE_URL must not match staging host.
		minimalStaging(t)
		t.Setenv("PRODUCTION_DATABASE_URL", "postgres://u:p@stgpool.example.com:5432/prod?sslmode=require")
		_, err := Load()
		if err == nil {
			t.Fatal("expected error")
		}
		if !errorIsStagingProdHostSeparation(err) {
			t.Fatalf("expected host-separation error, got: %v", err)
		}
	})
}

func TestLoad_Staging_RejectsProdMQTTPrefix(t *testing.T) {
	minimalStaging(t)
	t.Setenv("MQTT_TOPIC_PREFIX", "avf/devices")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Staging_RejectsLivePayment(t *testing.T) {
	minimalStaging(t)
	t.Setenv("PAYMENT_ENV", "live")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Production_RejectsLocalhost(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("DATABASE_URL", "postgres://u:p@127.0.0.1:5432/p?sslmode=disable")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Production_RejectsStagingHost(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("STAGING_DATABASE_HOST", "db.prod.internal")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Production_RejectsSandboxPayment(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("PAYMENT_ENV", "sandbox")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Production_RejectsMockCommercePaymentProvider(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("COMMERCE_PAYMENT_PROVIDER", "mock")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COMMERCE_PAYMENT_PROVIDER") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RequiresCommercePaymentProvider(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("COMMERCE_PAYMENT_PROVIDER")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COMMERCE_PAYMENT_PROVIDER") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsMissingRedis(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("REDIS_ADDR")
	_ = os.Unsetenv("REDIS_URL")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "REDIS_ADDR") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_AllowsMissingRedisWithDocumentedOverride(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("REDIS_ADDR")
	_ = os.Unsetenv("REDIS_URL")
	t.Setenv("PRODUCTION_ALLOW_MISSING_REDIS", "true")
	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_Production_RejectsSwaggerEnabledWithoutAllow(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("HTTP_SWAGGER_UI_ENABLED", "true")
	_ = os.Unsetenv("PRODUCTION_SWAGGER_UI_ALLOWED")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_Development_LivePaymentRequiresOverride(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("PAYMENT_ENV", "live")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	t.Setenv("DEVELOPMENT_ALLOW_LIVE_PAYMENT", "true")
	_, err = Load()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_Staging_EqualToProductionDBURL(t *testing.T) {
	// Fails first on exact-string match (before host-from-URL comparison).
	minimalStaging(t)
	prod := "postgres://u:p@stgpool.example.com:5432/same?sslmode=require"
	t.Setenv("PRODUCTION_DATABASE_URL", prod)
	t.Setenv("DATABASE_URL", prod)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must not equal") || !strings.Contains(err.Error(), "PRODUCTION_DATABASE_URL") {
		t.Fatalf("expected exact PRODUCTION_DATABASE_URL equality error, got: %v", err)
	}
}

func TestLoad_Staging_DefaultMaxConns(t *testing.T) {
	minimalStaging(t)
	_ = os.Unsetenv("DATABASE_MAX_CONNS")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Postgres.MaxConns != 5 {
		t.Fatalf("max conns: got %d want 5 (staging default)", cfg.Postgres.MaxConns)
	}
}

func TestLoad_Staging_RejectsShortHS256JWTSecret(t *testing.T) {
	minimalStaging(t)
	t.Setenv("HTTP_AUTH_JWT_SECRET", strings.Repeat("a", 31))
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP_AUTH_JWT_SECRET") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Staging_RejectsRedisTLSInsecureSkipVerify(t *testing.T) {
	minimalStaging(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("REDIS_TLS_ENABLED", "true")
	t.Setenv("REDIS_TLS_INSECURE_SKIP_VERIFY", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "REDIS_TLS_INSECURE_SKIP_VERIFY") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsRedisTLSInsecureSkipVerify(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("REDIS_TLS_ENABLED", "true")
	t.Setenv("REDIS_TLS_INSECURE_SKIP_VERIFY", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "REDIS_TLS_INSECURE_SKIP_VERIFY") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Development_AllowsRedisTLSInsecureSkipVerify(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("REDIS_TLS_ENABLED", "true")
	t.Setenv("REDIS_TLS_INSECURE_SKIP_VERIFY", "true")
	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_Production_RejectsReadinessStrictFalse(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("READINESS_STRICT", "false")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "READINESS_STRICT") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsDocumentationJWTSecret(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("HTTP_AUTH_JWT_SECRET", "dev-change-me-use-long-random-string")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP_AUTH_JWT_SECRET") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsTCPMQTTBrokerNonLocalhost(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx.prod.example.com:1883")
	t.Setenv("MQTT_TLS_ENABLED", "false")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "MQTT") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsOpenAPIJSONExplicitWithoutAllow(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("HTTP_OPENAPI_JSON_ENABLED", "true")
	_ = os.Unsetenv("PRODUCTION_OPENAPI_JSON_ALLOWED")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "PRODUCTION_OPENAPI_JSON_ALLOWED") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Development_AcceptsDocumentationJWTSecretExample(t *testing.T) {
	setMinimalValidLoadEnv(t)
	t.Setenv("HTTP_AUTH_JWT_SECRET", "dev-change-me-use-long-random-string")
	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoad_Production_RequiresMQTTPublisherWhenMQTTClientIDAPISet(t *testing.T) {
	t.Run("missing_flag", func(t *testing.T) {
		setMinimalProductionLoadEnv(t)
		t.Setenv("MQTT_CLIENT_ID_API", "avf-prod-api-publisher")
		_ = os.Unsetenv("API_REQUIRE_MQTT_PUBLISHER")
		_, err := Load()
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "API_REQUIRE_MQTT_PUBLISHER") {
			t.Fatalf("unexpected: %v", err)
		}
	})
	t.Run("with_flag", func(t *testing.T) {
		setMinimalProductionLoadEnv(t)
		t.Setenv("MQTT_CLIENT_ID_API", "avf-prod-api-publisher")
		t.Setenv("API_REQUIRE_MQTT_PUBLISHER", "true")
		_, err := Load()
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestLoad_Production_RejectsTriviallyWeakJWTSecret(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("HTTP_AUTH_JWT_SECRET", strings.Repeat("z", 40))
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP_AUTH_JWT_SECRET") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RequiresExplicitHTTPAuthModeEnv(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("HTTP_AUTH_MODE")
	_ = os.Unsetenv("USER_JWT_MODE")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP_AUTH_MODE") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RequiresExplicitMachineJWTModeEnv(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("MACHINE_JWT_MODE")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "MACHINE_JWT_MODE") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsMQTTWithoutCredentials(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	_ = os.Unsetenv("MQTT_USERNAME")
	_ = os.Unsetenv("MQTT_PASSWORD")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "MQTT_USERNAME") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_ArtifactsEnabledRequiresObjectStorage(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("API_ARTIFACTS_ENABLED", "true")
	_ = os.Unsetenv("OBJECT_STORAGE_BUCKET")
	_ = os.Unsetenv("OBJECT_STORAGE_PUBLIC_BASE_URL")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "OBJECT_STORAGE_BUCKET") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_Production_RejectsOutboxRequiredWithoutNATSURL(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv("NATS_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "NATS_URL") {
		t.Fatalf("unexpected: %v", err)
	}
}
