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
