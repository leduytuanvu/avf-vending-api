package redis

import (
	"context"
	"errors"
	"time"
)

// RunExclusive runs fn while holding an advisory Redis lock. If locker is nil, fn runs without locking.
// When another holder owns the lock, Acquire returns ErrLockNotAcquired and RunExclusive skips the callback (nil error).
func RunExclusive(ctx context.Context, lock Locker, name string, ttl time.Duration, fn func(context.Context) error) error {
	if fn == nil {
		return nil
	}
	if lock == nil || ttl <= 0 {
		return fn(ctx)
	}
	lk, err := lock.Acquire(ctx, name, ttl)
	if errors.Is(err, ErrLockNotAcquired) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _, _ = lock.Release(ctx, lk) }()
	return fn(ctx)
}
