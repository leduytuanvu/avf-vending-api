package temporal

import (
	"fmt"
	"strings"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// RegisterFunc registers workflows and activities on a worker before Run.
type RegisterFunc func(w worker.Worker)

// NewWorker constructs a worker for the given task queue. Call RegisterFunc to attach workflows/activities.
// No cmd/* process invokes NewWorker by default; add a dedicated worker binary when workflows are implemented.
func NewWorker(c client.Client, taskQueue string, wo worker.Options, reg RegisterFunc) (worker.Worker, error) {
	if c == nil {
		return nil, fmt.Errorf("temporal: nil client")
	}
	tq := strings.TrimSpace(taskQueue)
	if tq == "" {
		return nil, fmt.Errorf("temporal: task queue is required")
	}
	w := worker.New(c, tq, wo)
	if reg != nil {
		reg(w)
	}
	return w, nil
}

// RunInteractive runs the worker until SIGINT or SIGTERM.
func RunInteractive(w worker.Worker) error {
	if w == nil {
		return fmt.Errorf("temporal: nil worker")
	}
	return w.Run(worker.InterruptCh())
}
