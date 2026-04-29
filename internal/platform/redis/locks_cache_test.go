package redis

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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
	accountID := uuid.New()
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
	orgID := uuid.New()
	machineID := uuid.New()
	if _, ok, err := c.Get(ctx, orgID, machineID, "v1"); err != nil || ok {
		t.Fatalf("expected miss ok=%v err=%v", ok, err)
	}
	if err := c.Set(ctx, orgID, machineID, "v1", []byte(`{"ok":true}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	if b, ok, err := c.Get(ctx, orgID, machineID, "v1"); err != nil || !ok || string(b) != `{"ok":true}` {
		t.Fatalf("expected hit ok=%v err=%v body=%s", ok, err, string(b))
	}
	if err := c.InvalidateMachine(ctx, orgID, machineID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := c.Get(ctx, orgID, machineID, "v1"); err != nil || ok {
		t.Fatalf("expected miss after invalidate ok=%v err=%v", ok, err)
	}
}

func TestMemoryLoginFailureCounterLocksAtThreshold(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := NewMemoryLoginFailureCounter()
	orgID := uuid.New()
	locked, n, err := c.IncrementFailure(ctx, orgID, "USER@example.com", 2, time.Minute)
	if err != nil || locked || n != 1 {
		t.Fatalf("first failure locked=%v n=%d err=%v", locked, n, err)
	}
	locked, n, err = c.IncrementFailure(ctx, orgID, "user@example.com", 2, time.Minute)
	if err != nil || !locked || n != 2 {
		t.Fatalf("second failure locked=%v n=%d err=%v", locked, n, err)
	}
	if npeek, err := c.PeekFailureCount(ctx, orgID, "user@example.com"); err != nil || npeek != 2 {
		t.Fatalf("peek n=%d err=%v", npeek, err)
	}
	if err := c.ClearFailures(ctx, orgID, "user@example.com"); err != nil {
		t.Fatal(err)
	}
	locked, n, err = c.IncrementFailure(ctx, orgID, "user@example.com", 2, time.Minute)
	if err != nil || locked || n != 1 {
		t.Fatalf("after clear locked=%v n=%d err=%v", locked, n, err)
	}
}
