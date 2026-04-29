package payments

import (
	"context"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
)

// MockProvider is a non-outbound PSP used for development, tests, and PAYMENT_ENV=sandbox-style stacks.
// It verifies AVF HMAC and parses JSON; remote query and money movement return ErrNotImplemented.
type MockProvider struct {
	key string
}

// NewMockProvider registers under a normalized key (e.g. "mock", "sandbox").
func NewMockProvider(key string) *MockProvider {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		k = "mock"
	}
	return &MockProvider{key: k}
}

func (p *MockProvider) Key() string { return p.key }

func (p *MockProvider) VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error {
	return VerifyCommerceWebhookHMAC(secret, tsHeader, sigHeader, rawBody, skew)
}

func (p *MockProvider) ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error) {
	return ParseCommerceWebhookEventJSON(rawBody)
}

func (p *MockProvider) SupportsQueryPaymentStatus() bool { return false }

func (p *MockProvider) QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	_ = ctx
	_ = lookup
	return domaincommerce.PaymentStatusSnapshot{}, ErrQueryPaymentStatusNotSupported
}

func (p *MockProvider) CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error) {
	_ = ctx
	_ = in
	return CreatePaymentSessionResult{}, ErrNotImplemented
}

func (p *MockProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}

func (p *MockProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}
