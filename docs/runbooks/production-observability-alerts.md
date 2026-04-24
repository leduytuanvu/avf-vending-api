# Production observability alerts and runbooks

This document pairs **Prometheus alert rules** in `deployments/prod/observability/prometheus/alerts.yml` with **on-call response**: severity, trigger, first investigations, mitigation, and ownership.

**Canonical paths**

- Alert rules: `deployments/prod/observability/prometheus/alerts.yml`
- Prometheus scrape config: `deployments/prod/observability/prometheus/prometheus.yml`
- Grafana dashboards: `deployments/prod/observability/grafana/provisioning/dashboards/json/`

**Severity mapping**

- **P0** — Revenue, safety, or durable audit trail at risk; page immediately.
- **P1** — Degraded path or rising risk; respond within business SLA; page if SLO burn is high.
- **P2** — Early warning or capacity signal; ticket + dashboard review.

Prometheus `labels.severity` uses `critical` / `warning`. Treat **`critical` as P0–P1** and **`warning` as P1–P2** per alert notes below.

**Owner / team:** Platform SRE — AVF vending API (replace with your internal roster and escalation policy).

---

## Monitoring readiness check

Operators can verify that **live** `/metrics` endpoints expose the expected **`avf_*`** series and that **`/health/ready`** responds **before** scaling the fleet or treating production as enterprise-ready.

When **`METRICS_SCRAPE_TOKEN`** is configured, Prometheus (or curl tests) must send **`Authorization: Bearer <token>`** for **`/metrics`** on the API ops port; **`/health/*`** stays unauthenticated on that mux.

```bash
# From a host that reaches private ops ports (example hosts/ports — use yours).
export API_METRICS_URL="http://127.0.0.1:8081/metrics"
export MQTT_INGEST_METRICS_URL="http://127.0.0.1:9093/metrics"
export WORKER_METRICS_URL="http://127.0.0.1:9091/metrics"
# Optional: Prometheus / Alertmanager UI probes (non-fatal if unset or unreachable)
# export PROMETHEUS_URL="http://127.0.0.1:9090"
# export ALERTMANAGER_URL="http://127.0.0.1:9094"

bash deployments/prod/scripts/check_monitoring_readiness.sh
```

The script writes **`monitoring-readiness-result.json`** (override with `MONITORING_READINESS_RESULT_FILE`) with `final_result` **`pass`** or **`fail`**, `missing_metrics`, `unreachable_endpoints`, and `deferred_metric_groups` for signals that are **not** yet emitted by the app (Postgres pool saturation, container restarts — see TODO table below). It does **not** print URLs with embedded credentials.

Health URLs default to the same host as each `*_METRICS_URL` with `/metrics` replaced by `/health/ready`. If your API readiness is on a different port than metrics, set **`API_HEALTH_READY_URL`** (and similarly **`MQTT_INGEST_HEALTH_READY_URL`**, **`WORKER_HEALTH_READY_URL`**). Set **`RECONCILER_HEALTH_READY_URL`** only when the reconciler runs and you want that probe included.

Canonical metric prefixes match this document (**`avf_telemetry_ingest_*`**, **`avf_mqtt_ingest_dispatch_total`**, worker **`avf_telemetry_*`**); the readiness script searches the **scraped text** for these names (not synthetic test values).

---

## Metric names (application-exposed)

These series are defined in Go (`internal/app/telemetryapp/*_prom.go`, `internal/observability/reconcilerprom/`). Use **exact** names in dashboards and alerts.

| Metric | Job label (`job`) | Notes |
| --- | --- | --- |
| `avf_telemetry_ingest_queue_depth` | `avf_mqtt_ingest_metrics` | Bounded mqtt-ingest queue gauge |
| `avf_telemetry_ingest_dropped_total{reason}` | `avf_mqtt_ingest_metrics` | e.g. `droppable_queue_full` |
| `avf_telemetry_ingest_rejected_total{reason}` | `avf_mqtt_ingest_metrics` | Includes `critical_*`, `handler_error`, `rate_limited`, etc. |
| `avf_telemetry_ingest_critical_missing_identity_total` | `avf_mqtt_ingest_metrics` | Critical path without dedupe identity |
| `avf_telemetry_ingest_rate_limited_total` | `avf_mqtt_ingest_metrics` | Token-bucket denials |
| `avf_telemetry_ingest_publish_failures_total` | `avf_mqtt_ingest_metrics` | JetStream publish failures |
| `avf_mqtt_ingest_dispatch_total{kind,result}` | `avf_mqtt_ingest_metrics` | Dispatch outcomes |
| `avf_telemetry_consumer_lag{stream,durable}` | `avf_worker_metrics` | JetStream `NumPending` |
| `avf_telemetry_projection_failures_total{reason}` | `avf_worker_metrics` | e.g. `handler_err`, `fetch_err` |
| `avf_telemetry_projection_db_fail_consecutive_max` | `avf_worker_metrics` | Consecutive DB/handler failures |
| `avf_telemetry_duplicate_total{reason}` | `avf_worker_metrics` | Includes `stream_seq`, `idempotency_replay` (redelivery **proxy**, not broker redeliveries) |
| `avf_telemetry_idempotency_conflict_total` | `avf_worker_metrics` | Same idempotency key, different payload hash |
| `avf_worker_outbox_oldest_pending_age_seconds` | `avf_worker_metrics` | Outbox lag |
| `avf_worker_outbox_dispatch_publish_failed_total` | `avf_worker_metrics` | NATS publish failures from worker outbox |
| `avf_reconciler_job_failures_total{reconciler_job}` | `avf_reconciler_metrics` | Per-job failures |
| `avf_reconciler_cycle_completions_total{reconciler_job,result}` | `avf_reconciler_metrics` | Tick outcomes |

Blackbox and `node_exporter` metrics depend on your compose / host layout (see `prometheus.yml`).

---

## TODO: metrics not emitted by the application

Do **not** treat the following as live alert conditions until you add an exporter or log-based metrics. Document them here so operators know the gap.

| Desired signal | Status | Practical approach |
| --- | --- | --- |
| Postgres pool **saturation** (% of `max_conns` or pooler slots) | **TODO** | Expose `pgxpool` stats via a small custom metric endpoint, or scrape `postgres_exporter` / pooler stats; compare to managed pooler limit (see [production-2-vps.md](./production-2-vps.md)). |
| Postgres **pool acquire** errors | **TODO** | Log grep `failed to connect` / `pool` or add counter on acquire timeout in app (future code). |
| Container **restart count** / **OOMKilled** | **TODO** | `cadvisor`, Kubernetes `kube_pod_container_status_*`, or Docker event logs on the app node. |
| JetStream **consumer redeliveries** (exact count) | **TODO** | `nats consumer info` / monitoring API; app only exposes duplicate suppression via `avf_telemetry_duplicate_total`. |

---

## Investigation commands (templates)

Adjust hostnames and compose files to your node (`deployments/prod/app-node`, `deployments/prod/data-node`).

1. **Service logs (last 15–30 min):** `docker compose -f deployments/prod/app-node/docker-compose.app-node.yml logs mqtt-ingest --since 30m --tail 300` (or `worker`, `api`, `reconciler`).
2. **Prometheus instant query:** open Prometheus UI → Graph → paste the alert expression without `for` window or use `rate(...[5m])` for counters.
3. **NATS consumer backlog:** on data node, `nats consumer report` / `nats consumer info <stream> <durable>` (see [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md)).
4. **Postgres:** from bastion or app node, `psql "$DATABASE_URL" -c 'select now();'` and check managed dashboard for connection count vs limit.
5. **Readiness:** `curl -fsS http://127.0.0.1:<ops-port>/health/ready` on the private bind for the failing component (ports per compose).

---

## Telemetry pressure and correctness

### AVFTelemetryIngestQueueDepthHigh

| Field | Value |
| --- | --- |
| **Severity** | P1 (warning) |
| **Trigger** | `avg(avf_telemetry_ingest_queue_depth{job="avf_mqtt_ingest_metrics"}) > 200` for 10m |
| **Meaning** | mqtt-ingest bounded queue is persistently deep vs typical `TELEMETRY_GLOBAL_MAX_INFLIGHT` (often 256). |

**First investigations**

1. Graph `avf_telemetry_ingest_queue_depth` and `rate(avf_telemetry_ingest_received_total[5m])` by channel.
2. Check `avf_telemetry_ingest_dropped_total` and `avf_telemetry_ingest_rejected_total` rates by reason.
3. Inspect mqtt-ingest CPU and single-host affinity; confirm no deploy stuck.
4. Check EMQX connection storm or device replay (see [mqtt-ingest-telemetry-limits.md](./mqtt-ingest-telemetry-limits.md)).
5. Verify JetStream publish latency is not stalling critical inline path.

**Rollback / mitigation**

- Temporarily reduce fleet replay (device-side) or raise `TELEMETRY_GLOBAL_MAX_INFLIGHT` / workers **only** after confirming memory headroom.
- Scale mqtt-ingest horizontally only if supported by your deployment model; otherwise add app node capacity.
- If JetStream is slow, follow stream disk / replica guidance in [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md).

---

### AVFTelemetryDroppableDropsElevated

| Field | Value |
| --- | --- |
| **Severity** | P1 (warning) |
| **Trigger** | `sum(rate(avf_telemetry_ingest_dropped_total{reason="droppable_queue_full"}[10m])) > 5` for 15m |
| **Meaning** | Droppable telemetry is dropped under queue pressure — acceptable for metrics class but indicates **replay storm** or undersized queue. |

**First investigations**

1. Correlate with `avf_telemetry_ingest_queue_depth` and `avf_telemetry_ingest_rate_limited_total`.
2. Check device firmware batching and offline replay rate per rollout doc.
3. Inspect EMQX receive rate and mqtt-ingest logs for `mqtt_ingest_dropped_droppable`.
4. Compare to `AVFTelemetryConsumerLagCritical` — worker may be behind.
5. Review recent config change to `TELEMETRY_PER_MACHINE_*` or `DropOnBackpressure`.

**Rollback / mitigation**

- Slow replay at source; widen per-machine limits if legitimate burst.
- Increase ingest concurrency / queue only with memory monitoring.

---

### AVFTelemetryCriticalIngressRejected

| Field | Value |
| --- | --- |
| **Severity** | P0 (critical) |
| **Trigger** | Sustained `rate` of `avf_telemetry_ingest_rejected_total` for reasons `critical_queue_full`, `critical_queue_full_timeout`, `critical_missing_idempotency_identity` |
| **Meaning** | **Critical-path data** is not entering the durable pipeline at expected rate — **audit / vend / payment evidence** may be lost at the edge. |

**First investigations**

1. Break down by `reason` label in Prometheus.
2. If `critical_queue_full*`, treat as capacity: same as queue depth + worker lag.
3. If `critical_missing_idempotency_identity`, identify firmware / client versions omitting identity fields.
4. Check `avf_telemetry_ingest_publish_failures_total` and NATS health.
5. Read mqtt-ingest logs for `mqtt_ingest_retryable_backpressure` and `critical_failed`.

**Rollback / mitigation**

- Restore capacity (worker + JetStream + mqtt-ingest) before changing business logic.
- Hotfix client if identity fields missing.
- Roll back recent deploy if regression.

---

### AVFTelemetryCriticalMissingIdentity

| Field | Value |
| --- | --- |
| **Severity** | P0 (critical) |
| **Trigger** | `increase(avf_telemetry_ingest_critical_missing_identity_total[15m]) > 0` |
| **Meaning** | Critical telemetry reached the bridge without `event_id` / `dedupe_key` / `boot_id+seq` — **cannot** be safely deduped; events are rejected. |

**First investigations**

1. Sample raw MQTT payloads for offending `machine_id` (EMQX trace / debug subscription — **privacy / production policy**).
2. Map device app version and OTA channel.
3. Confirm schema docs shipped with firmware.
4. Check for partial publishes or middleware stripping fields.
5. Correlate with a single operator or site if isolated.

**Rollback / mitigation**

- Block bad OTA cohort; force fixed build.
- If server-side regression, roll back mqtt-ingest / bridge commit.

---

### AVFTelemetryRateLimitedSpike

| Field | Value |
| --- | --- |
| **Severity** | P2 (warning) |
| **Trigger** | `sum(rate(avf_telemetry_ingest_rate_limited_total[10m])) > 10` for 15m |
| **Meaning** | Per-machine token bucket is denying traffic — replay storm or **too-tight** `TELEMETRY_PER_MACHINE_*`. |

**First investigations**

1. Compare `rate_limited` to `avf_telemetry_ingest_rejected_total{reason="rate_limited"}` (both increment on limit).
2. Histogram of offending machines: parse logs or use `machine_id` if logged.
3. Check fleet-wide replay after outage.
4. Review limit env vars in [mqtt-ingest-telemetry-limits.md](./mqtt-ingest-telemetry-limits.md).
5. Confirm no integration test devices in production loop.

**Rollback / mitigation**

- Tune per-machine rate / burst temporarily.
- Stop abusive replay at source.

---

### AVFTelemetryJetStreamPublishFailures

| Field | Value |
| --- | --- |
| **Severity** | P0 (critical) |
| **Trigger** | `sum(rate(avf_telemetry_ingest_publish_failures_total[10m])) > 0.05` for 5m |
| **Meaning** | mqtt-ingest cannot publish to JetStream — backlog and device retries follow. |

**First investigations**

1. NATS / JetStream logs on data node; disk full; stream sealed.
2. `nats stream info` for telemetry stream; quorum / replicas.
3. Network from app node to `4222` / TLS route.
4. Recent stream config change (`TELEMETRY_STREAM_MAX_BYTES`, etc.).
5. Correlated `AVFNATSDown` or host disk alerts.

**Rollback / mitigation**

- Free disk; extend JetStream limits per resilience runbook.
- Fail over to standby broker plane if you run split topology.
- Pause ingest only as last resort — devices buffer; plan for replay.

---

### AVFTelemetryHandlerRejectRate

| Field | Value |
| --- | --- |
| **Severity** | P1 (warning) |
| **Trigger** | `rate(avf_telemetry_ingest_rejected_total{reason="handler_error"}[10m])` sustained high |
| **Meaning** | Critical inline handler path failing (validation, bridge, inner ingest). |

**First investigations**

1. mqtt-ingest logs: `mqtt_ingest_critical_failed`, stack traces.
2. NATS creds / connection limit.
3. Recent deploy diff for `cmd/mqtt-ingest` or bridge.
4. Postgres dependency if handler touches DB in critical path.
5. Sample failing topic kinds via `avf_mqtt_ingest_dispatch_total` error rate by `kind`.

**Rollback / mitigation**

- Roll back broken release.
- Fix broker or credential outage.

---

### AVFTelemetryConsumerLagCritical

| Field | Value |
| --- | --- |
| **Severity** | P0 (critical) |
| **Trigger** | `max(avf_telemetry_consumer_lag{job="avf_worker_metrics"}) > 10000` for 15m |
| **Meaning** | Worker durable consumer `NumPending` very high — projection cannot keep up (**replay storm** or DB slow). |

**First investigations**

1. Worker CPU, GOMAXPROCS, replica count.
2. `avf_telemetry_projection_flush_seconds` and batch sizes.
3. Postgres latency and lock waits for telemetry tables.
4. Compare with mqtt-ingest publish rate.
5. JetStream storage / memory on data node.

**Rollback / mitigation**

- Scale workers; reduce batch work; optimize hot queries.
- Throttle source replay if necessary.
- Address DB contention before raising stream retention blindly.

---

### AVFTelemetryProjectionHandlerErrors

| Field | Value |
| --- | --- |
| **Severity** | P1 (warning) |
| **Trigger** | Sustained `rate(avf_telemetry_projection_failures_total{reason="handler_err"}[10m])` |
| **Meaning** | Projection handler errors — **Nak / redelivery** pressure and delayed truth in read models. |

**First investigations**

1. Worker logs: `telemetry_handler_error`.
2. Postgres errors in same timeframe.
3. Malformed payload rate: `reason="malformed_json"`.
4. Fetch errors: `reason="fetch_err"` (consumer / network).
5. Compare `avf_telemetry_projection_db_fail_consecutive_max`.

**Rollback / mitigation**

- Fix schema / DB migration mismatch.
- Roll back bad worker release.

---

### AVFTelemetryDuplicateReplayPressure

| Field | Value |
| --- | --- |
| **Severity** | P2 (warning) |
| **Trigger** | Elevated `rate(avf_telemetry_duplicate_total{reason=~"stream_seq|idempotency_replay"})` |
| **Meaning** | **Proxy** for JetStream redelivery / duplicate delivery — not a 1:1 broker redelivery metric. |

**First investigations**

1. Recent worker restarts or consumer leader elections.
2. Consumer ack timing vs handler latency.
3. Broker redelivery counters (TODO metric) via `nats consumer info`.
4. Device duplicate publish behavior.
5. Idempotency store latency.

**Rollback / mitigation**

- Stabilize worker; fix handler slowness causing ack timeouts.
- Tune consumer ack wait / max ack pending per NATS guidance.

---

### AVFTelemetryIdempotencyConflict

| Field | Value |
| --- | --- |
| **Severity** | P0 (critical) |
| **Trigger** | `increase(avf_telemetry_idempotency_conflict_total[10m]) > 0` |
| **Meaning** | Same `idempotency_key` with **different** payload hash — risk of **duplicate business effect** if downstream ever mis-handles. |

**First investigations**

1. Extract key + machine from logs / DB telemetry idempotency table.
2. Determine if bug reuses ids across distinct events.
3. Check for man-in-the-middle or double-encoding in client.
4. Review recent idempotency schema or hash algorithm change.
5. Audit whether any side effect already committed (commerce / inventory).

**Rollback / mitigation**

- Freeze suspect client version; open incident for finance / ops.
- Patch server to reject unsafe keys if needed (requires code change — out of runbook scope).

---

## Reconciler — payment / vend signals

These alerts are **workflow / data-quality** signals, not a full accounting audit. Confirm money movement with PSP and internal ledger queries.

### AVFReconcilerPaymentProbeFailures

| Field | Value |
| --- | --- |
| **Severity** | P1 (warning) |
| **Trigger** | `increase(avf_reconciler_job_failures_total{reconciler_job="payment_provider_probe"}[30m]) >= 3` |
| **Meaning** | Payment provider probe job is failing rows — PSP unreachable, schema drift, or DB read errors. |

**First investigations**

1. Reconciler logs for `payment_provider_probe`.
2. PSP status page and API credentials expiry.
3. `psql` spot-check pending payments vs provider state.
4. Network egress from app node.
5. Recent deploy touching commerce schema.

**Rollback / mitigation**

- Restore PSP connectivity; rotate keys.
- Roll back schema-breaking migration (coordinate with migrations policy).

---

### AVFReconcilerRefundReviewFailures

| Field | Value |
| --- | --- |
| **Severity** | P1 (warning) |
| **Trigger** | `increase(avf_reconciler_job_failures_total{reconciler_job="refund_review"}[30m]) >= 1` |
| **Meaning** | Refund review loop failing — refunds may stall. |

**First investigations**

1. Reconciler logs; stack traces.
2. List rows in refund review query (read-only).
3. PSP refund API errors.
4. Temporal / workflow backlog if refunds are workflow-driven.
5. Permissions / RLS on new tables.

**Rollback / mitigation**

- Fix data row blocking query.
- Manual refund playbook if time-critical.

---

### AVFReconcilerDuplicatePaymentEnqueueFailures

| Field | Value |
| --- | --- |
| **Severity** | P1 (warning) |
| **Trigger** | `increase(avf_reconciler_job_failures_total{reconciler_job="duplicate_payments"}[30m]) >= 3` |
| **Meaning** | Duplicate-payment recovery path failing to enqueue or read. |

**First investigations**

1. NATS / workflow bus health.
2. DB query errors in reconciler logs.
3. Compare with `AVFWorkerPublishFailuresHigh` if enqueue uses outbox.
4. Row-level failures vs total job failure.
5. Recent change to duplicate detection SQL.

**Rollback / mitigation**

- Restore broker; fix SQL.
- Pause automation and run manual duplicate sweep **only** with finance sign-off.

---

### AVFReconcilerVendStuckCycleErrors

| Field | Value |
| --- | --- |
| **Severity** | P1 (warning) |
| **Trigger** | `increase(avf_reconciler_cycle_completions_total{reconciler_job="vend_stuck",result="error"}[1h]) >= 3` |
| **Meaning** | **Reconciler tick** errors (list/DB), not the raw count of stuck vends. For **payment/vend mismatch**, also run **TODO: SQL reconciliation** between `vend_sessions`, `orders`, and payments (documented in commerce ops runbooks). |

**First investigations**

1. Reconciler logs for `vend_stuck`.
2. DB connectivity and locks.
3. Stuck vend query timeout / large table scan.
4. Compare alert time to deploy window.
5. Sample stuck rows count via admin SQL (read-only).

**Rollback / mitigation**

- Fix DB performance or index regression.
- Manual vend resolution per ops playbook.

---

## Queue, worker, and mqtt-ingest (core platform)

### AVFWorkerOutboxLagHigh

| Field | Value |
| --- | --- |
| **Severity** | P2 (warning) |
| **Trigger** | `max(avf_worker_outbox_oldest_pending_age_seconds) > 300` for 10m |
| **Meaning** | Outbox not draining — side effects delayed. |

**First investigations:** worker logs, NATS connectivity, outbox table growth, publish failure counter.

**Mitigation:** fix broker; scale worker; clear poison messages per playbook.

---

### AVFWorkerPublishFailuresHigh

| Field | Value |
| --- | --- |
| **Severity** | P0–P1 (critical) |
| **Trigger** | Sustained `rate(avf_worker_outbox_dispatch_publish_failed_total[10m]) > 0.1` |
| **Meaning** | Cannot publish domain events — downstream inconsistency. |

**First investigations:** NATS stream ACL, creds, stream full, worker errors.

**Mitigation:** restore JetStream; replay outbox after fix.

---

### AVFTelemetryQueueLagHigh

| Field | Value |
| --- | --- |
| **Severity** | P2 (warning) |
| **Trigger** | `max(avf_telemetry_consumer_lag) > 2000` for 10m |
| **Meaning** | Early backlog warning (lower threshold than critical lag alert). |

**Mitigation:** same family as `AVFTelemetryConsumerLagCritical` but earlier stage.

---

### AVFTelemetryProjectionDBFailures

| Field | Value |
| --- | --- |
| **Severity** | P0 (critical) |
| **Trigger** | `max(avf_telemetry_projection_db_fail_consecutive_max) >= 3` for 5m |
| **Meaning** | Repeated DB failure in projection — **readiness** likely failing for worker. |

**First investigations:** Postgres uptime, pool exhaustion (see TODO table), migration mismatch, worker logs.

**Mitigation:** restore DB capacity; roll back migration if applicable.

---

### AVFMQTTIngestErrorRateHigh

| Field | Value |
| --- | --- |
| **Severity** | P2 (warning) |
| **Trigger** | `sum(rate(avf_mqtt_ingest_dispatch_total{result="error"}[10m])) > 1` |
| **Meaning** | Dispatch errors across MQTT topics — investigate before it becomes critical rejects. |

**Mitigation:** broker stability, topic ACL, payload validation.

---

### AVFMQTTIngestReadinessFlapping

| Field | Value |
| --- | --- |
| **Severity** | P2 (warning) |
| **Trigger** | Frequent `changes(probe_success{component="mqtt-ingest"})` |
| **Meaning** | Intermittent readiness — deploy, broker, or dependency flapping. |

**Mitigation:** stabilize network; finish or roll back deploy.

---

## Edge, readiness, and database correlation

### AVFPublicAPIHTTPSDown

| **Severity** | P0 |
| **Trigger** | Blackbox `probe_success{job="avf_public_api_https"} == 0` |
| **Investigations:** Caddy, DNS, TLS cert, upstream API pods, DDoS. **Mitigation:** fail over edge; restore API. |

---

### AVFAPIInstanceMetricsDown

| **Severity** | P1 |
| **Trigger** | `up{job="avf_api_metrics"} == 0` |
| **Investigations:** ops port 8081, firewall, process crash. **Mitigation:** restart `cmd/api`; fix scrape config. |

---

### AVFAppReadinessFailed

| **Severity** | P0 when `component=api`; P1 for other components |
| **Trigger** | Blackbox readiness probe failure per component |
| **Investigations:** hit private readiness URL; logs; DB ping failures inside `READINESS_STRICT`. **Mitigation:** fix dependency; roll back. |

---

### AVFDeployUnhealthy

| **Severity** | P2 |
| **Trigger** | Any readiness failing on a node for 5m |
| **Meaning** | Pause rolling deploy; node not stable. |

---

### AVFDatabaseConnectivityLikelyFailed

| **Severity** | P0 |
| **Trigger** | Multiple Postgres-dependent components failing readiness on same node |
| **Investigations:** Postgres incident, pooler saturation, wrong `DATABASE_URL`, network partition. **Mitigation:** scale pooler; fail over DB; reduce `DATABASE_MAX_CONNS` temporarily. |

**TODO alert:** explicit **70% pool capacity** — add when `postgres_exporter` or custom pool metrics exist.

---

## Data plane and host

### AVFNATSDown / AVFEMQXDown

| **Severity** | P0 |
| **Investigations:** data-node containers, disk, TLS certs (EMQX), firewall from app nodes. **Mitigation:** restart broker; restore quorum; follow split-VPS runbooks. |

---

### AVFHostMemoryPressure / AVFHostDiskPressure

| **Severity** | P2 |
| **Meaning** | Risk of **OOM** (see TODO for exact OOM metric). **Mitigation:** clear logs; expand disk; reduce retention; move JetStream / Loki volume. |

---

## Alert rule maintenance

- Re-run `promtool check rules deployments/prod/observability/prometheus/alerts.yml` in CI or before deploy.
- Tune thresholds after 2 weeks of baseline at ~1000 machines.
- When adding `postgres_exporter`, add a dedicated `group` in `alerts.yml` and remove the TODO rows from the table above.
