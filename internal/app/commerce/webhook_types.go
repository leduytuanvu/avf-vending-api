package commerce

import (
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

// ApplyPaymentProviderWebhookInput records a provider-scoped webhook or callback (no outbound PSP calls).
// Idempotency is enforced on (provider, provider_reference) and on (provider, webhook_event_id).
type ApplyPaymentProviderWebhookInput struct {
	OrganizationID    uuid.UUID
	OrderID           uuid.UUID
	PaymentID         uuid.UUID
	Provider          string
	ProviderReference string
	// WebhookEventID is required for PSP deliveries (unique per provider); persisted for replay detection.
	WebhookEventID         string
	EventType              string
	NormalizedPaymentState string
	Payload                []byte
	ProviderAmountMinor    *int64
	Currency               *string
	// WebhookValidationStatus is persisted on payment_provider_events.validation_status (hmac_verified | unsigned_development).
	WebhookValidationStatus string
	// ProviderMetadata is optional non-secret JSON metadata stored on payment_provider_events.provider_metadata.
	ProviderMetadata []byte

	// Optional transactional outbox emission. When OutboxTopic is set, persistence
	// inserts this event in the same DB transaction as the webhook state change.
	OutboxTopic          string
	OutboxEventType      string
	OutboxPayload        []byte
	OutboxAggregateType  string
	OutboxAggregateID    uuid.UUID
	OutboxIdempotencyKey string
}

// PaymentWebhookAppliedEvent is emitted to WebhookAppliedHook after a successful webhook apply (including idempotent replays).
type PaymentWebhookAppliedEvent struct {
	OrganizationID uuid.UUID
	OrderID        uuid.UUID
	PaymentID      uuid.UUID
	Replay         bool
	Provider       string
	WebhookEventID string
	Validation     string
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
