package payments

import (
	"context"
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type testWiredProvider struct {
	key string
}

func (p *testWiredProvider) Key() string { return p.key }
func (p *testWiredProvider) LivePaymentWired() bool { return true }
func (p *testWiredProvider) VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error {
	return VerifyCommerceWebhookHMAC(secret, tsHeader, sigHeader, rawBody, skew)
}
func (p *testWiredProvider) ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error) {
	return ParseCommerceWebhookEventJSON(rawBody)
}
func (p *testWiredProvider) SupportsQueryPaymentStatus() bool { return true }
func (p *testWiredProvider) QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	return domaincommerce.PaymentStatusSnapshot{}, nil
}
func (p *testWiredProvider) CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error) {
	return CreatePaymentSessionResult{ProviderReference: "wired-" + in.PaymentID.String()}, nil
}
func (p *testWiredProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error { return nil }
func (p *testWiredProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error { return nil }

func TestResolveForPaymentSession_WiredLiveProviderAllowed(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvLive,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider: "pilot_psp",
		},
	}
	reg := NewRegistry(cfg)
	reg.Register(&testWiredProvider{key: "pilot_psp"})
	p, key, err := reg.ResolveForPaymentSession(config.AppEnvProduction, "")
	require.NoError(t, err)
	require.Equal(t, "pilot_psp", key)
	require.NotNil(t, p)
	_, err = p.CreatePaymentSession(context.Background(), CreatePaymentSessionInput{
		OrderID:        uuid.New(),
		PaymentID:      uuid.New(),
		AmountMinor:    100,
		Currency:       "USD",
		IdempotencyKey: "k1",
	})
	require.NoError(t, err)
}

func TestDeploymentRuntime_LivePlaceholderUnavailable(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:     config.AppEnvProduction,
		PaymentEnv: config.PaymentEnvLive,
		Commerce: config.CommerceHTTPConfig{
			DefaultPaymentProvider: "momo",
		},
	}
	reg := NewRegistry(cfg)
	dr := reg.DeploymentRuntime(cfg)
	require.Equal(t, PaymentModeLivePSP, dr.PaymentMode)
	require.False(t, dr.CardQRSessionsAvailable)
	require.Equal(t, ProviderStatusPlaceholder, dr.CardQRProviderStatus)
	require.Equal(t, QRCardUnavailableReasonProviderUnavailable, dr.QRCardUnavailableReason)
}
