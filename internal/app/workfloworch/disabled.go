package workfloworch

import "context"

type disabledBoundary struct{}

// NewDisabled returns a boundary that reports Enabled() == false and rejects Start with [ErrDisabled].
func NewDisabled() Boundary {
	return disabledBoundary{}
}

func (disabledBoundary) Enabled() bool { return false }

func (disabledBoundary) Start(_ context.Context, _ StartInput) error {
	return ErrDisabled
}

func (disabledBoundary) Close() error { return nil }
