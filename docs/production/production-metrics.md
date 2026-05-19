# Production metrics (P1.1)

Canonical Prometheus series live in `internal/platform/observability/productionmetrics` and are registered at process init (`promauto`). Legacy `avf_*` namespaced metrics may still exist alongside them during migration; dashboards should prefer the stable names below.

## HTTP

| Metric | Labels | Notes |
|--------|--------|-------|
| `http_requests_total` | method, route, status | Completed responses |
| `http_request_duration_seconds` | method, route, status | Histogram |
| `http_errors_total` | method, route, status | Status ≥ 400 |

## gRPC

| Metric | Labels | Notes |
|--------|--------|-------|
| `grpc_requests_total` | service, method, grpc_code | Unary completion |
| `grpc_request_duration_seconds` | service, method, grpc_code | Histogram |
| `grpc_errors_total` | service, method, grpc_code | Non-OK |
| `grpc_auth_failures_total` | reason | Credential validation only |
| `grpc_idempotency_replays_total` | — | Idempotent replay |
| `grpc_idempotency_conflicts_total` | — | Payload conflict |

## Machine / commerce / MQTT / inventory / outbox / audit

See `metrics.go` for full histogram/counter definitions including:

- Machine: `machine_checkins_total`, `machine_last_seen_age_seconds`, `machine_offline_events_total`, `machine_offline_replay_failures_total`, `machine_sync_lag_seconds`
- Commerce: `orders_created_total`, `vend_success_total`, `vend_failure_total`, `payment_webhooks_total`, `payment_webhook_rejections_total`, `payment_webhook_amount_currency_mismatch_total`, `reconciliation_cases_open_total`, `refunds_requested_total`, `refunds_failed_total`
- Reconciler (also `avf_commerce_*`): `payment_provider_probe_stale_pending_queue` — rows selected as past payment pending-timeout on last probe tick (gauge; `job="avf_reconciler_metrics"`).
- MQTT commands: `commands_*`, `command_ack_latency_seconds`, `command_retry_total`
- Inventory: `inventory_adjustments_total`, `inventory_negative_stock_attempts_total`, `inventory_reconciliation_cases_total`
- Outbox/NATS: `outbox_*`, `outbox_lag_seconds`
- Audit: `audit_events_total`, `audit_write_failures_total`

Mirror calls from legacy `avf_*` namespaced packages forward into these canonical names where applicable so operations can migrate dashboards without losing history.

## Alerts reference

Production Prometheus rules: `deployments/prod/observability/prometheus/alerts.yml`. On-call narrative: `docs/runbooks/observability-alerts.md`.

### Readiness / data plane

Blackbox probes on `avf_data_node_tcp` / `avf_data_node_http` cover **PostgreSQL**, **Redis**, **NATS**, and **EMQX** (see alert group `avf-edge-and-readiness` and `avf-data-node-and-host`). Composite rule **`AVFDataPlaneDependencyDegraded`** fires when any of Postgres/Redis/NATS TCP probes fail together for several minutes.

### Payment webhooks (HMAC route)

| Metric | When it moves |
|--------|----------------|
| `payment_webhook_rejections_total{reason="webhook_hmac_invalid"}` | Signature verification failed |
| `payment_webhook_rejections_total{reason="webhook_timestamp_skew"}` | Timestamp outside skew window |
| `payment_webhook_amount_currency_mismatch_total` | Body amount/currency ≠ `payments` row (reconciliation case opened) |

### MQTT commands (legacy `avf_mqtt_*` mirrored in API)

Prefer `commands_*`, `command_ack_latency_seconds`, and `command_retry_total` on `avf_api_metrics`; cross-check `avf_mqtt_command_ack_timeout_total` for timeout rate alerts.

### Catalog / media (gRPC)

Use `grpc_errors_total{service=~".*MachineCatalogService.*|.*MachineMediaService.*"}` for sustained catalog/media failures (no extra process-local counter required).

## Structured logs / audit correlation

- Every HTTP request should carry **`X-Request-ID`** (`internal/middleware.RequestID`).
- Structured logs (`http_request`, `api_error_response`) include **`trace_id`** when OpenTelemetry is active, **`request_id`** from the middleware, and **`correlation_id`** (trace id preferred, otherwise request id) for joining Loki / log queries to API errors and enterprise audit entries that copy request metadata from context.
