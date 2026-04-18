package workfloworch

import "errors"

var (
	// ErrDisabled is returned from Start when Temporal orchestration is not enabled.
	ErrDisabled = errors.New("workfloworch: temporal orchestration disabled")

	// ErrWorkflowNotImplemented is returned when Temporal is enabled but no workflow is registered
	// for the requested kind (explicit extension point until implementations land).
	ErrWorkflowNotImplemented = errors.New("workfloworch: workflow not implemented for kind")
)
