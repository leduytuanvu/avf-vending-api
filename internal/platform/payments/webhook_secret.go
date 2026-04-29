package payments

import "strings"

// ResolveWebhookHMACSecret picks a signing secret: per-provider map (lowercase key) first, then global secret.
// Never log returned values.
func ResolveWebhookHMACSecret(globalSecret string, perProvider map[string]string, providerPeek string) string {
	k := strings.ToLower(strings.TrimSpace(providerPeek))
	if k != "" && perProvider != nil {
		// Exact key match on lowercased provider label from JSON.
		if s := strings.TrimSpace(perProvider[k]); s != "" {
			return s
		}
	}
	return strings.TrimSpace(globalSecret)
}
