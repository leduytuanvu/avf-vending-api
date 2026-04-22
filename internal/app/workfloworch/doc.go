// Package workfloworch is the application boundary for durable, long-running work on Temporal.
//
// When config.Temporal.Enabled is false, [NewDisabled] is used: [Boundary.Start] returns [ErrDisabled].
// When enabled, scheduler processes dial Temporal at startup and use [NewTemporal] to enqueue the
// registered workflows executed by cmd/temporal-worker.
//
// Synchronous commerce and reliability paths remain unchanged; callers may opt in later to schedule
// follow-up work through this boundary without blocking HTTP or tick handlers.
package workfloworch
