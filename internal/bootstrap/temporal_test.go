package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/config"
)

func TestBuildWorkflowOrchestration_DisabledDoesNotDialTemporal(t *testing.T) {
	t.Parallel()
	boundary, cleanup, err := BuildWorkflowOrchestration(context.Background(), &config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if cleanup == nil {
		t.Fatal("cleanup is nil")
	}
	defer cleanup()
	if boundary == nil || boundary.Enabled() {
		t.Fatalf("expected disabled boundary, got %#v", boundary)
	}
	if err := boundary.Start(context.Background(), workfloworch.StartInput{Kind: workfloworch.KindPaymentToVend}); !errors.Is(err, workfloworch.ErrDisabled) {
		t.Fatalf("got %v want ErrDisabled", err)
	}
}
