package workfloworch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

type temporalBoundary struct {
	c         client.Client
	taskQueue string
}

// NewTemporal returns a boundary backed by a real Temporal client. taskQueue must be non-empty.
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
	spec, err := workflowSpecForStart(in)
	if err != nil {
		return err
	}
	_, err = b.c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       strings.TrimSpace(in.WorkflowID),
		TaskQueue:                b.taskQueue,
		WorkflowExecutionTimeout: 24 * 7 * time.Hour,
		WorkflowRunTimeout:       24 * time.Hour,
		WorkflowTaskTimeout:      10 * time.Second,
	}, spec.name, spec.args...)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("workfloworch: execute workflow %s (%s): %w", spec.name, in.WorkflowID, err)
	}
	return nil
}

func (b *temporalBoundary) Close() error {
	if b.c == nil {
		return nil
	}
	b.c.Close()
	return nil
}

type workflowStartSpec struct {
	name string
	args []any
}

func workflowSpecForStart(in StartInput) (workflowStartSpec, error) {
	if strings.TrimSpace(in.WorkflowID) == "" {
		return workflowStartSpec{}, fmt.Errorf("workfloworch: workflow_id is required for kind=%s", in.Kind)
	}
	switch in.Kind {
	case KindPaymentPendingTimeoutFollowUp:
		return workflowStartSpec{name: WorkflowNamePaymentPendingTimeoutFollowUp, args: in.Args}, nil
	case KindVendFailureAfterPaymentSuccess:
		return workflowStartSpec{name: WorkflowNameVendFailureAfterPaymentSuccess, args: in.Args}, nil
	case KindRefundOrchestration:
		return workflowStartSpec{name: WorkflowNameRefundOrchestration, args: in.Args}, nil
	case KindManualReviewEscalation:
		return workflowStartSpec{name: WorkflowNameManualReviewEscalation, args: in.Args}, nil
	default:
		return workflowStartSpec{}, fmt.Errorf("%w: kind=%s task_queue=%q", ErrWorkflowNotImplemented, in.Kind, "")
	}
}
