# MQTT command dispatch debugging

**Commands to machines** are published over **MQTT** (QoS 1) from the API path that handles dispatch. gRPC is **not** a replacement for MQTT command delivery in this phase.

## Flow (high level)

1. API accepts an authenticated dispatch request (REST).
2. Backend records command state and publishes to the device command topic layout under `MQTT_TOPIC_PREFIX` (see `internal/platform/mqtt`).
3. Device acknowledges via command receipt/ack topics consumed by **mqtt-ingest** / telemetry pipeline.

## QoS, reconnect, retained traffic

- Commands use **QoS 1**; broker PUBACK does **not** mean the device executed the command.
- **mqtt-ingest** auto-reconnects and **re-subscribes at QoS 1** on each connect; duplicate PUBLISH delivery is possible — command and telemetry layers dedupe explicitly.
- Dispatch uses **retain false**; if you see retained command payloads, fix the publisher or broker policy.

See `docs/api/mqtt-contract.md` (sessions, retained messages, TLS rotation).

## Config

- `MQTT_BROKER_URL`, `MQTT_TOPIC_PREFIX`, client IDs (`MQTT_CLIENT_ID` or split API/ingest IDs).
- Staging/production: **TLS required** (broker URL scheme or `MQTT_TLS_ENABLED=true`).
- TLS envs: `MQTT_CA_FILE`, optional `MQTT_CERT_FILE` + `MQTT_KEY_FILE`, and `MQTT_INSECURE_SKIP_VERIFY` only for `APP_ENV=development` / `test`.
- Production topic policy: the default production prefix is `avf/devices`; for the strict enterprise tree use `MQTT_TOPIC_LAYOUT=enterprise` so topics become `avf/devices/machines/{machine_id}/...`. Nonstandard production prefixes such as `avf/production` require the documented `PRODUCTION_ALLOW_NONSTANDARD_MQTT_TOPIC_PREFIX=true` override.
- API wiring: when `API_REQUIRE_MQTT_PUBLISHER=true`, startup fails if the publisher is not configured.

## Enterprise topic layout

With `MQTT_TOPIC_LAYOUT=enterprise`, the backend publishes/subscribes under:

- API to machine commands: `{prefix}/machines/{machine_id}/commands`
- machine ACKs: `{prefix}/machines/{machine_id}/commands/ack`
- telemetry: `{prefix}/machines/{machine_id}/telemetry`
- generic events: `{prefix}/machines/{machine_id}/events`
- shadow updates: `{prefix}/machines/{machine_id}/shadow/reported` and `{prefix}/machines/{machine_id}/shadow/desired`

Set `MQTT_TOPIC_PREFIX=avf/<env>` if you want the literal `avf/{env}/machines/...` shape in a non-production or explicitly overridden production deployment.

## Command lifecycle labels

Operator-facing command state should use the enterprise lifecycle labels:

- `queued` — accepted into the command ledger but not yet sent to MQTT
- `published` — handed to the broker on the machine command topic
- `delivered` — reserved for a future broker/device delivery signal when supported
- `acked` — device ACK/receipt accepted idempotently
- `executed` — reserved for a future distinct device execution-complete signal
- `failed` — NACK, publish failure, ACK timeout, or terminal command failure
- `expired` — command was too late or stale and cannot be marked successful
- `canceled` — reserved for explicit cancellation/supersede flows

Current persistence states such as `pending`, `sent`, `completed`, `nack`, `ack_timeout`, `expired`, `duplicate`, and `late` map into these labels. Do not treat MQTT QoS broker acknowledgement as `acked`; `acked` means the device command receipt/ACK was validated and stored.

## ACK validation policy

ACK/receipt ingest validates:

- the machine id in the topic, and optional payload `machine_id`, must match
- the command sequence belongs to that machine in the ledger
- `dedupe_key` is required for idempotency
- duplicate same ACK is safe
- conflicting ACK on a terminal command is rejected and audited
- late successful ACKs after timeout/expiry are rejected by policy

## Metrics

- **`avf_mqtt_publish_duration_seconds`** — time from publish to broker ack for command dispatch (`result` = `ok` or `error`).
- **`avf_mqtt_invalid_topics_total{reason="machine_id_mismatch"}`** — topic vs envelope `machine_id` mismatch before persistence (telemetry/shadow envelopes); see also `avf_telemetry_ingest_rejected_total`.
- Ingest side: **`avf_mqtt_ingest_dispatch_total`**, **`avf_telemetry_ingest_rejected_total{reason=…}`**, and `avf_device_heartbeat_ingest_total` for heartbeat-shaped topics (not a substitute for end-to-end command ack).

## Readiness

With strict readiness, the API **ready** probe can fail if MQTT (or other required deps) is not connected when configured as required. Check logs for readiness warnings; do not expose broker URLs or credentials in responses.

## Debugging checklist

1. Confirm EMQX (or broker) shows the API client connected and ACL allows publish to command topics.
2. Confirm **machine id** in the topic matches the device’s subscription pattern.
3. If publish succeeds but device silent: verify firmware subscription, TLS, and ingest path for ack/receipt topics (`internal/platform/mqtt/router.go`).

## Stuck command playbook

1. Inspect **`command_ledger.route_key`** (JSON: MQTT topic + **`payload_sha256_hex`**) alongside **`machine_command_attempts`** (`status`, `ack_deadline_at`, `timeout_reason`) and receipts.
2. Classify state:
   - `queued` / `pending`: API accepted the command but publish has not completed. Check API MQTT publisher wiring and broker availability.
   - `published` / `sent`: broker accepted publish, but no validated device ACK arrived. Check device subscription, topic prefix, TLS, and ingest process.
   - `ack_timeout` / `expired`: the ACK window elapsed. Do not mark it successful manually; create an operator incident or retry with a new command/idempotency key if business policy allows.
   - `nack` / `failed`: inspect failure reason and device logs before retrying.
3. Check metrics for publish errors and ACK timeouts.
4. If local MQTT is available, reproduce only with a lab device/simulator.

Git Bash local broker probe, when EMQX management is exposed:

```bash
export SMOKE_CHECK_MQTT_URL="http://localhost:18083/api/v5/status"
export BASE_URL="http://localhost:8080"
export ENVIRONMENT_NAME="local"
python tools/smoke_test.py --report smoke-reports/mqtt-probe.json
```

PowerShell:

```powershell
$env:SMOKE_CHECK_MQTT_URL = "http://localhost:18083/api/v5/status"
$env:BASE_URL = "http://localhost:8080"
$env:ENVIRONMENT_NAME = "local"
python tools/smoke_test.py --report smoke-reports/mqtt-probe.json
```

Do not dispatch commands to production hardware from a generic smoke job. Command ACK smoke needs a known test machine or simulator.

## Related

- [mqtt-ingest-telemetry-limits.md](./mqtt-ingest-telemetry-limits.md)
- [production-readiness.md](./production-readiness.md)

## Prometheus signals (canonical)

- **`commands_dispatched_total`**, **`commands_acked_total`**, **`commands_failed_total`**, **`command_ack_latency_seconds`**, MQTT ingest mirrors under `productionmetrics` / `mqttprom`.

See [`docs/observability/production-metrics.md`](../observability/production-metrics.md).
