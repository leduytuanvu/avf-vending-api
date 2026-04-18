package httpserver

import "context"

// ReadinessProbe is implemented by runtime wiring to verify dependencies.
type ReadinessProbe interface {
	Ready(ctx context.Context) error
}
