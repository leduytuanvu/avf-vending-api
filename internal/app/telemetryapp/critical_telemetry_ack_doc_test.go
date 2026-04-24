package telemetryapp

import "testing"

// Critical telemetry application-level ACK semantics are documented in docs/api/mqtt-contract.md
// ("Application-level ACK, durable device outbox, and business durability").
//
// Enforced by tests:
//   - internal/platform/mqtt/offline_replay_contract_test.go — stable identity, duplicate replay, command receipt dedupe_key required
//   - internal/platform/telemetry/critical_idempotency_test.go — StableCriticalIdempotencyKey derivation / missing identity
//   - internal/modules/postgres/telemetry_idempotency_integration_test.go — OLTP duplicate suppression for edge events
//   - internal/app/telemetryapp/offline_replay_contract_test.go — projection-layer duplicate payload guard
//
// P0 product gap (called out in the MQTT contract): no device-scoped HTTP/MQTT reconcile that proves an arbitrary
// critical telemetry idempotency_key is already persisted in OLTP; MQTT QoS 1 PUBACK alone is insufficient.
func TestCriticalTelemetry_ackSemantics_documentationAnchors(t *testing.T) {
	t.Parallel()
	t.Log("see docs/api/mqtt-contract.md § Application-level ACK")
}
