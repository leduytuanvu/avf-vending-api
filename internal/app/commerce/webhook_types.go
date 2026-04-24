package commerce

import (
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// ApplyPaymentProviderWebhookInput records a provider-scoped webhook or callback (no outbound PSP calls).
// Idempotency is enforced on (provider, provider_reference) and on (provider, webhook_event_id) when webhook_event_id is set.
type ApplyPaymentProviderWebhookInput struct {
	OrganizationID    uuid.UUID
	OrderID           uuid.UUID
	PaymentID         uuid.UUID
	Provider          string
	ProviderReference string
	// WebhookEventID is an optional PSP delivery identifier; unique per provider when set.
	WebhookEventID         string
	EventType              string
	NormalizedPaymentState string
	Payload                []byte
	ProviderAmountMinor    *int64
	Currency               *string
}

// ApplyPaymentProviderWebhookResult is the outcome of a webhook apply transaction.
type ApplyPaymentProviderWebhookResult struct {
	Replay        bool
	Order         domaincommerce.Order
	Payment       domaincommerce.Payment
	Attempt       PaymentAttemptView
	ProviderRowID int64
}

// CheckoutStatusView aggregates authoritative commerce state for HTTP reads.
type CheckoutStatusView struct {
	Order          domaincommerce.Order
	Vend           domaincommerce.VendSession
	Payment        domaincommerce.Payment
	PaymentPresent bool
}
