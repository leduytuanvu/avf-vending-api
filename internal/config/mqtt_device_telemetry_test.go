package config

import (
	"strings"
	"testing"

	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
)

func TestLoad_Production_requiresNATSURL(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("OTEL_SERVICE_NAME", "avf-api-prod")
	t.Setenv("HTTP_AUTH_JWT_SECRET", "unit-test-production-hs256-secret-key-material")
	t.Setenv(platformnats.EnvNATSURL, "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when NATS_URL missing in production")
	}
	if !strings.Contains(err.Error(), platformnats.EnvNATSURL) {
		t.Fatalf("unexpected error: %v", err)
	}
}
