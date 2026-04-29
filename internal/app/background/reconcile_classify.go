package background

import "strings"

// classifyProviderNormalizedState maps provider-normalized strings to internal payment.state values.
// Returns ("", false) when the probe does not warrant a deterministic terminal transition from created|authorized.
func classifyProviderNormalizedState(normalized string) (toState string, ok bool) {
	s := strings.ToLower(strings.TrimSpace(normalized))
	switch s {
	case "captured", "succeeded", "settled", "complete", "completed", "paid":
		return "captured", true
	case "failed", "declined", "voided", "rejected":
		return "failed", true
	case "expired", "abandoned":
		return "expired", true
	case "canceled", "cancelled":
		return "canceled", true
	case "partially_refunded", "partial_refund", "partially-refunded":
		// Reconciler applies terminal transitions from created|authorized only; this maps probe hints when used elsewhere.
		return "", false
	default:
		return "", false
	}
}
