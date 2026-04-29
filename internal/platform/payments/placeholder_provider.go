package payments

import (
	"context"
	"fmt"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
)

// PlaceholderLiveProvider is a stub for real PSP integrations (Stripe, MoMo, ZaloPay, VNPay, ...).
// It verifies webhooks like other providers but does not implement outbound session creation until wired.
type PlaceholderLiveProvider struct {
	key string
}

// NewPlaceholderLiveProvider registers an adapter shell for a named PSP.
func NewPlaceholderLiveProvider(key string) *PlaceholderLiveProvider {
	k := strings.ToLower(strings.TrimSpace(key))
	return &PlaceholderLiveProvider{key: k}
}

func (p *PlaceholderLiveProvider) Key() string { return p.key }

func (p *PlaceholderLiveProvider) VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error {
	return VerifyCommerceWebhookHMAC(secret, tsHeader, sigHeader, rawBody, skew)
}

func (p *PlaceholderLiveProvider) ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error) {
	return ParseCommerceWebhookEventJSON(rawBody)
}

func (p *PlaceholderLiveProvider) SupportsQueryPaymentStatus() bool { return false }

func (p *PlaceholderLiveProvider) QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	_ = ctx
	_ = lookup
	return domaincommerce.PaymentStatusSnapshot{}, ErrQueryPaymentStatusNotSupported
}

func (p *PlaceholderLiveProvider) CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error) {
	_ = ctx
	_ = in
	return CreatePaymentSessionResult{}, fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.key)
}

func (p *PlaceholderLiveProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error {
	_ = ctx
	_ = in
	return ErrLiveProviderNotWired
}

func (p *PlaceholderLiveProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error {
	_ = ctx
	_ = in
	return ErrLiveProviderNotWired
}

type cashPaymentProvider struct{}

func (cashPaymentProvider) Key() string { return "cash" }

func (cashPaymentProvider) VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error {
	return VerifyCommerceWebhookHMAC(secret, tsHeader, sigHeader, rawBody, skew)
}

func (cashPaymentProvider) ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error) {
	return ParseCommerceWebhookEventJSON(rawBody)
}

func (cashPaymentProvider) SupportsQueryPaymentStatus() bool { return false }

func (cashPaymentProvider) QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	_ = ctx
	_ = lookup
	return domaincommerce.PaymentStatusSnapshot{}, ErrQueryPaymentStatusNotSupported
}

func (cashPaymentProvider) CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error) {
	_ = ctx
	_ = in
	return CreatePaymentSessionResult{}, fmt.Errorf("%w: use cash confirmation RPC instead of card session", ErrNotImplemented)
}

func (cashPaymentProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}

func (cashPaymentProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}
