package commerce

import "strings"

// PaymentCapturedMQTTIdempotencyKey builds the command ledger idempotency key for payment.captured MQTT push.
func PaymentCapturedMQTTIdempotencyKey(paymentID, webhookEventID string) string {
	return "payment.captured:" + strings.TrimSpace(paymentID) + ":" + strings.TrimSpace(webhookEventID)
}
