package payments

import (
	"encoding/json"
	"strings"
)

// CommerceWebhookEventJSON is the canonical JSON shape for POST .../payments/{id}/webhooks (AVF-normalized PSP callback).
type CommerceWebhookEventJSON struct {
	Provider               string          `json:"provider"`
	ProviderReference      string          `json:"provider_reference"`
	WebhookEventID         string          `json:"webhook_event_id,omitempty"`
	EventType              string          `json:"event_type"`
	NormalizedPaymentState string          `json:"normalized_payment_state"`
	PayloadJSON            json.RawMessage `json:"payload_json"`
	ProviderAmountMinor    *int64          `json:"provider_amount_minor"`
	Currency               *string         `json:"currency"`
}

// ParseCommerceWebhookEventJSON unmarshals and trims string fields for provider webhook application.
func ParseCommerceWebhookEventJSON(raw []byte) (CommerceWebhookEventJSON, error) {
	var wh CommerceWebhookEventJSON
	if err := json.Unmarshal(raw, &wh); err != nil {
		return CommerceWebhookEventJSON{}, err
	}
	wh.Provider = strings.TrimSpace(wh.Provider)
	wh.ProviderReference = strings.TrimSpace(wh.ProviderReference)
	wh.WebhookEventID = strings.TrimSpace(wh.WebhookEventID)
	wh.EventType = strings.TrimSpace(wh.EventType)
	wh.NormalizedPaymentState = strings.TrimSpace(wh.NormalizedPaymentState)
	return wh, nil
}
