package config

import (
	"os"
	"testing"

	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
)

func TestValidateReconciler_actionsDisabled_allowsEmptyDeps(t *testing.T) {
	t.Parallel()
	c := &ReconcilerConfig{ActionsEnabled: false, DryRun: false}
	if err := ValidateReconciler(c); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateReconciler_dryRunWithoutActionsFails(t *testing.T) {
	t.Parallel()
	c := &ReconcilerConfig{ActionsEnabled: false, DryRun: true}
	if err := ValidateReconciler(c); err == nil {
		t.Fatal("expected error when dry run without actions")
	}
}

func TestValidateReconciler_actionsEnabled_missingProbeTemplate(t *testing.T) {
	t.Parallel()
	c := &ReconcilerConfig{
		ActionsEnabled:          true,
		PaymentProbeURLTemplate: "",
		RefundReviewSubject:     "reconciler.refund_review",
		BatchLimit:              50,
	}
	if err := ValidateReconciler(c); err == nil {
		t.Fatal("expected error for missing probe template")
	}
}

func TestValidateReconciler_actionsEnabled_badPlaceholderCount(t *testing.T) {
	t.Parallel()
	c := &ReconcilerConfig{
		ActionsEnabled:          true,
		PaymentProbeURLTemplate: "http://x/%s/%s",
		RefundReviewSubject:     "reconciler.refund_review",
		BatchLimit:              50,
	}
	if err := ValidateReconciler(c); err == nil {
		t.Fatal("expected error for wrong URL template placeholder count")
	}
}

func TestValidateReconciler_actionsEnabled_missingNATSURL(t *testing.T) {
	t.Parallel()
	prev := os.Getenv(platformnats.EnvNATSURL)
	_ = os.Unsetenv(platformnats.EnvNATSURL)
	t.Cleanup(func() {
		if prev == "" {
			_ = os.Unsetenv(platformnats.EnvNATSURL)
		} else {
			_ = os.Setenv(platformnats.EnvNATSURL, prev)
		}
	})

	c := &ReconcilerConfig{
		ActionsEnabled:          true,
		PaymentProbeURLTemplate: "http://localhost/probe/%s",
		RefundReviewSubject:     "reconciler.refund_review",
		BatchLimit:              50,
	}
	if err := ValidateReconciler(c); err == nil {
		t.Fatal("expected error when NATS_URL missing")
	}
}

func TestValidateReconciler_actionsEnabled_ok(t *testing.T) {
	t.Parallel()
	prev := os.Getenv(platformnats.EnvNATSURL)
	_ = os.Setenv(platformnats.EnvNATSURL, "nats://127.0.0.1:4222")
	t.Cleanup(func() {
		if prev == "" {
			_ = os.Unsetenv(platformnats.EnvNATSURL)
		} else {
			_ = os.Setenv(platformnats.EnvNATSURL, prev)
		}
	})

	c := &ReconcilerConfig{
		ActionsEnabled:          true,
		PaymentProbeURLTemplate: "http://localhost/probe/%s",
		RefundReviewSubject:     "reconciler.refund_review",
		BatchLimit:              50,
	}
	if err := ValidateReconciler(c); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestValidateReconciler_actionsEnabled_emptyRefundSubject(t *testing.T) {
	t.Parallel()
	prev := os.Getenv(platformnats.EnvNATSURL)
	_ = os.Setenv(platformnats.EnvNATSURL, "nats://127.0.0.1:4222")
	t.Cleanup(func() {
		if prev == "" {
			_ = os.Unsetenv(platformnats.EnvNATSURL)
		} else {
			_ = os.Setenv(platformnats.EnvNATSURL, prev)
		}
	})

	c := &ReconcilerConfig{
		ActionsEnabled:          true,
		PaymentProbeURLTemplate: "http://localhost/probe/%s",
		RefundReviewSubject:     "   ",
		BatchLimit:              50,
	}
	if err := ValidateReconciler(c); err == nil {
		t.Fatal("expected error for empty refund subject")
	}
}

func TestValidateReconciler_actionsEnabled_nonPositiveBatch(t *testing.T) {
	t.Parallel()
	prev := os.Getenv(platformnats.EnvNATSURL)
	_ = os.Setenv(platformnats.EnvNATSURL, "nats://127.0.0.1:4222")
	t.Cleanup(func() {
		if prev == "" {
			_ = os.Unsetenv(platformnats.EnvNATSURL)
		} else {
			_ = os.Setenv(platformnats.EnvNATSURL, prev)
		}
	})

	c := &ReconcilerConfig{
		ActionsEnabled:          true,
		PaymentProbeURLTemplate: "http://localhost/probe/%s",
		RefundReviewSubject:     "reconciler.refund_review",
		BatchLimit:              0,
	}
	if err := ValidateReconciler(c); err == nil {
		t.Fatal("expected error for batch limit <= 0")
	}
}
