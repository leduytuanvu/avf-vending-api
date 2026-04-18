// Package workfloworch is the application boundary for durable, long-running work on Temporal.
//
// When config.Temporal.Enabled is false, [NewDisabled] is used: [Boundary.Start] returns [ErrDisabled].
// When enabled, the API process dials Temporal at startup and uses [NewTemporal]; [Boundary.Start]
// returns [ErrWorkflowNotImplemented] until concrete workflows are registered and mapped here.
//
// Synchronous commerce and reliability paths remain unchanged; callers may opt in later to schedule
// follow-up work through this boundary without blocking HTTP or tick handlers.
package workfloworch
