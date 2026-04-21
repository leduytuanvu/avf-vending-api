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
| `occurred_at` | RFC3339 (optional) | |
| `correlation_id` | UUID (optional) | |
| `operator_session_id` | UUID (optional) | |
| `event_type` | string | **Required** on `…/telemetry` when not inferable from topic; topic-derived defaults exist for e.g. `presence`, `state/heartbeat`, `events/vend` |
| `dedupe_key` | string (optional) | Preferred telemetry idempotency key when set |
| `payload` | JSON | Inner telemetry/event payload (object); used as inner body for telemetry |
| `reported` | JSON object | Shadow **reported** (or nested under `payload`) |
| `desired` | JSON object | Shadow **desired** (or nested under `payload`) |

Telemetry inner `payload` must be valid JSON; optional size/complexity limits apply (`MQTTDeviceTelemetry` config, enforced in `Dispatch`).

## Command receipt / ack (device → cloud)

On `{prefix}/{machineId}/commands/receipt` or `…/commands/ack`, `Dispatch` unmarshals the **top-level** JSON into:

| Field | Required | Notes |
|-------|----------|--------|
| `sequence` | yes | int ≥ 0 |
| `status` | yes | `acked`, `nacked`, `failed`, `timeout` (aliases: `ack` → `acked`, etc.) |
| `dedupe_key` | yes | non-empty string |
| `payload` | yes | JSON object (may be `{}`) |
| `correlation_id` | no | UUID |

## HTTP fallbacks (same auth as machine-scoped admin APIs)

When MQTT dispatch is degraded, the mobile app or edge bridge can use Bearer JWT + machine URL access:

- **`POST /v1/device/machines/{machineId}/vend-results`** — requires `Idempotency-Key` (or `X-Idempotency-Key`). Body: `order_id`, `slot_index`, `outcome` (`success` \| `failed`), optional `failure_reason`, optional `correlation_id`. See OpenAPI example on that path.
- **`POST /v1/device/machines/{machineId}/commands/poll`** — optional body `{"limit":20}` (max 100). Returns pending/sent commands.

OpenAPI: `docs/swagger/swagger.json` (regenerate with `python tools/build_openapi.py` from repo root).
