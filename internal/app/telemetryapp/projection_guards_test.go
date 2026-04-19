package telemetryapp

import "testing"

func TestBoundedSeenSet_eviction(t *testing.T) {
	t.Parallel()
	s := newBoundedSeenSet(3)
	s.remember("a")
	s.remember("b")
	s.remember("c")
	if !s.contains("c") {
		t.Fatal("expected c")
	}
	s.remember("d")
	if s.contains("a") {
		t.Fatal("expected a evicted")
	}
	if !s.contains("d") {
		t.Fatal("expected d")
	}
}

func TestIdempotencyPayloadGuard(t *testing.T) {
	t.Parallel()
	g := newIdempotencyPayloadGuard(100)
	dup, conflict := g.check("k", 42)
	if dup || conflict {
		t.Fatalf("first: dup=%v conflict=%v", dup, conflict)
	}
	g.remember("k", 42)
	dup, conflict = g.check("k", 42)
	if !dup || conflict {
		t.Fatalf("replay: dup=%v conflict=%v", dup, conflict)
	}
	dup, conflict = g.check("k", 99)
	if dup || !conflict {
		t.Fatalf("conflict: dup=%v conflict=%v", dup, conflict)
	}
}

func TestJetStreamWorkers_failStreak(t *testing.T) {
	t.Parallel()
	w := &JetStreamWorkers{}
	w.markFail("m")
	w.markFail("m")
	if w.maxFailStreak() != 2 {
		t.Fatalf("streak %d", w.maxFailStreak())
	}
	w.markOK("m")
	if w.maxFailStreak() != 0 {
		t.Fatalf("after ok %d", w.maxFailStreak())
	}
}
