package payments

import (
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
)

// PlaceholderLiveProviderKeys are registry keys registered as PlaceholderLiveProvider shells (not production-wired).
// Live VN PSPs (momo/zalopay/vnpay/shopeepay/vietqr) are registered as WiredLiveProvider adapters in NewRegistry.
var PlaceholderLiveProviderKeys = []string{"stripe"}

const (
	// PaymentModeSandbox is non-production PSP simulation (sandbox family keys).
	PaymentModeSandbox = "sandbox"
	// PaymentModeCashOnly is explicit production pilot without live QR/card PSP.
	PaymentModeCashOnly = "cash_only"
	// PaymentModeLivePSP is production with a wired outbound card/QR adapter.
	PaymentModeLivePSP = "live_psp"

	// ProviderStatusSandbox marks in-process sandbox/mock session adapters.
	ProviderStatusSandbox = "sandbox"
	// ProviderStatusWired marks a live adapter that implements outbound sessions.
	ProviderStatusWired = "wired"
	// ProviderStatusPlaceholder marks a named PSP shell without outbound I/O.
	ProviderStatusPlaceholder = "placeholder"
	// ProviderStatusUnavailable marks QR/card sessions as disabled for this deployment.
	ProviderStatusUnavailable = "unavailable"

	// MachineFeatureCashEnabled gates machine-local cash checkout (default enabled when unset).
	MachineFeatureCashEnabled = "commerce.cash_enabled"
	// MachineFeatureQRCardEnabled gates machine-local QR/card checkout when deployment supports it.
	MachineFeatureQRCardEnabled = "commerce.qr_card_enabled"

	// QRCardUnavailableReasonProviderUnavailable is returned to Android when no live PSP is wired.
	QRCardUnavailableReasonProviderUnavailable = "provider_unavailable"
	// QRCardUnavailableReasonMachineMethodDisabled is returned when per-machine config disables all QR methods.
	QRCardUnavailableReasonMachineMethodDisabled = "machine_method_disabled"
)

// WiredLiveProvider marks a production-ready live PSP adapter with outbound session I/O.
type WiredLiveProvider interface {
	PaymentProvider
	LivePaymentWired() bool
}

// DeploymentRuntime describes process-level payment capability (non-secret).
type DeploymentRuntime struct {
	PaymentEnv                string   `json:"payment_env"`
	PaymentMode               string   `json:"payment_mode"`
	CardQRProviderKey         string   `json:"card_qr_provider_key,omitempty"`
	CardQRProviderStatus      string   `json:"card_qr_provider_status"`
	CardQRSessionsAvailable   bool     `json:"card_qr_sessions_available"`
	CashAllowedByDeployment   bool     `json:"cash_allowed_by_deployment"`
	QRCardUnavailableReason   string   `json:"qr_card_unavailable_reason,omitempty"`
	DefaultSessionProviderKey string   `json:"default_session_provider_key,omitempty"`
	EnabledProviders          []string `json:"enabled_providers,omitempty"`
}

// ProviderCapabilityView is per-provider readiness exposed on machine bootstrap.
type ProviderCapabilityView struct {
	Key               string
	Enabled           bool
	Status            string
	Ready             bool
	SessionCreatable  bool
	UnavailableReason string
}

// MachinePaymentMethodsView is the machine-facing payment method matrix for bootstrap/config.
type MachinePaymentMethodsView struct {
	CashEnabled             bool
	QRCardEnabled           bool
	PaymentMode             string
	CardQRProviderKey       string
	CardQRProviderStatus    string
	QRCardUnavailableReason string
	EnabledProviders        []string // allowlist ∩ wired session-capable keys
	Providers               []ProviderCapabilityView
}

// IsPlaceholderLiveProviderKey reports whether key is a known unwired live PSP shell.
func IsPlaceholderLiveProviderKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, ph := range PlaceholderLiveProviderKeys {
		if k == ph {
			return true
		}
	}
	return false
}

// IsPlaceholderLiveProvider reports whether p is a PlaceholderLiveProvider adapter.
func IsPlaceholderLiveProvider(p PaymentProvider) bool {
	_, ok := p.(*PlaceholderLiveProvider)
	return ok
}

func providerSessionStatus(p PaymentProvider) (status string, sessionsAvailable bool) {
	if p == nil {
		return ProviderStatusUnavailable, false
	}
	if sandboxFamilyProviderKey(p.Key()) {
		return ProviderStatusSandbox, true
	}
	if IsPlaceholderLiveProvider(p) {
		return ProviderStatusPlaceholder, false
	}
	if w, ok := p.(WiredLiveProvider); ok {
		if w.LivePaymentWired() {
			return ProviderStatusWired, true
		}
		return ProviderStatusUnavailable, false
	}
	return ProviderStatusWired, true
}

// DeploymentRuntimeFromConfig derives deployment payment capability from config and registry.
func DeploymentRuntimeFromConfig(cfg *config.Config, reg *Registry) DeploymentRuntime {
	out := DeploymentRuntime{
		PaymentEnv:              "",
		PaymentMode:             PaymentModeSandbox,
		CardQRProviderStatus:    ProviderStatusUnavailable,
		CashAllowedByDeployment: true,
	}
	if cfg != nil {
		out.PaymentEnv = strings.ToLower(strings.TrimSpace(cfg.PaymentEnv))
	}
	switch out.PaymentEnv {
	case config.PaymentEnvCashOnly:
		out.PaymentMode = PaymentModeCashOnly
		out.CardQRProviderStatus = ProviderStatusUnavailable
		out.CardQRSessionsAvailable = false
		out.QRCardUnavailableReason = QRCardUnavailableReasonProviderUnavailable
		out.CashAllowedByDeployment = true
		return out
	case config.PaymentEnvLive:
		out.PaymentMode = PaymentModeLivePSP
	default:
		out.PaymentMode = PaymentModeSandbox
	}

	key := ""
	if reg != nil {
		key = strings.ToLower(strings.TrimSpace(reg.defaultPaymentProviderKey))
	}
	if key == "" && cfg != nil {
		key = strings.ToLower(strings.TrimSpace(cfg.Commerce.DefaultPaymentProvider))
	}
	out.DefaultSessionProviderKey = key
	out.CardQRProviderKey = key

	if key == "" {
		if out.PaymentMode == PaymentModeSandbox && cfg != nil && cfg.AppEnv != config.AppEnvProduction {
			out.CardQRProviderKey = "sandbox"
			out.CardQRProviderStatus = ProviderStatusSandbox
			out.CardQRSessionsAvailable = true
			return out
		}
		out.QRCardUnavailableReason = QRCardUnavailableReasonProviderUnavailable
		return out
	}

	var p PaymentProvider
	if reg != nil {
		p = reg.Get(key)
	}
	st, avail := providerSessionStatus(p)
	out.CardQRProviderStatus = st
	out.CardQRSessionsAvailable = avail
	if !avail {
		out.QRCardUnavailableReason = QRCardUnavailableReasonProviderUnavailable
	}
	if out.PaymentMode == PaymentModeLivePSP && !avail {
		out.QRCardUnavailableReason = QRCardUnavailableReasonProviderUnavailable
	}
	out.EnabledProviders = enabledWiredProviders(cfg, reg)
	return out
}

// MachineMethodOverride narrows deployment payment methods for a specific machine.
type MachineMethodOverride struct {
	Configured bool
	Enabled    map[string]bool // method_key -> enabled when Configured is true
}

func machineMethodEnabled(override MachineMethodOverride, key string, deploymentAllowed bool) bool {
	if !deploymentAllowed {
		return false
	}
	if !override.Configured {
		return deploymentAllowed
	}
	return override.Enabled[NormalizeProviderKey(key)]
}

var productionProviderOrder = []string{"momo", "zalopay", "vnpay", "vietqr", "shopeepay"}

func listProviderCapabilities(cfg *config.Config, reg *Registry, deploy DeploymentRuntime, enabled []string, override MachineMethodOverride) []ProviderCapabilityView {
	if reg == nil {
		return nil
	}
	enabledSet := map[string]bool{}
	for _, k := range enabled {
		enabledSet[NormalizeProviderKey(k)] = true
	}
	byKey := map[string]ProviderSummary{}
	for _, s := range reg.ProviderSummaries() {
		byKey[s.Key] = s
	}
	var out []ProviderCapabilityView
	for _, key := range productionProviderOrder {
		sum, ok := byKey[key]
		if !ok {
			continue
		}
		providerEnabled := enabledSet[key]
		if len(enabled) == 0 && cfg != nil && len(cfg.Commerce.AllowedPaymentProviders) == 0 {
			providerEnabled = sum.Wired && sum.SessionAvailable
		}
		providerEnabled = machineMethodEnabled(override, key, providerEnabled)
		ready := sum.Wired && sum.SessionAvailable && deploy.CardQRSessionsAvailable
		sessionCreatable := ready && providerEnabled && deploy.PaymentMode != PaymentModeCashOnly
		reason := ""
		if !sessionCreatable {
			switch {
			case deploy.PaymentMode == PaymentModeCashOnly:
				reason = "cash_only_deployment"
			case !sum.Wired:
				reason = QRCardUnavailableReasonProviderUnavailable
			case !deploy.CardQRSessionsAvailable:
				reason = QRCardUnavailableReasonProviderUnavailable
			case !providerEnabled:
				if override.Configured && !override.Enabled[key] {
					reason = QRCardUnavailableReasonMachineMethodDisabled
				} else {
					reason = "provider_not_enabled"
				}
			}
		}
		out = append(out, ProviderCapabilityView{
			Key:               key,
			Enabled:           providerEnabled,
			Status:            sum.ProviderStatus,
			Ready:             ready,
			SessionCreatable:  sessionCreatable,
			UnavailableReason: reason,
		})
	}
	return out
}

// ResolveMachinePaymentMethods builds machine bootstrap payment config from deployment runtime and optional feature flags.
func ResolveMachinePaymentMethods(cfg *config.Config, reg *Registry, featureFlags map[string]bool) MachinePaymentMethodsView {
	return ResolveMachinePaymentMethodsWithOverride(cfg, reg, featureFlags, MachineMethodOverride{})
}

// ResolveMachinePaymentMethodsWithOverride applies optional per-machine method narrowing on top of deployment capability.
func ResolveMachinePaymentMethodsWithOverride(cfg *config.Config, reg *Registry, featureFlags map[string]bool, override MachineMethodOverride) MachinePaymentMethodsView {
	deploy := DeploymentRuntimeFromConfig(cfg, reg)
	out := MachinePaymentMethodsView{
		PaymentMode:          deploy.PaymentMode,
		CardQRProviderKey:    deploy.CardQRProviderKey,
		CardQRProviderStatus: deploy.CardQRProviderStatus,
	}
	if deploy.QRCardUnavailableReason != "" {
		out.QRCardUnavailableReason = deploy.QRCardUnavailableReason
	}

	cashDefault := deploy.CashAllowedByDeployment
	if deploy.PaymentMode == PaymentModeLivePSP {
		cashDefault = true
	}
	out.CashEnabled = featureFlagDefaultTrue(featureFlags, MachineFeatureCashEnabled, cashDefault)
	if override.Configured {
		out.CashEnabled = out.CashEnabled && machineMethodEnabled(override, "cash", true)
	}

	qrDefault := deploy.CardQRSessionsAvailable
	out.QRCardEnabled = featureFlagDefaultTrue(featureFlags, MachineFeatureQRCardEnabled, qrDefault)
	if !deploy.CardQRSessionsAvailable {
		out.QRCardEnabled = false
		out.QRCardUnavailableReason = QRCardUnavailableReasonProviderUnavailable
	}
	out.EnabledProviders = enabledWiredProviders(cfg, reg)
	out.Providers = listProviderCapabilities(cfg, reg, deploy, out.EnabledProviders, override)
	if override.Configured && !anySessionCreatable(out.Providers) {
		out.QRCardEnabled = false
		if out.QRCardUnavailableReason == "" {
			out.QRCardUnavailableReason = QRCardUnavailableReasonMachineMethodDisabled
		}
	}
	return out
}

func anySessionCreatable(providers []ProviderCapabilityView) bool {
	for _, p := range providers {
		if p.SessionCreatable {
			return true
		}
	}
	return false
}

func enabledWiredProviders(cfg *config.Config, reg *Registry) []string {
	if cfg == nil || reg == nil {
		return nil
	}
	keys := cfg.Commerce.AllowedPaymentProviders
	if len(keys) == 0 {
		if def := NormalizeProviderKey(cfg.Commerce.DefaultPaymentProvider); def != "" {
			keys = []string{def}
		}
	}
	var out []string
	for _, k := range keys {
		k = NormalizeProviderKey(k)
		if k == "" || k == "cash" {
			continue
		}
		p := reg.Get(k)
		if p == nil || IsPlaceholderLiveProvider(p) {
			continue
		}
		if w, ok := p.(WiredLiveProvider); ok && !w.LivePaymentWired() {
			continue
		}
		if sandboxFamilyProviderKey(k) {
			continue
		}
		out = append(out, k)
	}
	return out
}

// EnabledWiredProviders returns deployment allowlist intersect wired session-capable provider keys.
func EnabledWiredProviders(cfg *config.Config, reg *Registry) []string {
	return enabledWiredProviders(cfg, reg)
}

func featureFlagDefaultTrue(flags map[string]bool, key string, defaultVal bool) bool {
	if flags == nil {
		return defaultVal
	}
	if v, ok := flags[key]; ok {
		return v
	}
	return defaultVal
}

// EnabledSessionCreatableProviders returns provider keys the kiosk may use for QR sessions.
func EnabledSessionCreatableProviders(m MachinePaymentMethodsView) []string {
	var out []string
	for _, p := range m.Providers {
		if p.SessionCreatable && strings.TrimSpace(p.Key) != "" {
			out = append(out, p.Key)
		}
	}
	return out
}
