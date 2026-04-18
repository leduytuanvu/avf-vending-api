package config

import (
	"errors"
	"strings"
)

// TemporalConfig gates optional Temporal SDK wiring for long-running workflows
// (reconciliation follow-up, delayed compensation, human-review escalations).
// Disabled by default; when Enabled, HostPort and TaskQueue are required and the API dials at startup.
type TemporalConfig struct {
	Enabled   bool
	HostPort  string
	Namespace string
	TaskQueue string
}

func loadTemporalConfig() TemporalConfig {
	ns := strings.TrimSpace(getenv("TEMPORAL_NAMESPACE", ""))
	if ns == "" {
		ns = "default"
	}
	return TemporalConfig{
		Enabled:   getenvBool("TEMPORAL_ENABLED", false),
		HostPort:  strings.TrimSpace(getenv("TEMPORAL_HOST_PORT", "")),
		Namespace: ns,
		TaskQueue: strings.TrimSpace(getenv("TEMPORAL_TASK_QUEUE", "")),
	}
}

func (t TemporalConfig) validate() error {
	if !t.Enabled {
		return nil
	}
	if strings.TrimSpace(t.HostPort) == "" {
		return errors.New("config: TEMPORAL_HOST_PORT is required when TEMPORAL_ENABLED=true")
	}
	if strings.TrimSpace(t.TaskQueue) == "" {
		return errors.New("config: TEMPORAL_TASK_QUEUE is required when TEMPORAL_ENABLED=true")
	}
	return nil
}
