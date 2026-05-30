package payments

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/stretchr/testify/require"
)

// Path B (cash-only production pilot): QR/card must never appear as available unless a WiredLiveProvider is registered.

func TestProductionPaymentSafety_CashOnlyHidesQRCardCapabilityFlags(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvCashOnly,
	}
	reg := NewRegistry(cfg)
	methods := ResolveMachinePaymentMethods(cfg, reg, nil)
	require.Equal(t, PaymentModeCashOnly, methods.PaymentMode)
	require.True(t, methods.CashEnabled, "cash-only pilot must expose cash by default")
	require.False(t, methods.QRCardEnabled, "Android must hide QR/card when qr_card_enabled=false")
	require.Equal(t, ProviderStatusUnavailable, methods.CardQRProviderStatus)
	require.Equal(t, QRCardUnavailableReasonProviderUnavailable, methods.QRCardUnavailableReason)
	require.Empty(t, methods.CardQRProviderKey)

	dr := reg.DeploymentRuntime(cfg)
	require.False(t, dr.CardQRSessionsAvailable)
	require.Equal(t, PaymentModeCashOnly, dr.PaymentMode)
	require.True(t, dr.CashAllowedByDeployment)
}

func TestProductionPaymentSafety_PlaceholderProvidersNeverOfferSessions(t *testing.T) {
	t.Parallel()
	for _, key := range PlaceholderLiveProviderKeys {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			cfg := &config.Config{
				AppEnv:     config.AppEnvProduction,
				PaymentEnv: config.PaymentEnvLive,
				Commerce: config.CommerceHTTPConfig{
					DefaultPaymentProvider: key,
				},
			}
			reg := NewRegistry(cfg)
			_, _, err := reg.ResolveForPaymentSession(config.AppEnvProduction, "")
			require.ErrorIs(t, err, ErrProviderUnavailable)

			methods := ResolveMachinePaymentMethods(cfg, reg, nil)
			require.False(t, methods.QRCardEnabled)
			require.Equal(t, ProviderStatusPlaceholder, methods.CardQRProviderStatus)
			require.Equal(t, QRCardUnavailableReasonProviderUnavailable, methods.QRCardUnavailableReason)

			summaries := reg.ProviderSummaries()
			var found *ProviderSummary
			for i := range summaries {
				if summaries[i].Key == key {
					found = &summaries[i]
					break
				}
			}
			require.NotNil(t, found, "registry must list placeholder %q", key)
			require.False(t, found.SessionAvailable)
			require.Equal(t, ProviderStatusPlaceholder, found.ProviderStatus)
		})
	}
}

func TestProductionPaymentSafety_SandboxFamilyBlockedInProduction(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv: config.AppEnvProduction,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider: "mock",
		},
	}
	reg := NewRegistry(cfg)
	_, _, err := reg.ResolveForPaymentSession(config.AppEnvProduction, "")
	require.ErrorIs(t, err, ErrSandboxProviderInProduction)
}

func TestProductionPaymentSafety_FeatureFlagCannotEnableQRCardWithoutWiredPSP(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvCashOnly,
	}
	reg := NewRegistry(cfg)
	methods := ResolveMachinePaymentMethods(cfg, reg, map[string]bool{
		MachineFeatureQRCardEnabled: true,
	})
	require.False(t, methods.QRCardEnabled, "machine flag must not override deployment without wired PSP")
	require.Equal(t, QRCardUnavailableReasonProviderUnavailable, methods.QRCardUnavailableReason)
}
