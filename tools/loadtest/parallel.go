package loadtest

import "context"

// ParallelFleet runs fn once per machine row with a concurrency limit.
func ParallelFleet(ctx context.Context, manifest []MachineRow, concurrency int, fn func(context.Context, MachineRow) error) error {
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	errCh := make(chan error, len(manifest))
	for _, m := range manifest {
		m := m
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sem <- struct{}{}:
		}
		go func() {
			defer func() { <-sem }()
			errCh <- fn(ctx, m)
		}()
	}
	var first error
	for range manifest {
		if err := <-errCh; err != nil && first == nil {
			first = err
		}
	}
	return first
}
