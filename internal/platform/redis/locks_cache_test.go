package redis

import (
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"testing"
	"time"
)

func TestMemoryLockerReleaseRequiresOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	locker := NewMemoryLocker("test")
	l, err := locker.Acquire(ctx, "payment:reconcile", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := locker.Release(ctx, Lock{Key: l.Key, Owner: "other"}); err != nil || ok {
		t.Fatalf("non-owner release ok=%v err=%v", ok, err)
	}
	if _, err := locker.Acquire(ctx, "payment:reconcile", time.Minute); err != ErrLockNotAcquired {
		t.Fatalf("expected still locked, got %v", err)
	}
	if ok, err := locker.Release(ctx, l); err != nil || !ok {
		t.Fatalf("owner release ok=%v err=%v", ok, err)
	}
}

func TestMemoryRefreshSessionCacheInvalidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewMemoryRefreshSessionCache()
	accountID := id.NewUUIDV7()
	if err := c.PutRefreshSession(ctx, []byte("hash-one"), accountID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !c.Has([]byte("hash-one")) {
		t.Fatal("expected cached refresh session")
	}
	if err := c.InvalidateRefreshSession(ctx, []byte("hash-one")); err != nil {
		t.Fatal(err)
	}
	if c.Has([]byte("hash-one")) {
		t.Fatal("expected single refresh session invalidated")
	}
	if err := c.PutRefreshSession(ctx, []byte("hash-two"), accountID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := c.InvalidateAccountSessions(ctx, accountID); err != nil {
		t.Fatal(err)
	}
	if c.Has([]byte("hash-two")) {
		t.Fatal("expected account sessions invalidated")
	}
}

func TestMemoryCatalogCacheHitMissInvalidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewMemoryCatalogCache()
	scopeID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	if _, ok, err := c.Get(ctx, scopeID, machineID, "v1"); err != nil || ok {
		t.Fatalf("expected miss ok=%v err=%v", ok, err)
	}
	if err := c.Set(ctx, scopeID, machineID, "v1", []byte(`{"ok":true}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	if b, ok, err := c.Get(ctx, scopeID, machineID, "v1"); err != nil || !ok || string(b) != `{"ok":true}` {
		t.Fatalf("expected hit ok=%v err=%v body=%s", ok, err, string(b))
	}
	if err := c.InvalidateMachine(ctx, scopeID, machineID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := c.Get(ctx, scopeID, machineID, "v1"); err != nil || ok {
		t.Fatalf("expected miss after invalidate ok=%v err=%v", ok, err)
	}
}

func TestMemoryLoginFailureCounterLocksAtThreshold(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewMemoryLoginFailureCounter()
	scopeID := id.NewUUIDV7()
	locked, n, err := c.IncrementFailure(ctx, scopeID, "USER@example.com", 2, time.Minute)
	if err != nil || locked || n != 1 {
		t.Fatalf("first failure locked=%v n=%d err=%v", locked, n, err)
	}
	locked, n, err = c.IncrementFailure(ctx, scopeID, "user@example.com", 2, time.Minute)
	if err != nil || !locked || n != 2 {
		t.Fatalf("second failure locked=%v n=%d err=%v", locked, n, err)
	}
	if npeek, err := c.PeekFailureCount(ctx, scopeID, "user@example.com"); err != nil || npeek != 2 {
		t.Fatalf("peek n=%d err=%v", npeek, err)
	}
	if err := c.ClearFailures(ctx, scopeID, "user@example.com"); err != nil {
		t.Fatal(err)
	}
	locked, n, err = c.IncrementFailure(ctx, scopeID, "user@example.com", 2, time.Minute)
	if err != nil || locked || n != 1 {
		t.Fatalf("after clear locked=%v n=%d err=%v", locked, n, err)
	}
}
