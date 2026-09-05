package payments

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMachinePaymentMethods_CashOnlyProduction(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvCashOnly,
	}
	reg := NewRegistry(cfg)
	methods := ResolveMachinePaymentMethods(cfg, reg, nil)
	require.Equal(t, PaymentModeCashOnly, methods.PaymentMode)
	require.True(t, methods.CashEnabled)
	require.False(t, methods.QRCardEnabled)
	require.Equal(t, QRCardUnavailableReasonProviderUnavailable, methods.QRCardUnavailableReason)
	require.Equal(t, ProviderStatusUnavailable, methods.CardQRProviderStatus)
}

func TestMachinePaymentMethods_SandboxDev(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvDevelopment,
		PaymentEnv: config.PaymentEnvSandbox,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider: "psp_grpc_int",
		},
	}
	reg := NewRegistry(cfg)
	methods := ResolveMachinePaymentMethods(cfg, reg, nil)
	require.Equal(t, PaymentModeSandbox, methods.PaymentMode)
	require.True(t, methods.QRCardEnabled)
	require.Equal(t, ProviderStatusSandbox, methods.CardQRProviderStatus)
	require.NotEmpty(t, methods.Providers, "Providers must be populated from listProviderCapabilities")
}

func TestMachinePaymentMethods_FeatureFlagDisablesCash(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvCashOnly,
	}
	reg := NewRegistry(cfg)
	methods := ResolveMachinePaymentMethods(cfg, reg, map[string]bool{
		MachineFeatureCashEnabled: false,
	})
	require.False(t, methods.CashEnabled)
}

func TestResolveForPaymentSession_CashOnlyUnavailable(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvCashOnly,
	}
	reg := NewRegistry(cfg)
	_, _, err := reg.ResolveForPaymentSession(config.AppEnvProduction, "")
	require.ErrorIs(t, err, ErrProviderUnavailable)
}

func TestResolveForPaymentSession_PlaceholderUnavailable(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvDevelopment,
		PaymentEnv: config.PaymentEnvLive,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider: "vnpay",
		},
	}
	reg := NewRegistry(cfg)
	_, _, err := reg.ResolveForPaymentSession(config.AppEnvDevelopment, "")
	require.ErrorIs(t, err, ErrProviderUnavailable)
}

func TestMachinePaymentMethods_MultiProviderAllowlist(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvLive,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider:  "momo",
			AllowedPaymentProviders: []string{"momo", "zalopay", "vietqr"},
		},
	}
	cfg.PSP.MoMo.AVF.PartnerCode = "PC"
	cfg.PSP.MoMo.AVF.AccessKey = "ak"
	cfg.PSP.MoMo.AVF.SecretKey = "sk"
	cfg.PSP.MoMo.AVF.Endpoint = "https://test.momo.vn"
	cfg.PSP.ZaloPay.AppID = "1"
	cfg.PSP.ZaloPay.Key1 = "k1"
	cfg.PSP.ZaloPay.Key2 = "k2"
	cfg.PSP.ZaloPay.Endpoint = "https://sb-openapi.zalopay.vn"
	reg := NewRegistry(cfg)
	methods := ResolveMachinePaymentMethods(cfg, reg, nil)
	require.True(t, methods.QRCardEnabled)

	byKey := map[string]ProviderCapabilityView{}
	for _, p := range methods.Providers {
		byKey[p.Key] = p
	}
	for _, key := range []string{"momo", "zalopay", "vietqr"} {
		p, ok := byKey[key]
		require.True(t, ok, "missing provider %s", key)
		require.True(t, p.Enabled, "provider %s should be enabled", key)
		require.True(t, p.Ready, "provider %s should be ready", key)
		require.True(t, p.SessionCreatable, "provider %s should be session_creatable", key)
	}
}

func TestMachinePaymentMethodsWithOverride_NarrowsProviders(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvLive,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider:  "momo",
			AllowedPaymentProviders: []string{"momo", "zalopay", "vietqr"},
		},
	}
	cfg.PSP.MoMo.AVF.PartnerCode = "PC"
	cfg.PSP.MoMo.AVF.AccessKey = "ak"
	cfg.PSP.MoMo.AVF.SecretKey = "sk"
	cfg.PSP.MoMo.AVF.Endpoint = "https://test.momo.vn"
	cfg.PSP.ZaloPay.AppID = "1"
	cfg.PSP.ZaloPay.Key1 = "k1"
	cfg.PSP.ZaloPay.Key2 = "k2"
	cfg.PSP.ZaloPay.Endpoint = "https://sb-openapi.zalopay.vn"
	reg := NewRegistry(cfg)
	methods := ResolveMachinePaymentMethodsWithOverride(cfg, reg, nil, MachineMethodOverride{
		Configured: true,
		Enabled: map[string]bool{
			"cash":    true,
			"momo":    true,
			"vietqr":  true,
			"zalopay": false,
		},
	})
	require.True(t, methods.CashEnabled)
	byKey := map[string]ProviderCapabilityView{}
	for _, p := range methods.Providers {
		byKey[p.Key] = p
	}
	require.True(t, byKey["momo"].SessionCreatable)
	require.False(t, byKey["zalopay"].SessionCreatable)
	require.Equal(t, QRCardUnavailableReasonMachineMethodDisabled, byKey["zalopay"].UnavailableReason)
	require.True(t, byKey["vietqr"].SessionCreatable)
}

func TestMachinePaymentMethodsWithOverride_EmptyUsesDeployment(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvLive,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider:  "momo",
			AllowedPaymentProviders: []string{"momo", "zalopay", "vietqr"},
		},
	}
	cfg.PSP.MoMo.AVF.PartnerCode = "PC"
	cfg.PSP.MoMo.AVF.AccessKey = "ak"
	cfg.PSP.MoMo.AVF.SecretKey = "sk"
	cfg.PSP.MoMo.AVF.Endpoint = "https://test.momo.vn"
	cfg.PSP.ZaloPay.AppID = "1"
	cfg.PSP.ZaloPay.Key1 = "k1"
	cfg.PSP.ZaloPay.Key2 = "k2"
	cfg.PSP.ZaloPay.Endpoint = "https://sb-openapi.zalopay.vn"
	reg := NewRegistry(cfg)
	base := ResolveMachinePaymentMethods(cfg, reg, nil)
	override := ResolveMachinePaymentMethodsWithOverride(cfg, reg, nil, MachineMethodOverride{})
	require.Equal(t, base.QRCardEnabled, override.QRCardEnabled)
	require.Len(t, base.Providers, len(override.Providers))
}
