package observability

import (
	"encoding/json"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/stretchr/testify/require"
)

func TestBuildVersionPayload_IncludesPaymentRuntime(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvCashOnly,
	}
	payload := BuildVersionPayload(cfg)
	require.NotNil(t, payload.PaymentRuntime)
	require.Equal(t, config.PaymentEnvCashOnly, payload.PaymentRuntime.PaymentEnv)
	require.Equal(t, platformpayments.PaymentModeCashOnly, payload.PaymentRuntime.PaymentMode)
	require.False(t, payload.PaymentRuntime.CardQRSessionsAvailable)
	require.True(t, payload.PaymentRuntime.CashAllowedByDeployment)
	require.Equal(t, platformpayments.ProviderStatusUnavailable, payload.PaymentRuntime.CardQRProviderStatus)
	require.Equal(t, platformpayments.QRCardUnavailableReasonProviderUnavailable, payload.PaymentRuntime.QRCardUnavailableReason)

	b, err := json.Marshal(payload)
	require.NoError(t, err)
	require.Contains(t, string(b), `"payment_runtime"`)
	require.Contains(t, string(b), `"cash_only"`)
}
