# MQTT device contract (implemented)

This describes what **this repository** actually publishes and what **cmd/mqtt-ingest** subscribes to. Source: `internal/platform/mqtt/` (topics, router, subscriber, publisher).

## Topic layout

- **Prefix**: `MQTT_TOPIC_PREFIX` (trimmed; no trailing slash). Canonical full topic: `{prefix}/{machineId}/{channel…}`.
- **Machine segment**: lowercase UUID string.

## Subscriptions (mqtt-ingest)

`internal/platform/mqtt/subscriber.go` connects with **QoS 1** and subscribes to **`mqtt.InboundDeviceTopicPatterns(prefix)`** (retain flag **false** on subscribe; broker retained messages may still be delivered):

| Pattern | Purpose |
|--------|---------|
| `{prefix}/+/telemetry` | Legacy telemetry + envelope parsing |
| `{prefix}/+/presence` | Presence events |
| `{prefix}/+/state/heartbeat` | Heartbeat |
| `{prefix}/+/telemetry/snapshot` | Telemetry snapshot |
| `{prefix}/+/telemetry/incident` | Telemetry incident |
| `{prefix}/+/events/vend` | Vend events |
| `{prefix}/+/events/cash` | Cash events |
| `{prefix}/+/events/inventory` | Inventory events |
| `{prefix}/+/shadow/reported` | Shadow **reported** |
| `{prefix}/+/shadow/desired` | Shadow **desired** (device-originated updates) |
| `{prefix}/+/commands/receipt` | Command receipt |
| `{prefix}/+/commands/ack` | Command ack |

## Outbound (API → broker)

`Publisher.PublishDeviceDispatch` (`internal/platform/mqtt/publisher.go`) publishes to **`{prefix}/{machineId}/commands/dispatch`** with **QoS 1**, retain **false**. The codebase also defines **`commands/down`** as an alternate outbound tail (`OutboundCommandDownTopic`); the default publisher path uses **`dispatch`**. Devices do **not** publish to `commands/down` for ingest.

| Topic | QoS | Retain |
|-------|-----|--------|
| `{prefix}/{machineId}/commands/dispatch` | **1** | **false** |

Payload is the JSON command wire produced by the command dispatcher (not the generic device envelope below).

## JSON envelope (device → cloud, telemetry / shadow paths)

For channels handled via `decodeDeviceWire` + `Dispatch` switch (`telemetry`, `presence`, `state/heartbeat`, `telemetry/snapshot`, `telemetry/incident`, `events/vend`, `events/cash`, `events/inventory`, `shadow/reported`, `shadow/desired`), the parser expects a JSON object with these fields (see `deviceWire` in `internal/platform/mqtt/router.go`):

| Field | Type | Notes |
|-------|------|--------|
| `schema_version` | int | Wire version hint |
| `event_id` | string | Stable id; used for idempotency when dedupe/boot+seq absent |
| `machine_id` | UUID (optional) | If set, must match topic machine id |
| `boot_id` | UUID (optional) | With `seq_no`, forms dedupe key |
| `seq_no` | int64 (optional) | With `boot_id`, forms dedupe key |
| `occurred_at` | RFC3339 (optional) | Current in-repo event timestamp field |
| `correlation_id` | UUID (optional) | |
| `operator_session_id` | UUID (optional) | |
| `event_type` | string | **Required** on `…/telemetry` when not inferable from topic; topic-derived defaults exist for e.g. `presence`, `state/heartbeat`, `events/vend` |
| `dedupe_key` | string (optional) | Preferred telemetry idempotency key when set |
| `payload` | JSON | Inner telemetry/event payload (object); used as inner body for telemetry |
| `reported` | JSON object | Shadow **reported** (or nested under `payload`) |
| `desired` | JSON object | Shadow **desired** (or nested under `payload`) |

Telemetry inner `payload` must be valid JSON; optional size/complexity limits apply (`MQTTDeviceTelemetry` config, enforced in `Dispatch`).

## Offline replay contract

When a device stores runtime events offline and replays them later, every replayed event must carry enough metadata for dedupe, ordering, and pacing:

- `machine_id`
- `event_id` **or** the pair `boot_id` + `seq_no`
- `emitted_at`
- `event_type`
- `idempotency_key`

For compatibility with the current ingest implementation in this repo:

- map `emitted_at` to the existing top-level wire field `occurred_at`
- mirror `idempotency_key` into `dedupe_key` on MQTT envelopes handled by this repository

Devices should treat the following as the stable replay identity inputs:

- primary: `idempotency_key` (mirror as `dedupe_key` on the wire)
- fallback: `event_id`
- secondary ordering/dedupe tuple: `boot_id` + `seq_no` (together with `event_type`; see below)

### Backend idempotency key (critical telemetry)

For **`critical_no_drop`** event types (vend, payment, cash, inventory, critical incidents, etc.), mqtt-ingest **rejects** the message before JetStream publish if **none** of the above identities are present. The canonical key written to the telemetry envelope and used as the JetStream `MsgId` is:

1. `dedupe_key` when non-empty after trim (device-provided; used as-is for backward compatibility).
2. Else `{machine_id}:{normalized_event_type}:{event_id}` where `normalized_event_type` is lowercased / trimmed `event_type`.
3. Else `{machine_id}:{boot_id}:{seq_no}:{normalized_event_type}`.

Non-critical metrics may still use time-based JetStream dedupe when no identity is provided (best-effort; not replay-safe).

## Sample fixtures and contract tests

- **JSON samples**: `testdata/telemetry/*.json` (vend, payment, cash, inventory, command ack, heartbeat, invalid cases, duplicate replay).
- **Index for device teams**: [examples/device-offline-replay-samples.md](./examples/device-offline-replay-samples.md).
- **Go tests**: `internal/platform/mqtt/offline_replay_contract_test.go` (Dispatch + critical identity rules), `internal/app/telemetryapp/offline_replay_contract_test.go` (projection duplicate suppression), `internal/platform/telemetry/offline_replay_classify_test.go` (criticality map).
- **JSON-only validator**: `go run ./tools/telemetry-contract` from repo root.

## MQTT QoS 1, broker ACK, and critical retries

- Device publishes should use **QoS 1** (matches mqtt-ingest subscribe). The broker **PUBACK** means the broker accepted the packet, not that downstream JetStream + projection finished.
- For **`critical_no_drop`**, mqtt-ingest runs the JetStream publish on the synchronous path before `Dispatch` returns success to the subscriber stack. Treat **handler errors** (e.g. missing identity, publish failure, oversize) as **not stored**; retry with backoff using the **same** logical idempotency (`dedupe_key` / `event_id` / `boot_id`+`seq_no`) so the cloud can dedupe.
- **Duplicate offline replay** of the same bytes and idempotency is expected; JetStream `Msg-Id` matches the stable key, and the worker projection skips a second **apply** when the payload hash matches (see `idempotencyPayloadGuard` tests).

## Application-level ACK, durable device outbox, and business durability (P0 clarity)

**MQTT QoS 1 alone is not a business-level ACK.** The broker `PUBACK` normally returns to the publisher **before** mqtt-ingest has finished `Dispatch` (subscriber runs asynchronously from the broker’s point of view). Even when ingest later completes, that completion is **not** surfaced to the device as a separate MQTT application ACK in this repository.

### When the device may remove a critical event from its durable outbox

Use the **most conservative** rule that matches how you actually ship:

1. **HTTP paths with synchronous success responses (supported today for a narrow subset)**  
   - `POST /v1/device/machines/{machineId}/vend-results` with `Idempotency-Key` / `X-Idempotency-Key`: treat **HTTP 2xx** after a successful response body as **“backend processed this idempotent write”** for that vend result path (commerce finalization + inventory projection in the handler).  
   - This is **not** the same contract as MQTT `events/vend`.

2. **Command transport ACKs (different problem than commerce telemetry)**  
   - For **remote command** lifecycle, the device publishes **`commands/receipt`** or **`commands/ack`** with a required **`dedupe_key`** (see below). That flow updates `device_command_receipts` and related attempt state. Operators can inspect recent receipts via **`GET /v1/machines/{machineId}/commands/receipts`** (Bearer JWT, machine URL access) — this is an **admin/operator** read model, not a dedicated “device reconcile by idempotency_key” API for all telemetry.

3. **MQTT critical telemetry (`events/vend`, `events/cash`, `events.inventory`, `payment.*` on `…/telemetry`, etc.)**  
   - **Ingest acceptance (JetStream durable write):** mqtt-ingest completes a **`critical_no_drop`** publish to JetStream before reporting handler success internally; the device **does not** receive an application-level ACK when that happens.  
   - **Processed (OLTP) acceptance:** the worker commits domain rows (e.g. `device_telemetry_events`, `inventory_events`) idempotently using the envelope **`Idempotency`** (from wire `dedupe_key` / stable key).  
   - **Gap — P0 for “device-driven reconcile”:** this API **does not** expose a first-class, machine-scoped **`GET`** (or MQTT topic) that returns “`idempotency_key` / `dedupe_key` X is persisted” for arbitrary critical telemetry. Until that exists, devices **must not** treat MQTT `PUBACK` alone, or “no client-side error”, as proof of OLTP durability.  
   - **Operational posture:** keep the event in the durable outbox and **retry with the same** `dedupe_key` / `event_id` / `boot_id`+`seq_no` until HTTP success (where applicable), operator tooling confirms persistence, or a future first-party reconcile/ACK contract ships.

**Enterprise-ready wording:** do **not** claim end-to-end “ACK to device” for all critical MQTT telemetry based only on QoS or broker behavior.

### Sample payloads with idempotency (`dedupe_key`)

On the wire, the canonical field is **`dedupe_key`** (mirror of the device’s logical **`idempotency_key`**). Copy-paste examples: [examples/critical-telemetry-idempotency.md](./examples/critical-telemetry-idempotency.md) and `testdata/telemetry/*.json`.

## Example: vend success (MQTT `…/events/vend`)

Full file: `testdata/telemetry/valid_vend_success.json`.

```json
{
  "schema_version": 1,
  "machine_id": "55555555-5555-5555-5555-555555555555",
  "event_id": "01JR8VEND-SUCCESS-EXAMPLE-0001",
  "occurred_at": "2026-04-24T12:00:05Z",
  "dedupe_key": "vend:55555555-5555-5555-5555-555555555555:slot-3:2026-04-24T12:00:05Z:01JR8VEND",
  "event_type": "vend.success",
  "payload": {
    "slot_index": 3,
    "order_id": "11111111-1111-1111-1111-111111111111",
    "outcome": "success",
    "correlation_id": "22222222-2222-2222-2222-222222222222"
  }
}
```

Topic: `{prefix}/55555555-5555-5555-5555-555555555555/events/vend`.

## Replay pacing requirements

Devices must not replay offline queues at line rate after a fleet-wide reconnect.

Required pacing policy:

- initial replay jitter: deterministic `0-300` seconds derived from a stable `machine_id` hash
- steady replay rate: `1-5` events/sec per machine
- batch size: `20-50` events per replay batch
- retries: exponential backoff with jitter between failed replay attempts

## Criticality classes

The backend applies three ingress criticality classes:

- `critical_no_drop`: processed **synchronously** in mqtt-ingest (not placed on the bounded async queue): `Dispatch` returns success only after the JetStream publish path completes (or returns the handler error). Per-machine rate limits still return a retryable error. Droppable traffic continues to use the bounded queue for memory protection.
- `compactable_latest`: latest-state style events may be compacted by machine + event key while waiting in the queue
- `droppable_metrics`: heartbeat, high-frequency metrics, and debug/noise events may be dropped under backpressure

`critical_no_drop` includes:

- vend success / failure / result
- payment / cashless / refund events
- cash inserted / payout / collection
- inventory delta / refill / adjustment
- command ack / config ack / command receipt
- critical incidents such as jam, door open, motor fault, and temperature critical

`compactable_latest` includes:

- telemetry snapshots
- shadow reported / desired updates
- other latest-state style updates where only the newest value matters

`droppable_metrics` includes:

- heartbeat / presence
- high-frequency telemetry metrics
- debug / noise events

Compaction must preserve the newest useful state while avoiding replay storms for stale metrics.

## Durability requirement

The device/app must persist the offline queue to durable local storage before attempting network send. Network handoff alone is not sufficient for critical events.

## Command receipt / ack (device → cloud)

On `{prefix}/{machineId}/commands/receipt` or `…/commands/ack`, `Dispatch` unmarshals the **top-level** JSON into:

| Field | Required | Notes |
|-------|----------|--------|
| `sequence` | yes | int ≥ 0 |
| `status` | yes | `acked`, `nacked`, `failed`, `timeout` (aliases: `ack` → `acked`, etc.) |
| `dedupe_key` | yes | non-empty string (**application idempotency key** for this receipt; ties to `device_command_receipts`) |
| `payload` | yes | JSON object (may be `{}`) |
| `correlation_id` | no | UUID |

Missing **`dedupe_key`** is rejected at the MQTT router (`commands.receipt.dedupe_key is required`). This is the **supported** application-level identity for command receipts; it is **not** a substitute for vend/cash/inventory MQTT event ACKs.

## HTTP fallbacks (same auth as machine-scoped admin APIs)

When MQTT dispatch is degraded, the mobile app or edge bridge can use Bearer JWT + machine URL access:

- **`POST /v1/device/machines/{machineId}/vend-results`** — requires `Idempotency-Key` (or `X-Idempotency-Key`). Body: `order_id`, `slot_index`, `outcome` (`success` \| `failed`), optional `failure_reason`, optional `correlation_id`. See OpenAPI example on that path.
- **`POST /v1/device/machines/{machineId}/commands/poll`** — optional body `{"limit":20}` (max 100). Returns pending/sent commands.

OpenAPI: `docs/swagger/swagger.json` (regenerate with `python tools/build_openapi.py` from repo root).
