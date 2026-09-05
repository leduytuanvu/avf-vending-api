package payments

// SameWebhookAdapterFamily reports whether two provider registry keys share one native
// PSP callback adapter (e.g. vietqr sessions use the ZaloPay HTTP API and callback URL).
func SameWebhookAdapterFamily(eventProvider, storedProvider string) bool {
	a := NormalizeProviderKey(eventProvider)
	b := NormalizeProviderKey(storedProvider)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	switch {
	case a == "zalopay" && b == "vietqr":
		return true
	case a == "vietqr" && b == "zalopay":
		return true
	default:
		return false
	}
}
