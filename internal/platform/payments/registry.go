package payments

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
)

// ProviderSummary is non-secret metadata for admin/ops (GET /v1/admin/payment/providers).
type ProviderSummary struct {
	Key              string `json:"key"`
	QuerySupported   bool   `json:"query_supported"`
	WebhookProfile   string `json:"webhook_profile"`
	ConfigSource     string `json:"config_source"`
	DefaultForEnv    bool   `json:"default_for_env,omitempty"`
	ActiveSessionKey bool   `json:"active_session_key,omitempty"`
}

// Registry holds registered PaymentProvider adapters and optional HTTP probe fallback for reconciliation.
type Registry struct {
	mu sync.RWMutex

	byKey map[string]PaymentProvider

	httpProbe *HTTPStatusGateway

	defaultPaymentProviderKey string
}

// NewRegistry builds the process-local provider registry from config.
func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{byKey: make(map[string]PaymentProvider)}
	if cfg != nil {
		r.defaultPaymentProviderKey = strings.ToLower(strings.TrimSpace(cfg.Commerce.DefaultPaymentProvider))
	}
	for _, k := range []string{"mock", "sandbox", "test", "psp_fixture", "dev", "psp_grpc_int"} {
		p := NewSandboxProvider(k)
		r.byKey[p.Key()] = p
	}
	for _, k := range []string{"stripe", "momo", "zalopay", "vnpay"} {
		p := NewPlaceholderLiveProvider(k)
		r.byKey[p.Key()] = p
	}
	r.byKey["cash"] = cashPaymentProvider{}
	return r
}

func sandboxFamilyProviderKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "mock", "sandbox", "test", "psp_fixture", "dev", "psp_grpc_int":
		return true
	default:
		return false
	}
}

// ResolveForPaymentSession selects the canonical PSP adapter for machine/API-initiated payment sessions.
// Client-declared provider keys must match COMMERCE_PAYMENT_PROVIDER when that env is set.
func (r *Registry) ResolveForPaymentSession(appEnv config.AppEnvironment, clientDeclaredProvider string) (PaymentProvider, string, error) {
	if r == nil {
		return nil, "", fmt.Errorf("payments: nil registry")
	}
	def := strings.ToLower(strings.TrimSpace(r.defaultPaymentProviderKey))
	client := strings.ToLower(strings.TrimSpace(clientDeclaredProvider))
	if client != "" && def != "" && client != def {
		return nil, "", fmt.Errorf("%w: got %q want %q", ErrProviderKeyMismatch, client, def)
	}
	key := def
	if key == "" {
		key = client
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
	return p, key, nil
}

// SetHTTPProbe configures the optional HTTP payment status probe used by cmd/reconciler (RECONCILER_PAYMENT_PROBE_*).
func (r *Registry) SetHTTPProbe(gw *HTTPStatusGateway) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.httpProbe = gw
}

// Get returns a provider by key or nil.
func (r *Registry) Get(key string) PaymentProvider {
	if r == nil {
		return nil
	}
	k := strings.ToLower(strings.TrimSpace(key))
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byKey[k]
}

// Register adds or replaces a provider (used by tests and future live adapters).
func (r *Registry) Register(p PaymentProvider) {
	if r == nil || p == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byKey == nil {
		r.byKey = make(map[string]PaymentProvider)
	}
	r.byKey[strings.ToLower(strings.TrimSpace(p.Key()))] = p
}

// Health implements bootstrap.PaymentProviderRegistry readiness (always OK when constructed).
func (r *Registry) Health(ctx context.Context) error {
	_ = ctx
	if r == nil {
		return fmt.Errorf("payments: nil registry")
	}
	return nil
}

// ProviderSummaries lists registered providers for admin read APIs.
func (r *Registry) ProviderSummaries() []ProviderSummary {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, 0, len(r.byKey))
	for k := range r.byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]ProviderSummary, 0, len(keys))
	defKey := strings.ToLower(strings.TrimSpace(r.defaultPaymentProviderKey))
	for _, k := range keys {
		p := r.byKey[k]
		if p == nil {
			continue
		}
		out = append(out, ProviderSummary{
			Key:              k,
			QuerySupported:   p.SupportsQueryPaymentStatus() || r.httpProbe != nil,
			WebhookProfile:   "avf_hmac",
			ConfigSource:     "environment",
			DefaultForEnv:    defKey != "" && k == defKey,
			ActiveSessionKey: defKey != "" && k == defKey,
		})
	}
	return out
}

// CompositePaymentGateway routes FetchPaymentStatus to a provider adapter, then optional HTTP probe, else error.
func (r *Registry) CompositePaymentGateway() domaincommerce.PaymentProviderGateway {
	return registryGateway{reg: r}
}

type registryGateway struct {
	reg *Registry
}

func (g registryGateway) FetchPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	if g.reg == nil {
		return domaincommerce.PaymentStatusSnapshot{}, ErrQueryPaymentStatusNotSupported
	}
	key := strings.ToLower(strings.TrimSpace(lookup.Provider))
	p := g.reg.Get(key)
	if p != nil && p.SupportsQueryPaymentStatus() {
		return p.QueryPaymentStatus(ctx, lookup)
	}
	if g.reg.httpProbe != nil {
		return g.reg.httpProbe.FetchPaymentStatus(ctx, lookup)
	}
	return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("%w for provider %q", ErrQueryPaymentStatusNotSupported, lookup.Provider)
}
