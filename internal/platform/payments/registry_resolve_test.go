package payments

import (
	"errors"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
)

func TestResolveForPaymentSession_sandboxDisallowedInProduction(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv: config.AppEnvProduction,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider: "mock",
		},
	}
	reg := NewRegistry(cfg)
	_, _, err := reg.ResolveForPaymentSession(config.AppEnvProduction, "")
	if !errors.Is(err, ErrSandboxProviderInProduction) {
		t.Fatalf("got %v want ErrSandboxProviderInProduction", err)
	}
}

func TestResolveForPaymentSession_productionRequiresDefaultWhenClientEmpty(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv: config.AppEnvProduction,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider: "",
		},
	}
	reg := NewRegistry(cfg)
	_, _, err := reg.ResolveForPaymentSession(config.AppEnvProduction, "")
	if !errors.Is(err, ErrPaymentProviderRequired) {
		t.Fatalf("got %v want ErrPaymentProviderRequired", err)
	}
}
