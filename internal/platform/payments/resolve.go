package payments

import (
	"fmt"

	"github.com/avf/avf-vending-api/internal/config"
)

// NormalizeProviderKey maps client aliases (e.g. MOMO_QR) onto registry keys.
func NormalizeProviderKey(key string) string {
	return config.NormalizePaymentProviderKey(key)
}

func (r *Registry) allowedProviderSet() map[string]struct{} {
	if r == nil || len(r.allowedPaymentProviders) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(r.allowedPaymentProviders))
	for _, k := range r.allowedPaymentProviders {
		k = NormalizeProviderKey(k)
		if k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

func (r *Registry) providerInAllowlist(key string) bool {
	set := r.allowedProviderSet()
	if set == nil {
		return false
	}
	_, ok := set[NormalizeProviderKey(key)]
	return ok
}

// ResolveForPaymentSession selects the canonical PSP adapter for machine/API-initiated payment sessions.
// When COMMERCE_PAYMENT_PROVIDERS is set, client-declared keys in that allowlist are accepted.
// Otherwise client must match COMMERCE_PAYMENT_PROVIDER when that env is set (legacy single-key mode).
func (r *Registry) ResolveForPaymentSession(appEnv config.AppEnvironment, clientDeclaredProvider string) (PaymentProvider, string, error) {
	if r == nil {
		return nil, "", fmt.Errorf("payments: nil registry")
	}
	def := NormalizeProviderKey(r.defaultPaymentProviderKey)
	client := NormalizeProviderKey(clientDeclaredProvider)
	allow := r.allowedProviderSet()

	var key string
	switch {
	case client != "" && allow != nil:
		if _, ok := allow[client]; !ok {
			return nil, "", fmt.Errorf("%w: %q not in COMMERCE_PAYMENT_PROVIDERS", ErrUnknownProvider, client)
		}
		key = client
	case client != "" && def != "" && client != def:
		return nil, "", fmt.Errorf("%w: got %q want %q", ErrProviderKeyMismatch, client, def)
	case def != "":
		key = def
		if client != "" {
			key = client
		}
	case client != "":
		key = client
	}

	if r.paymentEnv == config.PaymentEnvCashOnly {
		return nil, "", ErrProviderUnavailable
	}
	if key == "" {
		if appEnv == config.AppEnvProduction {
			return nil, "", ErrPaymentProviderRequired
		}
		key = "sandbox"
	}
	if key == "cash" {
		return nil, "", fmt.Errorf("%w: card/QR sessions cannot use provider cash", ErrInvalidCardSessionProvider)
	}
	p := r.Get(key)
	if p == nil {
		return nil, "", fmt.Errorf("%w: %q", ErrUnknownProvider, key)
	}
	if appEnv == config.AppEnvProduction && sandboxFamilyProviderKey(key) {
		return nil, "", fmt.Errorf("%w: %q", ErrSandboxProviderInProduction, key)
	}
	if IsPlaceholderLiveProvider(p) {
		return nil, "", fmt.Errorf("%w: %q", ErrProviderUnavailable, key)
	}
	if w, ok := p.(WiredLiveProvider); ok && !w.LivePaymentWired() {
		return nil, "", fmt.Errorf("%w: %q", ErrProviderUnavailable, key)
	}
	return p, key, nil
}

// AllowedPaymentProviders returns the configured multi-provider allowlist (may be empty).
func (r *Registry) AllowedPaymentProviders() []string {
	if r == nil {
		return nil
	}
	out := make([]string, len(r.allowedPaymentProviders))
	copy(out, r.allowedPaymentProviders)
	return out
}
