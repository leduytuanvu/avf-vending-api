# Observability alerts

Canonical on-call content for the production alert catalog in `deployments/prod/observability/prometheus/alerts.yml`.

**Log correlation:** Structured API logs include `trace_id` (OpenTelemetry), `request_id` (`X-Request-ID`), and **`correlation_id`** (W3C trace id when present, else request id). Filter Loki / log storage on `correlation_id` to join errors with the same request chain.

**Prometheus job names (reference):** `avf_api_metrics` (cmd/api + gRPC + MQTT command metrics), `avf_worker_metrics`, `avf_reconciler_metrics`, `avf_mqtt_ingest_metrics`.

**Canonical metric names without `avf_` prefix** (from `internal/platform/observability/productionmetrics`): `http_requests_total`, `http_request_duration_seconds`, `grpc_requests_total`, `grpc_errors_total`, `audit_events_total`, …

## API High 5xx Rate

**Meaning:** Elevated HTTP responses with status 5xx on the API.

**Severity:** P0 (see alert labels).

**Checks:** `sum(rate(http_requests_total{job="avf_api_metrics",status=~"5.."}[5m])) / sum(rate(http_requests_total{job="avf_api_metrics"}[5m]))` — top `route` labels; `http_errors_total`; app logs by `correlation_id`.

**Remediation:** Roll back bad deploy if correlated; check Postgres/Redis readiness; verify recent migrations.

**Escalation:** If customer impact, page platform on-call; include recent deploy id and top routes.

## gRPC High Error Rate

**Meaning:** Non-OK gRPC codes represent a large share of unary completions.

**Checks:** `grpc_errors_total{job="avf_api_metrics"}` by `service`, `method`, `grpc_code`; compare to `grpc_requests_total`. Machine JWT issues: `Unauthenticated` / `PermissionDenied`. Read `grpc_auth_failures_total`.

**Remediation:** Distinguish auth misconfiguration vs handler errors; inspect idempotency ledger and DB connectivity.

**Escalation:** P0 if fleet-wide command or sale path broken.

## gRPC High Latency P99

**Meaning:** Successful (`grpc_code="OK`") unary latency p99 above threshold.

**Checks:** `histogram_quantile(0.99, sum(rate(grpc_request_duration_seconds_bucket{job="avf_api_metrics",grpc_code="OK"}[5m])) by (le))`; split by `method` if needed.

**Remediation:** DB slow queries, hot rows on idempotency ledger, or machine payload size — profile and scale read path.

## gRPC Machine Idempotency Conflict

**Meaning:** `grpc_idempotency_conflicts_total` increased — same idempotency key, different payload hash.

**Remediation:** Client bug, double source, or corrupted retry; capture request payload hashes in logs; halt automated retries for affected machine until root cause.

## Payment Paid But Vend Not Completed

**Meaning:** Commerce reconciler incremented `avf_commerce_payment_paid_vend_failed_total`.

**Checks:** Reconciliation cases UI; `audit_events_total` for related org; payment session state.

**Escalation:** Customer money at risk — prioritize before clearing cases.

## Payment Webhook HMAC Rejections Spike

**Meaning:** `payment_webhook_rejections_total` with HMAC/401-style reasons is high.

**Checks:** Compare to `avf_commerce_payment_webhook_requests_total`; verify signing secret rotation timeline; blackbox / provider IP allowlists; `X-*` header preservation through edge.

**Remediation:** Roll back secret mismatch; confirm provider callback URL and clock sync.

## Command ACK Timeout Spike

**Meaning:** `avf_mqtt_command_ack_timeout_total` rate high on `avf_api_metrics`.

**Checks:** `avf_mqtt_command_dispatch_published_total`, `avf_mqtt_command_ack_deadline_exceeded_total`, EMQX dashboard, device connectivity.

## MQTT Command Dispatch Refused Elevated

**Meaning:** `avf_mqtt_command_dispatch_refused_total` — publish refused before broker (max attempts, validation).

**Remediation:** Inspect command ledger for stuck attempts; broker ACL; rate limits.

## Machine Offline Spike

**Meaning:** Fleet offline share from `avf_machine_connectivity_total` on worker scrape.

**Checks:** EMQX, cellular, OTA rollouts; `telemetry_heartbeats` / heartbeat ingest.

## NATS Outbox Lag High

**Meaning:** Oldest pending outbox row age or publish failures.

**Checks:** `avf_worker_outbox_*` gauges; `avf_worker_outbox_dispatch_publish_failed_total`; NATS JetStream streams.

## Outbox Dead Lettered

**Meaning:** `avf_worker_outbox_dispatch_dead_lettered_total` increased — row quarantined after publish exhaustion.

**Remediation:** Do not blindly replay — fix root publish error first; inspect DLQ companion publish if configured.

## Redis Unavailable In Production

**Meaning:** Blackbox TCP probe to Redis failed.

**Remediation:** Data-node Redis container; network from app nodes; session/revocation degradation.

## PostgreSQL Unavailable

**Meaning:** TCP probe to Postgres failed.

**Remediation:** Managed DB status, credentials, connection limits; treat as P0.

## MQTT Ingest Down

**Meaning:** Metrics scrape or mqtt-ingest readiness failing.

**Remediation:** Broker credentials, TLS certs, process OOM.

## Reconciler Failing

**Meaning:** Non-ok reconciler cycle completions.

**Checks:** By `reconciler_job` label; PSP reachability for payment jobs.

## Refund Pending Too Long

**Meaning:** Refund review path detected long-pending refund.

**Remediation:** Provider portal, manual_case workflow.

## Media Processing Failed Audit Spike

**Meaning:** `audit_events_total{action="media.processing_failed"}` high from API.

**Checks:** Object storage reachability, image decode errors in logs, org asset size limits.

**Remediation:** Fix bad uploads; adjust `maxBytes` / format policy; re-run complete after fix.

## Payment webhook amount / currency mismatch

**Severity:** P0 — see `AVFPaymentWebhookAmountCurrencyMismatch`.

**Meaning:** `payment_webhook_amount_currency_mismatch_total` increased — PSP payload disagrees with trusted `payments` row.

**Queries:** `payment_webhook_amount_currency_mismatch_total`; `payment_webhook_rejections_total`; commerce reconciliation queue `case_type=webhook_amount_currency_mismatch`; logs with `correlation_id` on webhook POST path.

**Mitigation:** Do not force-apply webhook; confirm provider test vs live; verify kiosk-sent amount matches order; resolve case via admin commerce tools after root cause.

**Escalation:** Finance + platform if widespread or provider-side bug.

## Payment provider probe stale backlog

**Severity:** P1 — see `AVFPaymentProviderProbeStaleBacklog`.

**Meaning:** `payment_provider_probe_stale_pending_queue{job="avf_reconciler_metrics"} > 0` for 30m — payments past internal pending-timeout still appear in probe batch.

**Queries:** Reconciler logs `payment_provider_probe`; PSP dashboards; `avf_reconciler_job_failures_total{reconciler_job="payment_provider_probe"}`.

**Mitigation:** Check PSP outage, credentials, reconciler `RECONCILER_ACTIONS_ENABLED`, and network from app to PSP.

**Escalation:** If captures stall fleet-wide, page commerce owner.

## Payment paid vend not started

**Severity:** P0 — see `AVFPaymentPaidVendNotStartedCases`.

**Meaning:** New `avf_commerce_reconciliation_cases_total{case_type="payment_paid_vend_not_started"}` — captured payment without vend start.

**Queries:** Same cases UI; order/payment state; machine gRPC logs.

**Mitigation:** Follow vend recovery runbooks; avoid duplicating capture; coordinate field ops.

## Inventory negative stock attempts spike

**Severity:** P1 — see `AVFInventoryNegativeStockAttemptsSpike`.

**Meaning:** `inventory_negative_stock_attempts_total` rate high — oversell prevention.

**Queries:** Logs around vending/inventory; planogram versions vs machine.

**Mitigation:** Resync catalog/inventory; investigate double vend or bad slot telemetry.

**Escalation:** If fraud or systemic, widen to security review.

## Catalog or media gRPC error burst

**Severity:** P1 — see `AVFCatalogOrMediaGRPCErrorBurst`.

**Meaning:** Non-OK rate on `MachineCatalogService` / `MachineMediaService` RPCs.

**Queries:** `grpc_errors_total` by `service`, `method`, `grpc_code`; Postgres; object store presign errors in logs.

**Mitigation:** See `docs/runbooks/catalog-sync-mismatch.md`.

## Worker outbox pending depth high

**Severity:** P1 — see `AVFWorkerOutboxPendingDepthHigh`.

**Meaning:** `avf_worker_outbox_pending_total` very large — backlog of unpublished rows.

**Queries:** `avf_worker_outbox_oldest_pending_age_seconds`, `avf_worker_outbox_dispatch_publish_failed_total`, NATS JetStream health.

**Mitigation:** Fix broker/auth first; scale worker only after confirming publish path healthy.

## Data plane dependency degraded (composite)

**Severity:** P0 — see `AVFDataPlaneDependencyDegraded`.

**Meaning:** At least one of Postgres / Redis / NATS TCP probes on the data node failed for several minutes.

**Mitigation:** Use per-probe runbooks (PostgreSQL unavailable, Redis unavailable, NATS down) and `production-day-2-incidents.md`.

## EMQX probe flapping

**Severity:** P1 — see `AVFEmqxProbeFlapping`.

**Meaning:** EMQX HTTP status probe unstable — broker reload or host issues.

**Mitigation:** Inspect EMQX logs, cert expiry, memory; correlate with `AVFMachineOfflineSpike` and mqtt-ingest errors.
