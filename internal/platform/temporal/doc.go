// Package temporal is a thin wrapper around the Temporal Go SDK client and worker constructors.
//
// When Temporal is enabled in application config, scheduler processes dial via Dial and wire
// internal/app/workfloworch for long-running orchestration (optional; default off). The repository
// now includes a dedicated cmd/temporal-worker process that registers and runs workfloworch
// workflows/activities on the configured task queue.
//
// Use [Dial] to obtain a [client.Client], then [NewWorker] to bind a task queue and register workflows
// and activities. [RunInteractive] blocks until SIGINT/SIGTERM (same semantics as worker.InterruptCh).
//
// For Temporal Cloud or mTLS, extend [DialOptions] in client.go; keep workflow execution out of the
// HTTP request path—schedule via [workfloworch.Boundary.Start] instead.
package temporal
