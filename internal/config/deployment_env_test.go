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
	t.Setenv("APP_NODE_NAME", "stg-node")
	t.Setenv("APP_INSTANCE_ID", "stg-1")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("DATABASE_URL", "postgres://u:p@stgpool.example.com:5432/staging?sslmode=require")
	t.Setenv("NATS_URL", "nats://nats.stg:4222")
	t.Setenv("PAYMENT_ENV", "sandbox")
	t.Setenv("READINESS_STRICT", "true")
	t.Setenv("PUBLIC_BASE_URL", "https://staging-api.ldtv.dev")
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx.stg:1883")
	t.Setenv("MQTT_CLIENT_ID", "stg")
	t.Setenv("MQTT_TOPIC_PREFIX", "avf-staging/devices")
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
