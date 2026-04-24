# Telemetry contract samples (device → mqtt-ingest)

JSON bodies for MQTT publish payloads (QoS **1**). Topic prefix and `{machine_id}` segment are omitted here; see [docs/api/mqtt-contract.md](../../docs/api/mqtt-contract.md). **Application-level ACK / durable outbox:** [mqtt-contract.md — Application-level ACK](../../docs/api/mqtt-contract.md#application-level-ack-durable-device-outbox-and-business-durability-p0-clarity).

Canonical machine UUID in samples: `55555555-5555-5555-5555-555555555555` (replace in topics).

| File | Topic tail (under prefix) | Purpose |
|------|---------------------------|---------|
| `valid_vend_success.json` | `.../events/vend` | Critical vend success |
| `valid_vend_failed.json` | `.../events/vend` | Critical vend failure |
| `valid_payment_success.json` | `.../telemetry` | Critical payment |
| `valid_cash_inserted.json` | `.../events/cash` | Critical cash |
| `valid_inventory_delta.json` | `.../events/inventory` | Critical inventory |
| `valid_command_ack.json` | `.../commands/ack` | Command ack wire |
| `valid_heartbeat_metrics.json` | `.../state/heartbeat` | Droppable heartbeat |
| `invalid_critical_missing_identity.json` | `.../events/vend` | Must be rejected (critical) |
| `invalid_critical_wrong_machine_id.json` | `.../events/vend` | Body `machine_id` ≠ topic |
| `invalid_occurred_at_malformed.json` | `.../events/vend` | Bad `occurred_at` (JSON time) |
| `duplicate_replay_vend.json` | `.../events/vend` | Same idempotency as `valid_vend_success` (replay drill) |

Validate JSON:

```bash
for f in testdata/telemetry/*.json; do python -m json.tool "$f" >/dev/null && echo OK "$f"; done
```
