# Operations runbook (AVF vending API)

Concise notes for **production support**. Code paths are in this repo unless noted.

**Ownership (where to look first):**

| Symptom area | Application code | Persistence | Processes |
| ------------ | ----------------- | ------------- | --------- |
| Operator login/logout/heartbeat, attribution | `internal/app/operator`, `internal/httpserver/operator_http.go` | `machine_operator_sessions`, `machine_operator_auth_events`, `machine_action_attributions` | `cmd/api` |
| Bearer JWT mode / JWKS / rate limits | `internal/platform/auth`, `internal/config` (`HTTPAuthConfig`), `internal/httpserver/middleware_rate_limit.go` | N/A | `cmd/api` |
| Backend artifacts (S3 uploads, presigned download) | `internal/app/artifacts`, `internal/httpserver/artifacts_http.go`, `internal/platform/objectstore` | S3 bucket via `S3_BUCKET` / `AWS_*` | `cmd/api` when `API_ARTIFACTS_ENABLED=true` |
| Outbox publish / retries | `internal/app/background/worker.go` (`OutboxDispatchTick`), `internal/app/reliability` | `outbox_events` | `cmd/worker` |
| Broker (NATS JetStream) | `internal/platform/nats`, `cmd/worker/main.go` | N/A (broker) | `cmd/worker` when `NATS_URL` set |
| Commerce reconciliation (lists; optional PSP probe + refund enqueue when enabled) | `internal/app/background/reconciler.go`, `internal/bootstrap/reconciler.go`, `internal/modules/postgres` commerce reconcile repo | orders, payments, vend sessions | `cmd/reconciler` (+ Prometheus `avf_reconciler_*` when `METRICS_ENABLED`) |
| Temporal workflow follow-up (payment timeout, vend failure, refund/manual review) | `internal/app/workfloworch`, `internal/platform/temporal`, `cmd/temporal-worker` | authoritative state still in Postgres; review/refund fan-out uses existing NATS refund-review sink | `cmd/temporal-worker` |
| Device MQTT ingest | `internal/platform/mqtt`, `internal/app/telemetryapp` (NATS bridge), `internal/modules/postgres` | JetStream `AVF_TELEMETRY_*` buffers + projected tables (`machine_current_snapshot`, `telemetry_rollups`, …); legacy hot path only without `NATS_URL` | `cmd/mqtt-ingest` (+ `avf_mqtt_ingest_*` when `METRICS_ENABLED`); see `ops/TELEMETRY_PIPELINE.md` |

---

## 1. Operator session issues

**Symptoms:** machine UI cannot log in; `409 active_session_exists`; heartbeats failing; wrong technician attributed; org-admin cannot revoke.

**HTTP (client-facing):** routes under `/v1/machines/{machineId}/operator-sessions/*`. JSON errors use `error.code` (e.g. `active_session_exists`, `session_not_active`, `technician_not_assigned`, `invalid_session_id`, `admin_takeover_forbidden`). See `internal/httpserver/operator_http.go` and `writeOperatorError`.

**Lifecycle (field behavior):**

- **Same principal** calling `POST .../login` while their session is still `ACTIVE` **resumes** that row (updates `last_activity_at` / optional `expires_at` / `client_metadata`) and may append `session_refresh` to `machine_operator_auth_events`.
- **Different principal** sees `409 active_session_exists` until the prior session is **idle** longer than the server reclaim window (no heartbeat/login touching `last_activity_at`), after which login **ends** the old row (`ended_reason` = `stale_session_reclaimed`, status `ENDED`) and opens a new `ACTIVE` session.
- **Org/platform admin** may set `force_admin_takeover` on login to **revoke** the current session (`ended_reason` = `admin_forced_takeover`, status `REVOKED`) and open a new one without waiting for idle.

**Checks:**

1. **Current session:** `GET .../operator-sessions/current` — confirms whether an `ACTIVE` row exists for that machine.
2. **Stuck ACTIVE:** DB invariant — at most one `ACTIVE` per `machine_id` (`ux_machine_operator_sessions_one_active`). If two ACTIVE rows ever appear, that is a **data integrity** incident (not normal app behavior).
3. **Logout without body session_id:** allowed only when an active session exists; otherwise `400 session_id_required`.
4. **Technician on machine:** `technician_not_assigned` → fleet assignments + JWT principal; assignment checker is required in production (`operator.NewService`).

**SQL (examples, tune IDs):**

```sql
-- Sessions for a machine (recent first)
SELECT id, status, actor_type, technician_id, user_principal, started_at, last_activity_at, ended_at, ended_reason
FROM machine_operator_sessions
WHERE machine_id = $1
ORDER BY started_at DESC
LIMIT 20;

-- Active session only
SELECT * FROM machine_operator_sessions WHERE machine_id = $1 AND status = 'ACTIVE';
```

**Logs:** API access logs + JWT subject; correlate with `X-Request-ID` / `X-Correlation-ID` (see `internal/middleware/requestid.go`).

---

## 2. Stuck / growing unpublished outbox

**Symptoms:** downstream consumers starved; `outbox_events.published_at` stays null; worker logs show high `outbox_pending_total` or `outbox_pipeline_unhealthy`.

**Mechanics:** `cmd/worker` runs `OutboxDispatchTick` on a fixed interval (default 3s). **Poison / quarantine:** after each failed JetStream publish, `publish_attempt_count` increments and `next_publish_after` applies exponential backoff; when the next count would reach **`OutboxMaxPublishAttempts`** (default **24** after `NormalizeRecoveryPolicy` unless overridden), Postgres sets **`dead_lettered_at`** and the row leaves the unpublished queue permanently. When NATS is wired, one copy is also published to **`AVF_INTERNAL_DLQ`** (`avf.internal.dlq.outbox_publish_exhausted`) with a **distinct `Nats-Msg-Id`** (`outbox-dlq-<id>`) so live outbox dedupe is unchanged. Each cycle logs `outbox_pipeline_snapshot` (debug) or **`outbox_pipeline_unhealthy`** (warn) when `dead_lettered_total > 0` or `max_pending_attempts >= 6`. Non-terminal failures log **`outbox publish failed`**; terminal quarantine logs **`outbox_dead_lettered`** (and **`outbox_dead_letter_dlq_publish_failed`** if the DLQ copy fails). Successful publishes log **`outbox_publish_lag_seconds`**. Summaries: **`worker_job_summary`** `job=outbox_dispatch`. After SIGTERM, tickers drain in-flight cycles then a **single bounded final outbox pass** runs (`worker_shutdown_outbox_drain_*`).

**Common causes:**

- **`skipped_no_publisher` > 0` and `NATS_URL` unset** — expected: rows stay in Postgres until you configure JetStream publish or another publisher implements `OutboxPublisher`.
- **Broker down / ACL / stream missing** — `publish_failed` increases; see `last_publish_error` on the row.
- **Batch cap** — `worker_batch_at_limit_may_lag` with `job=outbox_dispatch`: backlog may exceed `MaxItems` (200 in `cmd/worker`); next ticks continue but **lag** can grow if publish is slower than enqueue.

**SQL:**

```sql
-- Oldest unpublished, not dead-lettered
SELECT id, topic, event_type, created_at, publish_attempt_count, next_publish_after, dead_lettered_at, last_publish_error
FROM outbox_events
WHERE published_at IS NULL AND dead_lettered_at IS NULL
ORDER BY created_at ASC
LIMIT 50;

-- Dead-lettered (excluded from dispatch until manual fix)
SELECT id, topic, created_at, publish_attempt_count, dead_lettered_at, last_publish_error
FROM outbox_events
WHERE dead_lettered_at IS NOT NULL
ORDER BY dead_lettered_at DESC
LIMIT 20;
```

**Recovery:** fix root cause (broker, credentials, stream ensure). For mistaken dead-letters, treat as **manual DBA / app-owner** action after root-cause analysis (columns described in migration `00012_outbox_publish_pipeline.sql`).

---

## 3. Broker publish failure (NATS JetStream)

**Symptoms:** worker logs `outbox publish failed`; worker exits at startup with `nats connect` / `nats streams` fatal.

**Wiring:** `NATS_URL` non-empty → `ConnectJetStream`, `EnsureInternalStreams`, `NewJetStreamOutboxPublisher`. See `internal/platform/nats/doc.go` for subject/stream names.

**Checks:**

1. From worker host: reachability to `NATS_URL`, JetStream enabled, disk/resources on NATS.
2. Worker startup log: **`outbox jetstream publisher enabled`** vs no line (publisher nil).
3. **Dedup:** publish uses JetStream idempotency (`Nats-Msg-Id` / outbox id). Crash after publish but before DB mark → safe retry; duplicate side effects should be bounded for well-behaved consumers.

**Note:** this repo **publishes** only; there is **no** in-process JetStream consumer in `cmd/*`. If messages disappear “in NATS” but DB still shows unpublished, investigate **consumers outside this repo** or stream retention.

---

## 4. Reconciler lag

**Symptoms:** known bad commerce rows appear late in logs; repeated **`reconciler_batch_at_limit_may_lag`**; **`background_cycle_end`** with `result=cycle_deadline_exceeded`.

**Mechanics:** five jobs in `RunReconciler` — `unresolved_orders`, `payment_provider_probe`, `vend_stuck`, `duplicate_payments`, `refund_review`. Each tick logs **`background_cycle_start` / `background_cycle_end`** with `job` name and duration. Per-domain summaries: **`reconciler_job_summary`** with `job`, `selected`, `at_batch_limit`, `stable_before`, `batch_limit`.

**Important:** When **`RECONCILER_ACTIONS_ENABLED=false`** (default), bootstrap leaves **`Gateway`** and **`RefundSink`** unset — PSP fetch and refund enqueue **do not run**; `payment_provider_probe` and `refund_review` ticks are no-ops or list-only as designed. When **`RECONCILER_ACTIONS_ENABLED=true`**, `internal/bootstrap/reconciler.go` wires an HTTP payment probe gateway, NATS refund review sink, and Postgres payment applier — validate env at startup (`ValidateReconciler`). **Unresolved orders / vend stuck / duplicate payments** always emit list logs for operator follow-up regardless of actions mode.

**Lag drivers:** `batch_limit` (200) with sustained `at_batch_limit=true`; slow Postgres; `StableAge` (default 2m) excludes very new rows by design.

---

## 5. MQTT ingest health

**Symptoms:** device telemetry or shadow not updating; repeated **`mqtt ingest failed`**; process exit on `mqtt connect`.

**Process:** `cmd/mqtt-ingest` — broker from env (`internal/platform/mqtt` `LoadBrokerFromEnv`). Subscribes under the configured topic prefix to the full inbound device pattern set: `+/telemetry`, `+/presence`, `+/state/heartbeat`, `+/telemetry/snapshot`, `+/telemetry/incident`, `+/events/vend`, `+/events/cash`, `+/events/inventory`, `+/shadow/reported`, `+/shadow/desired`, `+/commands/receipt`, and `+/commands/ack`.

**Checks:**

1. Process up and connected (log **`mqtt subscribed`** per pattern).
2. **EMQX / broker** ACLs, credentials, TLS vs `mqtt://` URL.
3. **Topic prefix** matches devices (`TopicPrefix` trailing slash normalized in code).
4. Ingest errors: **`mqtt ingest failed`** includes `topic`, `payload_bytes`, error — often JSON/shape or DB constraint; cross-check with postgres logs.

**Health endpoints:** `cmd/mqtt-ingest` now exposes `/health/live` and `/health/ready` on `MQTT_INGEST_METRICS_LISTEN` (default `127.0.0.1:9093`). `/metrics` is still conditional on `METRICS_ENABLED=true`.

---

## 6. Temporal workflow operations

**Symptoms:** compensation/review follow-up not happening; repeated scheduling logs with no worker execution; workflows appear stuck after sink or database outages.

**Current topology:** scheduler call sites live in `cmd/api`, `cmd/worker`, and `cmd/reconciler` behind `TEMPORAL_SCHEDULE_*` flags. Execution happens in `cmd/temporal-worker` on `TEMPORAL_TASK_QUEUE`. Activities re-read Postgres state before taking action. External review/refund fan-out uses the existing NATS refund-review sink and is intentionally configured with single-attempt activity dispatch to avoid duplicate publishes on automatic retries.

**Checks:**

1. Confirm `TEMPORAL_ENABLED=true`, `TEMPORAL_HOST_PORT`, `TEMPORAL_NAMESPACE`, and `TEMPORAL_TASK_QUEUE` match between schedulers and `cmd/temporal-worker`.
2. Confirm `cmd/temporal-worker` is healthy on `TEMPORAL_WORKER_METRICS_LISTEN` and can reach both Postgres and NATS.
3. Inspect scheduler logs:
   - `payment_timeout_workflow_scheduled`
   - `refund_review_workflow_scheduled`
   - `vend failure workflow enqueue failed`
   - `duplicate_payment_workflow_schedule_failed`
4. Inspect Temporal UI / CLI for the deterministic workflow IDs:
   - `payment-pending-timeout:<payment_id>`
   - `vend-failure-after-payment:<vend_id>`
   - `refund-orchestration:<payment_id>`
   - `manual-review:<payment_id>`

**Retry posture:**

- Read/state-check activities retry automatically with bounded backoff.
- NATS review/refund dispatch activities use **one attempt** to avoid duplicate ticket publishes on ambiguous network failures.
- A failed dispatch activity leaves the workflow failed for operator inspection/redrive instead of blindly retrying the external publish.

**Stuck workflow handling:**

1. Fix the underlying dependency first (Postgres, NATS, Temporal frontend, refund-review subject).
2. Re-run the failed workflow from Temporal UI/CLI only after confirming the current order/payment state still warrants action.
3. If the original workflow already produced the external side effect and only the ack/result path failed, prefer marking the follow-up handled in the downstream review system rather than replaying immediately.

**Redrive guidance:**

- Safe to re-drive when the workflow failed **before** `EnqueueRefundReview` / manual review dispatch, or when downstream confirms no ticket was created.
- Be careful re-driving after manual review/refund dispatch failures because NATS core publish is not broker-deduplicated.
- Deterministic workflow IDs prevent duplicate scheduling from the app side; use a new workflow ID only for an intentional fresh compensation pass.

---

## 7. Dashboards and alerts (high value)

Use **Grafana + Prometheus** for first-line panels when `METRICS_ENABLED` is on (`ops/grafana/provisioning/dashboards/json/`, `ops/METRICS.md`). Supplement with **JSON logs** (Loki / CloudWatch Logs Insights / etc.) for context and fields not exported as series.

| Signal | Primary source | Alert idea |
| ------ | -------------- | ---------- |
| Active session anomalies | API logs + DB `count(*) where status='ACTIVE'` per machine | Sudden spike in `active_session_exists` or duplicate ACTIVE rows |
| Pending outbox age | `avf_worker_outbox_*` + logs `outbox_oldest_pending_age`, `outbox_pending_total` | `avf_worker_outbox_oldest_pending_age_seconds` > SLO OR `outbox_pipeline_unhealthy` |
| Retry growth | SQL `publish_attempt_count`; logs `publish_attempt_after` | P95 attempts high or `dead_lettered` increasing |
| Publish failure rate | `rate(avf_worker_outbox_dispatch_publish_failed_total[5m])` + `worker_job_summary` | sustained non-zero publish failures |
| Reconciler cycle latency | `avf_reconciler_cycle_duration_seconds` + `background_cycle_end` `duration` | p95 near `cycle_timeout` or `cycle_deadline_exceeded` logs |
| MQTT ingest error rate | `rate(avf_mqtt_ingest_dispatch_total{result="error"}[5m])` + `mqtt ingest failed` logs | threshold + broker disconnect errors |

**Noise control:** correlate `worker_job_summary` `at_batch_limit=true` with `selected` counts before paging.

### Production alert -> first action

| Alert | First operator action |
| ----- | --------------------- |
| `AVFPublicAPIHTTPSDown` | Check app-node Caddy status, recent deploy activity, and LB/DNS/TLS reachability first. |
| `AVFAPIInstanceMetricsDown` | Confirm the node still exposes the private API ops port and that `cmd/api` is running. |
| `AVFAppReadinessFailed` / `AVFDeployUnhealthy` | Stop or pause the rollout; inspect the failing node's service logs before moving traffic. |
| `AVFDatabaseConnectivityLikelyFailed` | Verify Postgres reachability and credentials from the affected node before restarting app processes. |
| `AVFWorkerOutboxLagHigh` / `AVFWorkerPublishFailuresHigh` | Check worker logs, then NATS connectivity/stream health. |
| `AVFTelemetryQueueLagHigh` / `AVFTelemetryProjectionDBFailures` | Check worker telemetry projection logs and JetStream backlog, then Postgres health. |
| `AVFMQTTIngestErrorRateHigh` / `AVFMQTTIngestReadinessFlapping` | Check `cmd/mqtt-ingest` logs first, then EMQX or managed MQTT broker reachability. |
| `AVFNATSDown` | Check the data-node NATS container plus disk pressure on the data node. |
| `AVFEMQXDown` | Check EMQX process health, MQTT TLS cert files, and 8883 reachability. |
| `AVFHostMemoryPressure` / `AVFHostDiskPressure` | Check node-exporter host stats, container growth, and retention/storage pressure before restarting anything. |

---

## 8. Quick reference — log message names

| Message | Meaning |
| ------- | ------- |
| `worker_startup` / `reconciler_startup` / `temporal_worker_bootstrap` | Feature flags, queue wiring, ticks, limits |
| `outbox_pipeline_unhealthy` | Elevated dead-letters or high pending attempts — investigate |
| `outbox publish failed` | Broker or transport error for one row |
| `worker_job_summary` | End of outbox dispatch, payment timeout scan, or stuck command scan tick |
| `reconciler_job_summary` | End of reconciler list pass |
| `background_cycle_end` `result=cycle_deadline_exceeded` | Tick took longer than per-cycle timeout — risk of lag |
| `mqtt ingest failed` | Single message handling failed |

---

## 9. CI regression gates (engineering)

Pull requests run the repository workflows under **`.github/workflows/`** (notably `ci.yml`): `gofmt`, `go vet`, `go test`, **sqlc** drift check, **goose** migration `up` on a clean Postgres, and bash scripts under `scripts/` that fail on nil reconciler adapters, fake empty list handlers, `httptest` in non-test production paths, and missing startup validation hooks (`ValidateRuntimeWiring`, `ValidateReconciler`, etc.). When extending production wiring, update those scripts if the check is a false positive, or fix the regression—do not bypass with broad `rg` ignores unless justified in review.
