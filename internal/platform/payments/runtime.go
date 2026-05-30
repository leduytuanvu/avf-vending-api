package payments

import (
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
)

// PlaceholderLiveProviderKeys are registry keys registered as PlaceholderLiveProvider shells (not production-wired).
var PlaceholderLiveProviderKeys = []string{"stripe", "momo", "zalopay", "vnpay"}

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
)

// WiredLiveProvider marks a production-ready live PSP adapter with outbound session I/O.
type WiredLiveProvider interface {
	PaymentProvider
	LivePaymentWired() bool
}

// DeploymentRuntime describes process-level payment capability (non-secret).
type DeploymentRuntime struct {
	PaymentEnv                string `json:"payment_env"`
	PaymentMode               string `json:"payment_mode"`
	CardQRProviderKey         string `json:"card_qr_provider_key,omitempty"`
	CardQRProviderStatus      string `json:"card_qr_provider_status"`
	CardQRSessionsAvailable   bool   `json:"card_qr_sessions_available"`
	CashAllowedByDeployment   bool   `json:"cash_allowed_by_deployment"`
	QRCardUnavailableReason   string `json:"qr_card_unavailable_reason,omitempty"`
	DefaultSessionProviderKey string `json:"default_session_provider_key,omitempty"`
}

// MachinePaymentMethodsView is the machine-facing payment method matrix for bootstrap/config.
type MachinePaymentMethodsView struct {
	CashEnabled             bool
	QRCardEnabled           bool
	PaymentMode             string
	CardQRProviderKey       string
	CardQRProviderStatus    string
	QRCardUnavailableReason string
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
	return out
}

// ResolveMachinePaymentMethods builds machine bootstrap payment config from deployment runtime and optional feature flags.
func ResolveMachinePaymentMethods(cfg *config.Config, reg *Registry, featureFlags map[string]bool) MachinePaymentMethodsView {
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

	qrDefault := deploy.CardQRSessionsAvailable
	out.QRCardEnabled = featureFlagDefaultTrue(featureFlags, MachineFeatureQRCardEnabled, qrDefault)
	if !deploy.CardQRSessionsAvailable {
		out.QRCardEnabled = false
		out.QRCardUnavailableReason = QRCardUnavailableReasonProviderUnavailable
	}
	return out
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
