package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// SandboxProvider simulates a PSP for development, CI, and staging. It never calls the network.
// CreatePaymentSession is deterministic per (payment_id, idempotency_key).
type SandboxProvider struct {
	key string
}

// NewSandboxProvider registers an in-memory PSP implementation.
func NewSandboxProvider(key string) *SandboxProvider {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		k = "sandbox"
	}
	return &SandboxProvider{key: k}
}

func (p *SandboxProvider) Key() string { return p.key }

func (p *SandboxProvider) VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error {
	return VerifyCommerceWebhookHMAC(secret, tsHeader, sigHeader, rawBody, skew)
}

func (p *SandboxProvider) ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error) {
	return ParseCommerceWebhookEventJSON(rawBody)
}

func (p *SandboxProvider) SupportsQueryPaymentStatus() bool { return false }

func (p *SandboxProvider) QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	_ = ctx
	_ = lookup
	return domaincommerce.PaymentStatusSnapshot{}, ErrQueryPaymentStatusNotSupported
}

func (p *SandboxProvider) CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error) {
	_ = ctx
	if in.PaymentID == uuid.Nil || in.OrderID == uuid.Nil {
		return CreatePaymentSessionResult{}, errors.New("payments sandbox: payment_id and order_id required")
	}
	idem := strings.TrimSpace(in.IdempotencyKey)
	if idem == "" {
		return CreatePaymentSessionResult{}, errors.New("payments sandbox: idempotency_key required")
	}
	if in.AmountMinor < 0 {
		return CreatePaymentSessionResult{}, errors.New("payments sandbox: amount_minor invalid")
	}
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	if len(cur) != 3 {
		return CreatePaymentSessionResult{}, errors.New("payments sandbox: currency must be ISO 4217")
	}
	ref := fmt.Sprintf("sb_%s", in.PaymentID.String())
	sessID := fmt.Sprintf("sess_%s_%s", in.PaymentID.String(), hash10(idem))
	exp := time.Now().UTC().Add(15 * time.Minute)
	qr := fmt.Sprintf("https://sandbox.pay.avf.invalid/qr/%s", ref)
	payURL := fmt.Sprintf("https://sandbox.pay.avf.invalid/pay/%s", ref)
	display, _ := json.Marshal(map[string]any{
		"environment":          "sandbox",
		"provider_key":         p.key,
		"provider_reference":   ref,
		"provider_session_id":  sessID,
		"qr_url":               qr,
		"payment_url":          payURL,
		"amount_minor":         in.AmountMinor,
		"currency":             cur,
		"expires_at":           exp.Format(time.RFC3339Nano),
		"psp_idempotency_hint": idem,
	})
	return CreatePaymentSessionResult{
		ProviderReference:   ref,
		ProviderSessionID:   sessID,
		QRPayloadOrURL:      qr,
		PaymentURL:          payURL,
		CheckoutURL:         payURL,
		ExpiresAt:           &exp,
		ProviderDisplayJSON: display,
	}, nil
}

func (p *SandboxProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}

func (p *SandboxProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}

func hash10(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%08x", h.Sum32())
}
