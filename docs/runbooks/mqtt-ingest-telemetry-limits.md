# mqtt-ingest: telemetry ingress limits

This runbook describes environment variables that cap device MQTT telemetry before JetStream publish. They apply to `cmd/mqtt-ingest` and are validated in `internal/config`.

When `avf_telemetry_ingest_*` counters move in production, use [production-observability-alerts.md](./production-observability-alerts.md) for alert thresholds and first-response commands.

## Production guardrails

- `APP_ENV=production` requires non-empty `NATS_URL` at process startup (`config.Validate`).
- `TELEMETRY_LEGACY_POSTGRES_INGEST=true` is rejected in production (high-frequency telemetry must use the NATS bridge).
- The HTTP API bootstrap (`internal/bootstrap/api.go`) also fails fast if `APP_ENV=production` and `NATS_URL` is unset.

## Ingress and backpressure

| Variable | Role |
| --- | --- |
| `TELEMETRY_MAX_PAYLOAD_BYTES` | Max outer MQTT JSON body per message (Dispatch rejects early). |
| `TELEMETRY_MAX_POINTS_PER_MESSAGE` / `TELEMETRY_MAX_TAGS_PER_MESSAGE` | Bounds inner `telemetry.payload` JSON complexity. |
| `TELEMETRY_PER_MACHINE_MSGS_PER_SEC` / `TELEMETRY_PER_MACHINE_BURST` | Token-bucket rate limit per machine UUID (telemetry only). |
| `TELEMETRY_GLOBAL_MAX_INFLIGHT` | Bounded channel capacity for queued ingest jobs. |
| `TELEMETRY_WORKER_CONCURRENCY` | Worker goroutines draining the queue. |
| `TELEMETRY_DROP_ON_BACKPRESSURE` | When `true`, a full queue drops droppable work with metric `avf_telemetry_ingest_dropped_total{reason="droppable_queue_full"}` instead of blocking. |
| `TELEMETRY_SUBMIT_WAIT_MS` | When drop mode is `false`, max wait to enqueue before `queue_full_timeout` rejection. |

## Device replay expectations

This process enforces ingress limits, but reconnect-storm safety must begin on the device/app side.

Required offline replay policy:

- persist the offline queue durably before attempting network send
- include replay identity on each event:
  - `machine_id`
  - `event_id` or `boot_id` + `seq_no`
  - `emitted_at`
  - `event_type`
  - `idempotency_key`
- apply deterministic initial replay jitter of `0-300` seconds per machine
- replay at `1-5` events/sec per machine
- use replay batch sizes of `20-50`
- use exponential backoff with jitter after failed sends

Criticality classes enforced at mqtt-ingest:

- `critical_no_drop`: vend success/failure/result, payment/cashless/refund, cash inserted/payout/collection, inventory delta/refill/adjustment, command ack / receipt, and critical incidents such as jam, door open, motor fault, and temperature critical
- `compactable_latest`: latest-state style events such as snapshots and shadow/state updates; mqtt-ingest may compact these by machine + event key while they are still queued
- `droppable_metrics`: heartbeat, high-frequency metrics, and debug/noise telemetry; mqtt-ingest may drop these under queue backpressure

Critical vs droppable guidance:

- retry until acknowledged: `critical_no_drop` events
- compact when superseded by a newer equivalent state: `compactable_latest` events
- drop when the bounded queue is full: `droppable_metrics` events

Backpressure outcomes:

- dropped droppable events increment `avf_telemetry_ingest_dropped_total{reason="droppable_queue_full"}`
- `critical_no_drop` telemetry and command receipts bypass the async queue: failures increment `avf_telemetry_ingest_rejected_total{reason="handler_error"}` (or `invalid_payload` from the bridge when inner bounds fail)
- per-machine rate limit denials increment `avf_telemetry_ingest_rate_limited_total` and `avf_telemetry_ingest_rejected_total{reason="rate_limited"}`
- compactable events may still see `avf_telemetry_ingest_rejected_total{reason="compactable_queue_full"}` or `_timeout` under pressure
- accepted critical events increment `avf_telemetry_ingest_received_total{channel="critical_no_drop"}` (and `channel="telemetry"` or `command_receipt` as applicable) **after** successful handling

Ingress throttles are not a substitute for device-side pacing; they are the final guardrail after correct replay shaping on the device.

## Metrics (Prometheus)

When `METRICS_ENABLED=true` and `MQTT_INGEST_METRICS_LISTEN` is set, scrape `/metrics` on the mqtt-ingest process. Useful series:

- `avf_telemetry_ingest_received_total{channel=...}`
- `avf_telemetry_ingest_rejected_total{reason=...}`
- `avf_telemetry_ingest_dropped_total{reason=...}`
- `avf_telemetry_ingest_queue_depth`
- `avf_telemetry_ingest_publish_failures_total`
- `avf_telemetry_ingest_rate_limited_total`
- `avf_telemetry_ingest_payload_bytes` (histogram)
- `avf_mqtt_ingest_dispatch_total{kind,result}` (Dispatch outcome)

Definitions live in `internal/app/telemetryapp/mqtt_ingest_prom.go`.

## Storm load test accounting

`deployments/prod/scripts/telemetry_storm_load_test.sh` (see [telemetry-production-rollout.md](./telemetry-production-rollout.md)) certifies **critical** traffic using the **`avf_telemetry_ingest_received_total{channel="critical_no_drop"}`** counter delta versus the script’s planned critical publish count, and **`avf_mqtt_ingest_dispatch_total{kind="telemetry",result="ok"}`** versus total MQTT publishes (including planned duplicate replays). If mqtt-ingest `/metrics` is not reachable for both snapshots, default **`STRICT_ACCOUNTING=true`** forces a **FAIL** so large-fleet scale-up is not signed off without observability.

## Tuning for a small VPS

Defaults target a single-node profile: modest per-machine RPS, bounded queue depth, and drop-on-backpressure enabled so broker spikes cannot grow unbounded goroutine stacks or stall the MQTT client forever. Raise `TELEMETRY_GLOBAL_MAX_INFLIGHT` only if JetStream and Postgres connectivity keep up under load tests.
