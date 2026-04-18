package workfloworch

import (
	"context"
	"fmt"
	"strings"

	"go.temporal.io/sdk/client"
)

type temporalBoundary struct {
	c         client.Client
	taskQueue string
}

// NewTemporal returns a boundary backed by a real Temporal client. taskQueue must be non-empty
// (validated via config when Temporal is enabled). Start returns [ErrWorkflowNotImplemented] until
// workflows are registered and invoked here.
func NewTemporal(c client.Client, taskQueue string) (Boundary, error) {
	if c == nil {
		return nil, fmt.Errorf("workfloworch: nil temporal client")
	}
	tq := strings.TrimSpace(taskQueue)
	if tq == "" {
		return nil, fmt.Errorf("workfloworch: empty task queue")
	}
	return &temporalBoundary{c: c, taskQueue: tq}, nil
}

func (b *temporalBoundary) Enabled() bool { return true }

func (b *temporalBoundary) Start(ctx context.Context, in StartInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = in.WorkflowID
	_ = in.Args
	return fmt.Errorf("%w: kind=%s task_queue=%q", ErrWorkflowNotImplemented, in.Kind, b.taskQueue)
}

func (b *temporalBoundary) Close() error {
	if b.c == nil {
		return nil
	}
	b.c.Close()
	return nil
}
