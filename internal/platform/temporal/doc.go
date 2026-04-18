// Package temporal is a thin wrapper around the Temporal Go SDK client and worker constructors.
//
// When Temporal is enabled in application config, bootstrap.BuildRuntime dials via Dial and wires
// internal/app/workfloworch for long-running orchestration (optional; default off). No workflow
// implementations are registered in this repository yet—workfloworch.Boundary.Start returns
// ErrWorkflowNotImplemented until kinds are mapped. A dedicated cmd/* Temporal worker is optional.
//
// Use [Dial] to obtain a [client.Client], then [NewWorker] to bind a task queue and register workflows
// and activities. [RunInteractive] blocks until SIGINT/SIGTERM (same semantics as worker.InterruptCh).
//
// For Temporal Cloud or mTLS, extend [DialOptions] in client.go; keep workflow execution out of the
// HTTP request path—schedule via [workfloworch.Boundary.Start] instead.
package temporal
