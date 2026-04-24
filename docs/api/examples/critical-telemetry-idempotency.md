# Critical telemetry — idempotency / `dedupe_key` samples

Device implementations should assign a **stable logical `idempotency_key`** for each critical event and mirror it on the MQTT wire as **`dedupe_key`** (see [mqtt-contract.md](../mqtt-contract.md)).

**Application-level ACK:** MQTT QoS 1 `PUBACK` is **not** sufficient to drop a durable device outbox entry for business-critical data. See [mqtt-contract.md — Application-level ACK](../mqtt-contract.md#application-level-ack-durable-device-outbox-and-business-durability-p0-clarity).

## JSON files in this folder

| File | Typical topic tail | Notes |
|------|-------------------|--------|
| [vend-success-idempotency.json](./vend-success-idempotency.json) | `…/events/vend` | `dedupe_key` mirrors device idempotency |
| [payment-success-idempotency.json](./payment-success-idempotency.json) | `…/telemetry` | `event_type` `payment.captured` |
| [cash-inserted-idempotency.json](./cash-inserted-idempotency.json) | `…/events/cash` | |
| [inventory-delta-idempotency.json](./inventory-delta-idempotency.json) | `…/events/inventory` | |
| [command-ack-idempotency.json](./command-ack-idempotency.json) | `…/commands/ack` | Command receipt wire (**not** vend telemetry) |

Canonical copies also live under `testdata/telemetry/` for CI; these examples are aligned with those fixtures.
