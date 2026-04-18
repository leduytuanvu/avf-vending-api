package background

import "strings"

// classifyProviderNormalizedState maps provider-normalized strings to internal payment.state values.
// Returns ("", false) when the probe does not warrant a deterministic terminal transition from created|authorized.
func classifyProviderNormalizedState(normalized string) (toState string, ok bool) {
	s := strings.ToLower(strings.TrimSpace(normalized))
	switch s {
	case "captured", "succeeded", "settled", "complete", "completed", "paid":
		return "captured", true
	case "failed", "declined", "canceled", "cancelled", "voided", "rejected":
		return "failed", true
	default:
		return "", false
	}
}
