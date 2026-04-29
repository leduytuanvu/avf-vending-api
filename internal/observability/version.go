package observability

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/version"
)

// VersionPayload is a stable operator-facing build/runtime description.
type VersionPayload struct {
	Name                 string `json:"name"`
	Version              string `json:"version"`
	GitSHA               string `json:"git_sha,omitempty"`
	BuildTime            string `json:"build_time,omitempty"`
	AppEnv               string `json:"app_env"`
	Process              string `json:"process,omitempty"`
	RuntimeRole          string `json:"runtime_role,omitempty"`
	Region               string `json:"region,omitempty"`
	NodeName             string `json:"node_name,omitempty"`
	InstanceID           string `json:"instance_id,omitempty"`
	PublicBaseURL        string `json:"public_base_url,omitempty"`
	MachinePublicBaseURL string `json:"machine_public_base_url,omitempty"`
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
	}
}

func WriteVersionJSON(w http.ResponseWriter, cfg *config.Config) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(BuildVersionPayload(cfg))
}
