# mqtt-ingest: telemetry ingress limits

This runbook describes environment variables that cap device MQTT telemetry before JetStream publish. They apply to `cmd/mqtt-ingest` and are validated in `internal/config`.

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
| `TELEMETRY_DROP_ON_BACKPRESSURE` | When `true`, a full queue drops with metric `avf_telemetry_ingest_dropped_total{reason="queue_full"}` instead of blocking. |
| `TELEMETRY_SUBMIT_WAIT_MS` | When drop mode is `false`, max wait to enqueue before `queue_full_timeout` rejection. |

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

## Tuning for a small VPS

Defaults target a single-node profile: modest per-machine RPS, bounded queue depth, and drop-on-backpressure enabled so broker spikes cannot grow unbounded goroutine stacks or stall the MQTT client forever. Raise `TELEMETRY_GLOBAL_MAX_INFLIGHT` only if JetStream and Postgres connectivity keep up under load tests.
