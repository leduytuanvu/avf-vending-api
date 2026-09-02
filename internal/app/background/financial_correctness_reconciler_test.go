package background_test

import (
	"context"
	"testing"

	appbackground "github.com/avf/avf-vending-api/internal/app/background"
)

func TestFinancialCorrectnessReconcileTick_noPool(t *testing.T) {
	err := appbackground.FinancialCorrectnessReconcileTick(context.Background(), appbackground.ReconcilerDeps{})
	if err != nil {
		t.Fatalf("expected nil when observability pool unset: %v", err)
	}
}
