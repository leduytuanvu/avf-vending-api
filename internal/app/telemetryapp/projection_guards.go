package telemetryapp

import (
	"hash/fnv"
	"sync"
)

func hashPayload(b []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(b)
	return h.Sum64()
}

// boundedSeenSet tracks string keys with FIFO eviction (projection stream sequence dedupe).
type boundedSeenSet struct {
	mu   sync.Mutex
	maxN int
	seen map[string]struct{}
	fifo []string
}

func newBoundedSeenSet(maxN int) *boundedSeenSet {
	if maxN < 1 {
		maxN = 1
	}
	return &boundedSeenSet{
		maxN: maxN,
		seen: make(map[string]struct{}),
		fifo: make([]string, 0, minInt(1024, maxN)),
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *boundedSeenSet) contains(k string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.seen[k]
	return ok
}

func (s *boundedSeenSet) remember(k string) {
	if s == nil || k == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seen[k]; ok {
		return
	}
	s.seen[k] = struct{}{}
	s.fifo = append(s.fifo, k)
	for len(s.seen) > s.maxN && len(s.fifo) > 0 {
		old := s.fifo[0]
		s.fifo = s.fifo[1:]
		delete(s.seen, old)
	}
}

// idempotencyPayloadGuard remembers last payload hash per idempotency key (bounded FIFO eviction).
type idempotencyPayloadGuard struct {
	mu   sync.Mutex
	maxN int
	m    map[string]uint64
	fifo []string
}

func newIdempotencyPayloadGuard(maxN int) *idempotencyPayloadGuard {
	if maxN < 1 {
		maxN = 1
	}
	return &idempotencyPayloadGuard{
		maxN: maxN,
		m:    make(map[string]uint64),
		fifo: make([]string, 0, minInt(1024, maxN)),
	}
}

// check returns: skipDuplicate=true when key seen with same hash; conflict=true when same key, different hash.
func (g *idempotencyPayloadGuard) check(key string, payloadHash uint64) (skipDuplicate, conflict bool) {
	if g == nil || key == "" {
		return false, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	prev, ok := g.m[key]
	if !ok {
		return false, false
	}
	if prev == payloadHash {
		return true, false
	}
	return false, true
}

func (g *idempotencyPayloadGuard) remember(key string, payloadHash uint64) {
	if g == nil || key == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.m[key]; !exists {
		g.m[key] = payloadHash
		g.fifo = append(g.fifo, key)
	} else {
		g.m[key] = payloadHash
	}
	for len(g.m) > g.maxN && len(g.fifo) > 0 {
		old := g.fifo[0]
		g.fifo = g.fifo[1:]
		delete(g.m, old)
	}
}
