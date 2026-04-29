# Load test harness (P1.5 fleet storm)

Optional tooling to simulate **100–1000** vending machines (and adjacent admin / payment / MQTT paths) against **staging, local, or isolated perf** stacks. Normal CI uses `make test-short` only; this harness is **manual / scheduled**.

## Components

| Artifact | Purpose |
| --- | --- |
| `tools/loadtest/cmd/avf-loadtest` | Go CLI: HTTP check-ins, phased gRPC runtime (sync / telemetry / offline), signed webhook bursts, fleet MQTT command→ACK, admin GET storm, Prometheus scrape helper |
| `scripts/loadtest/run_fleet_storm.sh` | Executes `storm` with `-machines N` + `-execute` (`LOADTEST_MACHINE_MANIFEST` required) |
| `scripts/loadtest/run_suite.sh` | Staging orchestrator (`EXECUTE_LOAD_TEST=true` adds `-manifest`) + optional k6 |
| `scripts/loadtest/run_small.sh` | Calls `make loadtest-small` (dry-run) |
| `scripts/loadtest/admin_smoke.k6.js` | k6 admin + org reports |
| `deployments/loadtest/env.example` | Safe env template (copy privately; never commit secrets) |

## Makefile targets

| Target | Behavior |
| --- | --- |
| `make loadtest-small` | **Dry-run only** — prints plan (`scenario=storm`, manifest path, `-machines`, `storm_waves`); no network |
| `make loadtest-build` | Builds `bin/avf-loadtest` |
| `make loadtest-100` | Runs `storm` with **`-machines 100` + `-execute`** via `run_fleet_storm.sh` |
| `make loadtest-500` | Same for **500** machines |
| `make loadtest-1000` | Same for **1000** machines — **use a dedicated staging/perf tier** (not laptop docker-default) |

Pass extra CLI flags through Make:

```bash
make loadtest-100 EXTRA_LOADTEST_FLAGS="-metrics-url http://127.0.0.1:9091/metrics -concurrency 64"
```

## Quick local dry-run (no secrets)

```bash
make loadtest-small
# equivalent:
go run ./tools/loadtest/cmd/avf-loadtest -scenario storm
```

Prints the planned scenario and exits **without traffic**. Safe for CI/docs pipelines.

## Machine manifest (fleet-shaped tests)

TSV (**tab-separated**), one row per machine:

```text
# machine_id<TAB>jwt
11111111-1111-1111-1111-111111111111	eyJ...
```

Or JSON array: `[{"machine_id":"...","jwt":"..."}]` (use `.json` suffix).

JWTs must be **real machine-scoped access tokens** for the target environment—this repository does **not** mint them.

## Scenario: `storm` / `suite`

`suite` and `storm` are **aliases** — both run the **sequential** P1.5 storm (clear per-phase latency lines):

1. **Machine check-in reconnect** — `POST /v1/machines/{id}/check-ins` for the full manifest, repeated **`storm-waves`** times (default **3**), simulating reconnect churn. Env: `AVF_LOADTEST_STORM_WAVES`.
2. **gRPC bootstrap / catalog / media** — `GetBootstrap`, `GetCatalogSnapshot`, `GetMediaManifest`, `GetMediaDelta` (per machine, parallel to `concurrency`).
3. **gRPC telemetry batch** — `PushTelemetryBatch`.
4. **gRPC offline replay** — `GetSyncCursor` + `PushOfflineEvents` (`telemetry.batch` envelope).
5. **MQTT command dispatch + ACK** — `diagnostic.ping` per machine using that machine’s UUID in topics (`MQTT_TOPIC_PREFIX`, `MQTT_TOPIC_LAYOUT`). Skip with `-skip-mqtt` or `LOADTEST_SKIP_MQTT=1`.
6. **Payment webhook burst + duplicate replay** — signed `POST .../webhooks` (`-webhook-duplicate-every`). Skip with `-skip-webhook`, `LOADTEST_SKIP_WEBHOOK=1`, or unset secret.
7. **Admin read pressure** — `RunHTTPAdminSequence` (health, machine list, org reports) for **`duration`** (`AVF_LOADTEST_DURATION`). Requires `ADMIN_JWT` + `LOADTEST_ORGANIZATION_ID`.

Individual scenarios (`checkin`, `grpc`, `mqtt`, `webhook`, `admin`) remain available for focused debugging.

## Metrics emitted / captured

| Output | Source |
| --- | --- |
| Total requests, errors, p50/p**95**/p**99**, throughput (rps) | `LatencyRecorder` per phase (`requests=… rps=…` in stdout) |
| Outbox backlog | `outbox_pending_total` |
| Outbox lag (mean) | `outbox_lag_seconds_sum` / `outbox_lag_seconds_count` when both present |
| Payment webhooks (server) | Sum of `avf_commerce_payment_webhook_requests_total` (all `result` labels) |
| Redis | `avf_redis_rate_limit_hits_total` (rate-limit path; dedicated cache hit/miss metrics are not wired in this repo) |
| MQTT ACK timeouts | `avf_mqtt_command_ack_timeout_total` |
| DB pool pressure | `avf_db_pool_acquired_conns`, `idle`, `total`, `max` |

Client-side **MQTT command ACK latency** and **webhook POST latency** appear in the `mqtt_command_ack` and `webhook_signed_burst` phase lines.

Point `-metrics-url` at the **ops** `/metrics` listener (often `HTTP_OPS_ADDR`); optional `METRICS_SCRAPE_TOKEN`.

**Histogram quantiles** (true server-side p99) need Prometheus `histogram_quantile` / Grafana; the harness scrapes text for counters/gauges and webhook sums.

## Required / common environment variables

| Variable | When |
| --- | --- |
| `EXECUTE_LOAD_TEST=true` or `-execute` | Enable real requests |
| `LOADTEST_MACHINE_MANIFEST` | Path to TSV/JSON for fleet scenarios |
| `LOAD_TEST_ENV` | `staging` / `local` / etc. — see safety below |
| `ADMIN_JWT`, `LOADTEST_ORGANIZATION_ID` | Admin phase |
| `COMMERCE_PAYMENT_WEBHOOK_SECRET`, `LOADTEST_WEBHOOK_ORDER_ID`, `LOADTEST_WEBHOOK_PAYMENT_ID` | Webhook phase |
| `MQTT_BROKER_URL`, `MQTT_TOPIC_PREFIX`, `MQTT_TOPIC_LAYOUT` | MQTT phase (machine UUID comes from the manifest) |
| `AVF_LOADTEST_METRICS_URL`, `METRICS_SCRAPE_TOKEN` | Post-run Prometheus scrape |

See `deployments/loadtest/env.example`.

## Safe limits (avoid harming production)

- **`LOAD_TEST_ENV=production` + `-execute` is refused** — target **staging** or an isolated perf stack.
- Never commit JWTs, webhook secrets, or MQTT passwords; use **short-lived staging** credentials.
- Prefer **`make loadtest-small`** in CI or docs pipelines.
- **1000-machine** runs should use a **dedicated** staging/perf environment; local docker is for small N sanity checks.

## Staging load test procedure

1. Fill a private env file from `deployments/loadtest/env.example` (`set -a; . ./staging-load.env`).
2. Provision manifests with **N** machines (100 / 500 / 1000 rows).
3. Run one of:

```bash
export EXECUTE_LOAD_TEST=true
export LOADTEST_MACHINE_MANIFEST=/secure/path/machines.tsv
bash scripts/loadtest/run_suite.sh

# or direct fleet size:
export LOADTEST_MACHINE_MANIFEST=/secure/path/machines.tsv
make loadtest-500
```

4. Point `AVF_LOADTEST_METRICS_URL` at ops `/metrics` when available.
5. Capture stdout (phase lines + `metrics snapshot:`) and Prometheus/Grafana for governance packs.

## Expected baseline numbers

Baselines **vary** by CPU, Postgres sizing, Redis, network RTT, and feature flags. Order-of-magnitude **sanity** checks—not SLAs:

| Tier | gRPC sync (4 RPCs) p95 | Check-in p95 | Admin sweep p95 |
| --- | --- | --- | --- |
| Local docker-compose | ~80–250 ms | ~40–120 ms | ~50–200 ms |
| Staging (same region) | ~120–400 ms | ~60–180 ms | ~80–350 ms |

Investigate if **error rate >1–2% sustained**, **`outbox_pending_total` grows without bound after load stops**, or **p95 doubles** versus the last comparable run.

## CI

- `go test ./tools/loadtest/...` — parser / stats / manifest tests (no network).
- `python -m unittest discover -s tests -p "*_test.py"` — includes Makefile anchor test for load targets.

## Legacy paths

Older scripts under `scripts/load/` remain; prefer `scripts/loadtest/` for new work.
