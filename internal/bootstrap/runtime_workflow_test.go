package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/config"
)

func TestBuildRuntime_WorkflowOrchestrationWhenTemporalDisabled(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("HTTP_ADDR", ":0")
	t.Setenv("OTEL_SERVICE_NAME", "test")
	t.Setenv("TEMPORAL_ENABLED", "false")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	rt, err := BuildRuntime(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if rt.Deps.WorkflowOrchestration == nil {
		t.Fatal("expected non-nil WorkflowOrchestration")
	}
	if rt.Deps.WorkflowOrchestration.Enabled() {
		t.Fatal("expected temporal orchestration disabled")
	}
	err = rt.Deps.WorkflowOrchestration.Start(ctx, workfloworch.StartInput{Kind: workfloworch.KindDelayedCompensation})
	if !errors.Is(err, workfloworch.ErrDisabled) {
		t.Fatalf("got %v want ErrDisabled", err)
	}
}
