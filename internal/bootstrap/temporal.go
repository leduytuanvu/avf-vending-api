package bootstrap

import (
	"context"
	"fmt"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/config"
	platformtemporal "github.com/avf/avf-vending-api/internal/platform/temporal"
)

// BuildWorkflowOrchestration returns a workflow boundary plus its cleanup hook.
// When Temporal is disabled, the returned boundary is a non-nil disabled implementation.
func BuildWorkflowOrchestration(ctx context.Context, cfg *config.Config) (workfloworch.Boundary, func(), error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("bootstrap: nil config")
	}
	if !cfg.Temporal.Enabled {
		return workfloworch.NewDisabled(), func() {}, nil
	}
	tc, err := platformtemporal.Dial(platformtemporal.DialOptions{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("bootstrap: temporal dial: %w", err)
	}
	tb, err := workfloworch.NewTemporal(tc, cfg.Temporal.TaskQueue)
	if err != nil {
		tc.Close()
		return nil, nil, fmt.Errorf("bootstrap: temporal workflow boundary: %w", err)
	}
	return tb, func() { _ = tb.Close() }, nil
}
