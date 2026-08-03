package payments

import (
	"errors"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
)

func TestResolveForPaymentSession_AllowlistAcceptsClientProvider(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv: config.AppEnvDevelopment,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider:  "momo",
			AllowedPaymentProviders: []string{"momo", "zalopay", "vnpay"},
		},
	}
	reg := NewRegistry(cfg)
	reg.Register(&testWiredProvider{key: "zalopay"})
	p, key, err := reg.ResolveForPaymentSession(config.AppEnvDevelopment, "zalopay")
	if err != nil {
		t.Fatal(err)
	}
	if key != "zalopay" || p == nil {
		t.Fatalf("got key=%q provider=%v", key, p)
	}
}

func TestResolveForPaymentSession_AllowlistRejectsUnknown(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv: config.AppEnvDevelopment,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider:  "momo",
			AllowedPaymentProviders: []string{"momo", "zalopay"},
		},
	}
	reg := NewRegistry(cfg)
	_, _, err := reg.ResolveForPaymentSession(config.AppEnvDevelopment, "vnpay")
	if !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("got %v want ErrUnknownProvider", err)
	}
}

func TestNormalizeProviderKey_Aliases(t *testing.T) {
	t.Parallel()
	if got := NormalizeProviderKey("MOMO_QR"); got != "momo" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeProviderKey("viet_qr"); got != "vietqr" {
		t.Fatalf("got %q", got)
	}
}
