package config

import (
	"fmt"
	"os"
	"strings"

	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
)

// ReconcilerConfig controls commerce reconciliation side effects in cmd/reconciler.
// When ActionsEnabled is false, the process runs read-only listing jobs (no PSP HTTP, no refund publishes).
// When ActionsEnabled is true, PaymentProbeURLTemplate and NATS connectivity for refund routing are required.
type ReconcilerConfig struct {
	ActionsEnabled bool
	// DryRun when true: provider probe HTTP still runs, but payment rows are not mutated (ApplyReconciledPaymentTransition dry_run).
	DryRun bool

	PaymentProbeURLTemplate string
	PaymentProbeBearerToken string

	RefundReviewSubject string

	BatchLimit int32
}

func loadReconcilerConfig() ReconcilerConfig {
	return ReconcilerConfig{
		ActionsEnabled:          getenvBool("RECONCILER_ACTIONS_ENABLED", false),
		DryRun:                  getenvBool("RECONCILER_DRY_RUN", false),
		PaymentProbeURLTemplate: strings.TrimSpace(os.Getenv("RECONCILER_PAYMENT_PROBE_URL_TEMPLATE")),
		PaymentProbeBearerToken: strings.TrimSpace(os.Getenv("RECONCILER_PAYMENT_PROBE_BEARER_TOKEN")),
		RefundReviewSubject:     strings.TrimSpace(getenv("RECONCILER_REFUND_REVIEW_SUBJECT", "reconciler.refund_review")),
		BatchLimit:              int32(getenvInt("RECONCILER_BATCH_LIMIT", 200)),
	}
}

// ValidateReconciler enforces wiring rules for cmd/reconciler when actions are enabled.
func ValidateReconciler(c *ReconcilerConfig) error {
	if c == nil {
		return fmt.Errorf("config: nil reconciler config")
	}
	if !c.ActionsEnabled {
		if c.DryRun {
			return fmt.Errorf("config: RECONCILER_DRY_RUN requires RECONCILER_ACTIONS_ENABLED=true")
		}
		return nil
	}
	if strings.TrimSpace(c.PaymentProbeURLTemplate) == "" {
		return fmt.Errorf("config: RECONCILER_ACTIONS_ENABLED=true requires RECONCILER_PAYMENT_PROBE_URL_TEMPLATE")
	}
	if strings.Count(c.PaymentProbeURLTemplate, "%s") != 1 {
		return fmt.Errorf("config: RECONCILER_PAYMENT_PROBE_URL_TEMPLATE must contain exactly one %%s placeholder")
	}
	if strings.TrimSpace(os.Getenv(platformnats.EnvNATSURL)) == "" {
		return fmt.Errorf("config: RECONCILER_ACTIONS_ENABLED=true requires %s for refund review routing", platformnats.EnvNATSURL)
	}
	if strings.TrimSpace(c.RefundReviewSubject) == "" {
		return fmt.Errorf("config: RECONCILER_REFUND_REVIEW_SUBJECT must be non-empty when actions are enabled")
	}
	if c.BatchLimit <= 0 {
		return fmt.Errorf("config: RECONCILER_BATCH_LIMIT must be > 0 when actions are enabled")
	}
	return nil
}
