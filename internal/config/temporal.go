package config

import (
	"errors"
	"strings"
)

// TemporalConfig gates optional Temporal SDK wiring for long-running workflows.
// Disabled by default; when Enabled, HostPort and TaskQueue are required.
type TemporalConfig struct {
	Enabled bool
	// Schedule* flags gate which business paths enqueue Temporal workflows.
	SchedulePaymentPendingTimeout  bool
	ScheduleVendFailureFollowUp    bool
	ScheduleRefundOrchestration    bool
	ScheduleManualReviewEscalation bool
	HostPort                       string
	Namespace                      string
	TaskQueue                      string
}

func loadTemporalConfig() TemporalConfig {
	ns := strings.TrimSpace(getenv("TEMPORAL_NAMESPACE", ""))
	if ns == "" {
		ns = "default"
	}
	return TemporalConfig{
		Enabled:                        getenvBool("TEMPORAL_ENABLED", false),
		SchedulePaymentPendingTimeout:  getenvBool("TEMPORAL_SCHEDULE_PAYMENT_PENDING_TIMEOUT", false),
		ScheduleVendFailureFollowUp:    getenvBool("TEMPORAL_SCHEDULE_VEND_FAILURE_FOLLOW_UP", false),
		ScheduleRefundOrchestration:    getenvBool("TEMPORAL_SCHEDULE_REFUND_ORCHESTRATION", false),
		ScheduleManualReviewEscalation: getenvBool("TEMPORAL_SCHEDULE_MANUAL_REVIEW_ESCALATION", false),
		HostPort:                       strings.TrimSpace(getenv("TEMPORAL_HOST_PORT", "")),
		Namespace:                      ns,
		TaskQueue:                      strings.TrimSpace(getenv("TEMPORAL_TASK_QUEUE", "")),
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
