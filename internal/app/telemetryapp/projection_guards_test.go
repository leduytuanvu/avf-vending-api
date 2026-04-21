//go:build !windows

package telemetryapp

import (
	"testing"
)

func TestIdempotencyPayloadGuard_replayAndConflict(t *testing.T) {
	t.Parallel()
	g := newIdempotencyPayloadGuard(128)
	key := "11111111-1111-1111-1111-111111111111:22222222-2222-2222-2222-222222222222:42"
	h1 := hashPayload([]byte(`{"a":1}`))
	h2 := hashPayload([]byte(`{"a":2}`))

	skip, conflict := g.check(key, h1)
	if skip || conflict {
		t.Fatalf("first check: skip=%v conflict=%v", skip, conflict)
	}
	g.remember(key, h1)

	skip, conflict = g.check(key, h1)
	if !skip || conflict {
		t.Fatalf("replay: skip=%v conflict=%v", skip, conflict)
	}

	skip, conflict = g.check(key, h2)
	if skip || !conflict {
		t.Fatalf("conflict: skip=%v conflict=%v", skip, conflict)
	}
}

func TestBoundedSeenSet_remembers(t *testing.T) {
	t.Parallel()
	s := newBoundedSeenSet(3)
	if s.contains("a") {
		t.Fatal("unexpected contains")
	}
	s.remember("a")
	s.remember("b")
	s.remember("c")
	if !s.contains("a") {
		t.Fatal("expected a")
	}
	s.remember("d")
	if s.contains("a") {
		t.Fatal("expected eviction of oldest")
	}
}
