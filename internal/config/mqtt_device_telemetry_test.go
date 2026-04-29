package config

import (
	"strings"
	"testing"

	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
)

func TestLoad_Production_requiresNATSURL(t *testing.T) {
	setMinimalProductionLoadEnv(t)
	t.Setenv(platformnats.EnvNATSURL, "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when NATS_URL missing in production")
	}
	if !strings.Contains(err.Error(), platformnats.EnvNATSURL) {
		t.Fatalf("unexpected error: %v", err)
	}
}
