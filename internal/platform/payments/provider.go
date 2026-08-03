package payments

import (
	"context"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// CreatePaymentSessionInput is passed to outbound PSP session creators after the payment row exists.
// AmountMinor and Currency are always server-authoritative (never copied from the vending client).
type CreatePaymentSessionInput struct {
	CompanyID           uuid.UUID
	OrderID             uuid.UUID
	PaymentID           uuid.UUID
	AmountMinor         int64
	Currency            string
	IdempotencyKey      string
	MachineExternalCode string // machine_code for AVF/TFO tenant selection
	StoreID             string // terminal/store hint for PSP
	ProviderReference   string // optional pre-assigned ref (legacy order_code); else adapter generates ≤21
	PreferredMethod     string // e.g. vietqr embed preference when using zalopay adapter
}

// CreatePaymentSessionResult carries provider-owned session material returned by the adapter.
// Display fields (QRPayloadOrURL, etc.) MUST originate from the provider (or sandbox shim), never from vending-app input.
type CreatePaymentSessionResult struct {
	ProviderReference   string
	ProviderSessionID   string
	QRPayloadOrURL      string
	PaymentURL          string
	CheckoutURL         string
	ExpiresAt           *time.Time
	ProviderDisplayJSON []byte // optional opaque JSON persisted on payment_attempt.payload for support/debug
}

// CancelPaymentInput cancels or voids an in-flight authorization when supported by the PSP adapter.
type CancelPaymentInput struct {
	CompanyID         uuid.UUID
	PaymentID         uuid.UUID
	ProviderReference string
	IdempotencyKey    string
}

// RefundPaymentInput requests money movement on the PSP (distinct from internal refund ledger rows).
type RefundPaymentInput struct {
	CompanyID         uuid.UUID
	PaymentID         uuid.UUID
	ProviderReference string
	AmountMinor       int64
	Currency          string
	IdempotencyKey    string
}

// PaymentProvider is the enterprise integration surface for a single PSP key (mock, sandbox, or future live adapters).
// Webhook verification uses AVF HMAC; outbound calls may return ErrNotImplemented until wired.
type PaymentProvider interface {
	Key() string

	VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error
	ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error)

	SupportsQueryPaymentStatus() bool
	QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error)

	CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error)
	CancelPayment(ctx context.Context, in CancelPaymentInput) error
	RefundPayment(ctx context.Context, in RefundPaymentInput) error
}
