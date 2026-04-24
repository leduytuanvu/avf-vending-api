package telemetryapp

import (
	"testing"
)

// Offline replay delivers the same idempotency key and payload bytes more than once.
// The projection path uses idempotencyPayloadGuard to skip duplicate applies (same key + same payload hash).
func TestOfflineReplay_duplicatePayload_skippedByProjectionGuard(t *testing.T) {
	t.Parallel()
	g := newIdempotencyPayloadGuard(128)
	// Same dedupe_key as testdata/telemetry/valid_vend_success.json
	key := "vend:55555555-5555-5555-5555-555555555555:slot-3:2026-04-24T12:00:05Z:01JR8VEND"
	raw := []byte(`{"slot_index":3,"order_id":"11111111-1111-1111-1111-111111111111","outcome":"success","correlation_id":"22222222-2222-2222-2222-222222222222"}`)
	h := hashPayload(raw)

	skip, conflict := g.check(key, h)
	if skip || conflict {
		t.Fatalf("first: skip=%v conflict=%v", skip, conflict)
	}
	g.remember(key, h)

	skip, conflict = g.check(key, h)
	if !skip || conflict {
		t.Fatalf("replay: skip=%v conflict=%v", skip, conflict)
	}
}
