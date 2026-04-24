# Device offline replay — sample payloads

Canonical copies live under **`testdata/telemetry/`** in the repository root. Use them in integration tests and as fixtures for mobile/device teams.

## Topic pattern

```
{MQTT_TOPIC_PREFIX}/{machine_id_lowercase_uuid}/{channel...}
```

Example machine id (used in samples): `55555555-5555-5555-5555-555555555555`.

## Valid examples (copy from `testdata/telemetry/`)

| Scenario | File | Typical topic tail |
|----------|------|-------------------|
| Vend success | `valid_vend_success.json` | `.../events/vend` |
| Vend failed | `valid_vend_failed.json` | `.../events/vend` (uses `boot_id` + `seq_no`) |
| Payment | `valid_payment_success.json` | `.../telemetry` |
| Cash inserted | `valid_cash_inserted.json` | `.../events/cash` |
| Inventory delta | `valid_inventory_delta.json` | `.../events/inventory` |
| Command ack | `valid_command_ack.json` | `.../commands/ack` |
| Heartbeat / metrics | `valid_heartbeat_metrics.json` | `.../state/heartbeat` |

## Invalid examples (expect broker/app rejection)

| Scenario | File | Expected behavior |
|----------|------|-------------------|
| Critical without identity | `invalid_critical_missing_identity.json` | No `dedupe_key`, `event_id`, or `boot_id`+`seq_no` → ingest error after parse (mqtt-ingest records `critical_missing_idempotency_identity`). |
| Wrong body `machine_id` | `invalid_critical_wrong_machine_id.json` | `machine_id` in JSON ≠ topic machine id → `machine_id_mismatch`. |
| Bad `occurred_at` | `invalid_occurred_at_malformed.json` | Not RFC3339 → JSON decode failure on wire. |

## Duplicate replay drill

`duplicate_replay_vend.json` matches `valid_vend_success.json` idempotency. Publishing it twice is **valid JSON**; the backend uses the same stable idempotency key and JetStream `Msg-Id`, and the worker projection suppresses a second **apply** when the payload hash matches (see contract tests).

## Contract reference

Full rules: [mqtt-contract.md](../mqtt-contract.md).

**Durable outbox / ACK:** [mqtt-contract.md — Application-level ACK](../mqtt-contract.md#application-level-ack-durable-device-outbox-and-business-durability-p0-clarity). Copy-paste samples with explicit `dedupe_key` (device idempotency): [critical-telemetry-idempotency.md](./critical-telemetry-idempotency.md).
