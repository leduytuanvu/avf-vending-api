package observability

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/avf/avf-vending-api/internal/version"
)

// PaymentRuntimeInfo exposes non-secret payment deployment mode for operators and Android config probes.
type PaymentRuntimeInfo struct {
	PaymentEnv              string `json:"payment_env"`
	PaymentMode             string `json:"payment_mode"`
	CardQRProviderKey       string `json:"card_qr_provider_key,omitempty"`
	CardQRProviderStatus    string `json:"card_qr_provider_status"`
	CardQRSessionsAvailable bool   `json:"card_qr_sessions_available"`
	CashAllowedByDeployment bool   `json:"cash_allowed_by_deployment"`
	QRCardUnavailableReason string `json:"qr_card_unavailable_reason,omitempty"`
}

// VersionPayload is a stable operator-facing build/runtime description.
type VersionPayload struct {
	Name                 string              `json:"name"`
	Version              string              `json:"version"`
	GitSHA               string              `json:"git_sha,omitempty"`
	BuildTime            string              `json:"build_time,omitempty"`
	AppEnv               string              `json:"app_env"`
	Process              string              `json:"process,omitempty"`
	RuntimeRole          string              `json:"runtime_role,omitempty"`
	Region               string              `json:"region,omitempty"`
	NodeName             string              `json:"node_name,omitempty"`
	InstanceID           string              `json:"instance_id,omitempty"`
	PublicBaseURL        string              `json:"public_base_url,omitempty"`
	MachinePublicBaseURL string              `json:"machine_public_base_url,omitempty"`
	PaymentRuntime       *PaymentRuntimeInfo `json:"payment_runtime,omitempty"`
}

func BuildPaymentRuntimeInfo(cfg *config.Config) *PaymentRuntimeInfo {
	if cfg == nil {
		return nil
	}
	reg := platformpayments.NewRegistry(cfg)
	dr := reg.DeploymentRuntime(cfg)
	return &PaymentRuntimeInfo{
		PaymentEnv:              dr.PaymentEnv,
		PaymentMode:             dr.PaymentMode,
		CardQRProviderKey:       dr.CardQRProviderKey,
		CardQRProviderStatus:    dr.CardQRProviderStatus,
		CardQRSessionsAvailable: dr.CardQRSessionsAvailable,
		CashAllowedByDeployment: dr.CashAllowedByDeployment,
		QRCardUnavailableReason: dr.QRCardUnavailableReason,
	}
}

func BuildVersionPayload(cfg *config.Config) VersionPayload {
	if cfg == nil {
		return VersionPayload{Name: version.Name}
	}
	return VersionPayload{
		Name:                 version.Name,
		Version:              cfg.Build.Version,
		GitSHA:               cfg.Build.GitSHA,
		BuildTime:            cfg.Build.BuildTime,
		AppEnv:               string(cfg.AppEnv),
		Process:              strings.TrimSpace(cfg.ProcessName),
		RuntimeRole:          cfg.Runtime.EffectiveRuntimeRole(cfg.ProcessName),
		Region:               cfg.Runtime.Region,
		NodeName:             cfg.Runtime.NodeName,
		InstanceID:           cfg.Runtime.InstanceID,
		PublicBaseURL:        cfg.Runtime.PublicBaseURL,
		MachinePublicBaseURL: cfg.Runtime.MachinePublicBaseURL,
		PaymentRuntime:       BuildPaymentRuntimeInfo(cfg),
	}
}

func WriteVersionJSON(w http.ResponseWriter, cfg *config.Config) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(BuildVersionPayload(cfg))
}
