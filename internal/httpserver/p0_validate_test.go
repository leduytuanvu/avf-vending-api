package httpserver

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
)

func TestValidateP0HTTPApplication_skippedOutsideProduction(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AppEnv: config.AppEnvDevelopment}
	if err := ValidateP0HTTPApplication(cfg, nil); err != nil {
		t.Fatalf("expected nil outside production, got %v", err)
	}
}

func TestValidateP0HTTPApplication_production_requiresCoreServices(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AppEnv: config.AppEnvProduction}
	if err := ValidateP0HTTPApplication(cfg, nil); err == nil {
		t.Fatal("expected error for nil app")
	}
	if err := ValidateP0HTTPApplication(cfg, &api.HTTPApplication{}); err == nil {
		t.Fatal("expected error for empty app")
	}
}
