# Telemetry production rollout (100–1000 machines)

This runbook complements [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md) with **fleet ramp checks**, **overload detection before outage**, **mitigations**, **rollback**, and **safe tuning**.
It now assumes the split production topology under `deployments/prod/app-node` and `deployments/prod/data-node`; references to the legacy single-host stack are rollback-only.

## Repo static gate (before any fleet-size conversation)

Run **`make verify-enterprise-release`** on the release commit ([production-release-readiness.md](./production-release-readiness.md)): it runs **`go test ./...`**, **Swagger regen + drift check**, **shell syntax** under `scripts/` and `deployments/`, **Compose config** against example env files, **OpenAPI release checks** (production server first, no planned-only Swagger paths, JSON write examples), and **docs/testdata secret heuristics**. **Pilot** deploys do **not** require storm evidence; **scale-100 / scale-500 / scale-1000** do — see **Required storm evidence** in the readiness runbook and **Fleet-scale storm gate** below.

## Before increasing fleet size (100 → 300 → 1000)

1. **Config and compose**
   - Before any production promotion, run the full static gate: `make verify-enterprise-release` (see [production-release-readiness.md](./production-release-readiness.md)).
   - For the current primary production path, you can also run `docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml config` and `docker compose --env-file deployments/prod/data-node/.env.data-node.example -f deployments/prod/data-node/docker-compose.data-node.yml config` when the fallback broker plane is enabled (the verify target runs these for you).
   - Use `bash deployments/prod/scripts/validate_prod_telemetry.sh` and the legacy `deployments/prod/docker-compose.prod.yml` validation flow only if you are intentionally rehearsing or maintaining the rollback-only single-host path.
2. **Images and data plane**
   - On the active 2-VPS path, deploy **immutable** image refs (`APP_IMAGE_REF` / `GOOSE_IMAGE_REF`) and record the previous refs for rollback. The tag-based flow in [prod-ghcr-image-only-deploy.md](./prod-ghcr-image-only-deploy.md) applies only to the rollback-only legacy single-host path.
   - Run migrations through the current app-node rollout policy before or with the telemetry rollout.
3. **NATS JetStream disk**
   - Set explicit **`TELEMETRY_STREAM_MAX_BYTES`** per fleet tier (see capacity table in [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md#jetstream-capacity-by-fleet-tier)); plan **`nats_data`** for roughly **6×** that value at peak plus headroom.
4. **Postgres**
   - Review `DATABASE_MAX_CONNS` and per-process overrides (`API_DATABASE_MAX_CONNS`, `WORKER_DATABASE_MAX_CONNS`, `MQTT_INGEST_DATABASE_MAX_CONNS`, `RECONCILER_DATABASE_MAX_CONNS`, `TEMPORAL_WORKER_DATABASE_MAX_CONNS`). **Estimated total clients** ≈ sum of each process’s effective max pool size × number of app nodes running that stack — stay **below ~60–70%** of the managed pooler session limit before adding a second app node. See [production-2-vps.md](./production-2-vps.md#postgresql-pool-sizing-managed-pooler--supabase).
   - Processes log `postgres_pool_effective` at startup (never `DATABASE_URL` or passwords).
   - Run `bash deployments/prod/shared/scripts/validate_production_deploy_inputs.sh` on `.env.app-node` before enabling `ENABLE_APP_NODE_B` or colocation flags.
   - Keep app-node B disabled until managed Postgres / Supabase pool capacity has been verified for the combined app-node footprint.
5. **EMQX**
   - Confirm connection and auth limits for expected concurrent devices; verify `MQTT_BROKER_URL` and TLS listener capacity if you terminate TLS on EMQX.
6. **Observability**
   - For fleets **≥ ~100 devices**, set `METRICS_ENABLED=true` on **worker** and **mqtt-ingest**.
   - On the 2-VPS path, layer `deployments/prod/docker-compose.observability.yml` with `deployments/prod/app-node/docker-compose.app-node.yml` on the app node that hosts the monitoring stack, or scrape the same private ops ports from your existing Prometheus deployment.
   - Without metrics, overload is harder to see before user-visible lag.
   - Wire Prometheus alert rules from `deployments/prod/observability/prometheus/alerts.yml` and keep on-call runbooks in [production-observability-alerts.md](./production-observability-alerts.md) aligned with those expressions.
   - **Before calling the fleet enterprise-ready or promoting `fleet_scale_target` beyond pilot**, run the live **monitoring readiness** script from a host that can reach ops URLs: `bash deployments/prod/scripts/check_monitoring_readiness.sh` (required env vars and JSON output are documented in the script header and [production-observability-alerts.md](./production-observability-alerts.md#monitoring-readiness-check)). The script asserts real **`avf_*`** telemetry series on mqtt-ingest and worker metrics endpoints (not placeholder names) and probes `/health/ready`; it does **not** treat Postgres pool or container-restart metrics as present until those exporters exist (see the TODO table in the observability runbook).
7. **API / Caddy**
   - HTTP health is orthogonal to telemetry volume but still matters for operator tooling; confirm `READINESS_STRICT` behavior matches your expectations.
8. **Offline replay policy**
   - Confirm device firmware / app rollout uses deterministic initial replay jitter of `0-300` seconds per `machine_id`.
   - Confirm per-machine replay stays within `1-5` events/sec and batch size `20-50`.
   - Confirm critical events are retried until **application-level** acknowledgement per [mqtt-contract.md](../api/mqtt-contract.md#application-level-ack-durable-device-outbox-and-business-durability-p0-clarity) (MQTT `PUBACK` / QoS 1 alone is **not** sufficient for business durability). Stale heartbeat / metric updates may be compacted or dropped.

## How to detect telemetry overload before outage

Watch **both** broker-side backlog and worker-side processing (see metric table in [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md)):

| Signal | Interpretation |
| --- | --- |
| Rising `avf_telemetry_consumer_lag` | JetStream `NumPending` growing — publish or fetch/consume imbalance. |
| High `avf_telemetry_projection_backlog` | Projection semaphore saturated — Postgres or handler work not keeping up. |
| `avf_telemetry_projection_flush_seconds` p99 up | Slow batches — often DB or lock contention. |
| Nonzero `avf_mqtt_ingest_dispatch_total` with error-like labels / log spikes | Ingress-side rejects or publish failures — check mqtt-ingest logs and `TELEMETRY_*` bounds. |
| Worker `GET /health/ready` → **503** (when `TELEMETRY_READINESS_*` > 0) | Controlled overload signal — orchestration can surface this before silent data loss. |
| `avf_telemetry_ingest_rejected_total{reason="critical_missing_idempotency_identity"}` / `avf_telemetry_ingest_critical_missing_identity_total` | Critical telemetry reached mqtt-ingest without `dedupe_key`, `event_id`, or `boot_id`+`seq_no` — fix device envelopes (see `docs/api/mqtt-contract.md`). |
| `avf_telemetry_duplicate_total` (e.g. `edge_event`, `inventory_event`, `critical_metrics_rollup`, `command_receipt`, `idempotency_replay`) | Duplicate replay or redelivery was acknowledged without a second OLTP / rollup mutation — expected during offline replay. |

Also watch **Postgres**: active sessions, slow queries on telemetry paths, and disk IO saturation.

## Offline replay storm prevention

When many machines reconnect after a network outage or site power event, do not allow devices to dump their full offline queues immediately.

Expected device-side behavior:

- deterministic initial replay jitter: `0-300` seconds from stable `machine_id`
- replay budget: `1-5` events/sec per machine
- replay batches: `20-50`
- exponential backoff with jitter on failed batches

Priority rules:

- retry until ack: vend, cash, inventory, command ack / receipt, and other audit-critical events
- compact or drop when stale: heartbeat and high-frequency metrics that have been superseded by newer state

If production symptoms look like a reconnect storm, verify device pacing first before raising broker, worker, or database limits.

## Immediate mitigations (do not disable safety guards)

1. **Reduce burst at the source**: firmware / reporting interval / batching on devices (strongest lever).
2. **Broker vs worker**: If lag rises but projection backlog is low, tune **fetch** behavior cautiously (`TELEMETRY_CONSUMER_BATCH_SIZE`, `TELEMETRY_CONSUMER_PULL_TIMEOUT`) per the resilience runbook — avoid large jumps to `TELEMETRY_CONSUMER_MAX_ACK_PENDING` (memory and redelivery exposure).
3. **Postgres-bound**: Fix queries and indexes first; **do not** raise `TELEMETRY_PROJECTION_MAX_CONCURRENCY` blindly if the database is already saturated.
4. **Second worker instance**: Possible with the same durable consumers — requires operational care (splitting consumers, avoiding duplicate projection assumptions). Prefer vertical headroom and tuning first on the single-VPS profile.

**Never** use `TELEMETRY_LEGACY_POSTGRES_INGEST=true` as a “quick fix” in production — it is forbidden by app config and removes JetStream-backed safety.

## Rollback

1. **Revert image refs or tags** in the active app-node env files to the last known-good values, then rerun the shared split-topology rollback or release flow as appropriate.
2. **Restore env files from backup** if telemetry-related variables changed incorrectly.
3. JetStream stream/consumer shapes are re-applied from env on process start — rollback is primarily **image + env**, not manual NATS CLI surgery, unless you introduced out-of-band broker changes.

## Tuning limits without disabling protections

- **Increase** stream max age/bytes only when you understand disk retention and compliance implications.
- **Readiness thresholds** (`TELEMETRY_READINESS_MAX_PENDING`, `TELEMETRY_READINESS_MAX_PROJECTION_FAIL_STREAK`): tightening fails readiness earlier (good for staging); loosening in prod should follow evidence from metrics, not to silence alerts.
- **Ingress** (`TELEMETRY_PER_MACHINE_*`, `TELEMETRY_GLOBAL_MAX_INFLIGHT`): lowering reduces burst tolerance but protects downstream; raising increases risk — pair with metrics review.

See [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md) for first actions when lag rises.

## Bursty load validation (lab / maintenance window)

Use the automated generator (defaults to **dry-run**; no credentials required to validate the plan):

- `deployments/prod/scripts/telemetry_storm_load_test.sh`
- `deployments/prod/scripts/telemetry_load_smoke.sh` (wrapper; calls the same script)

### Dry-run (print scrape commands + sample payload)

```bash
cd /path/to/avf-vending-api
DRY_RUN=true SCENARIO_PRESET=1000x500 \
  bash deployments/prod/scripts/telemetry_storm_load_test.sh
```

### Staging execute (example)

From a host with `mosquitto_pub`, `python3`, and registered machine UUIDs (ingest calls `GetMachineOrgSite` — synthetic IDs fail unless seeded):

```bash
export EXECUTE_LOAD_TEST=true DRY_RUN=false LOAD_TEST_ENV=staging
export SCENARIO_PRESET=100x100
export MQTT_BROKER_URL='mqtt://device:YOUR_PASS@staging-emqx:1883'
export MQTT_TOPIC_PREFIX='avf/v1'
export MACHINE_IDS_FILE='./staging-machine-ids.txt'
export COMPOSE_FILE='deployments/prod/app-node/docker-compose.app-node.yml'
export ENV_FILE='deployments/prod/app-node/.env.app-node'
export API_READY_URL='https://staging-api.example.com/health/ready'
bash deployments/prod/scripts/telemetry_storm_load_test.sh
```

### Staging storm evidence suite (scale 100 → 500 → 1000)

For **fleet-scale rollout evidence**, run the three certification scenarios **in order** (100×100, 500×200, 1000×500). The wrapper **stops on the first failure** and writes artifacts under **`STORM_SUITE_ARTIFACT_DIR`** (default: `<repo>/telemetry-storm-suite-artifacts/<UTC timestamp>`).

**Plan-only (no broker credentials; validates wiring + writes JSON shells):**

```bash
cd /path/to/avf-vending-api
# Optional: STORM_SUITE_ARTIFACT_DIR="$PWD/my-storm-artifacts"
LOAD_TEST_ENV=staging bash deployments/prod/scripts/run_staging_telemetry_storm_suite.sh --plan-only
```

**Full staging execute (operator maintenance window; requires `python3`, `mosquitto_pub`, scrapable metrics, real machine IDs):**

```bash
cd /path/to/avf-vending-api
export LOAD_TEST_ENV=staging
export EXECUTE_LOAD_TEST=true
export DRY_RUN=false
export MACHINE_IDS_FILE="./staging-machine-ids.txt"
export MQTT_BROKER_URL='mqtts://staging-mqtt.example.com:8883'
export MQTT_USERNAME='staging-device'
export MQTT_PASSWORD='YOUR_STAGING_SECRET'
export MQTT_TOPIC_PREFIX='avf/v1'
export API_READY_URL='https://staging-api.example.com/health/ready'
# Optional: export WORKER_READY_URL='http://127.0.0.1:9091/health/ready'
# Metrics: either both URLs, or a common host + ports (defaults 9093 / 9091):
export METRICS_BASE_URL='http://127.0.0.1'
# export MQTT_INGEST_METRICS_URL='http://127.0.0.1:9093/metrics'
# export WORKER_METRICS_URL='http://127.0.0.1:9091/metrics'
# Same compose scrape defaults as single-scenario storm, if you scrape via docker:
# export COMPOSE_FILE='deployments/prod/app-node/docker-compose.app-node.yml'
# export ENV_FILE='deployments/prod/app-node/.env.staging'
# If this host only has HTTP metrics URLs (no local `docker compose` for the stack):
# export SKIP_DOCKER_GATES=true

bash deployments/prod/scripts/run_staging_telemetry_storm_suite.sh
```

**Safety:** the suite **refuses** `LOAD_TEST_ENV=production`. It does **not** print MQTT passwords or broker userinfo. URLs are checked for **production-shaped hosts** (e.g. documented prod API host patterns); override only with `STORM_ALLOW_PRODUCTION_SHAPED_URL=true` in exceptional lab setups. **`STORM_SCENARIO_MODE`** selects `all` (default) or a single preset (`100x100`, `500x200`, `1000x500`). Per-scenario results: `telemetry-storm-result-100x100.json`, `telemetry-storm-result-500x200.json`, `telemetry-storm-result-1000x500.json`. Summary: **`telemetry-storm-suite-result.json`** (`completed_at_utc`, per-scenario `final_result`, `critical_lost`, `duplicate_critical_effects`, `db_pool_result`, `health_result`, `restart_result`, `final_suite_pass`, `failed_at_scenario`).

### GitHub Actions manual staging storm

Use the **workflow_dispatch** workflow [`.github/workflows/telemetry-storm-staging.yml`](../../.github/workflows/telemetry-storm-staging.yml) for a **repeatable** staging run from GitHub (Ubuntu runner). It is **manual only** and hardcodes **`LOAD_TEST_ENV=staging`**; the suite script **refuses production** and **production-shaped URLs** unless you set an explicit escape hatch in the script env (not set by the workflow).

1. Create a GitHub **Environment** named **`staging`** (or map the job to your org’s staging environment) and add secrets (repository or environment scope):
   - `STAGING_MQTT_BROKER_URL`, `STAGING_MQTT_USERNAME`, `STAGING_MQTT_PASSWORD`
   - `STAGING_API_READY_URL` (e.g. `https://…/health/ready` on **staging**, not production)
   - `STAGING_METRICS_BASE_URL` (scheme + host, no path; workflow inputs **`metrics_mqtt_ingest_port`** / **`metrics_worker_port`** default to **9093** / **9091**). The metrics host must be **reachable from the Actions runner** (GitHub-hosted runners need a public or allowlisted endpoint, or run the workflow on a **self-hosted** runner in your VPC). The workflow sets **`SKIP_DOCKER_GATES=true`** because the hosted runner has no local `docker compose` for staging.
   - Optional: `STAGING_WORKER_READY_URL`
2. Commit a **newline-separated machine UUID** file in-repo (UUIDs only; no secrets), e.g. `.github/workflows/data/staging-storm-machine-ids.txt`, and pass its **repo-relative path** as workflow input **`machine_ids_repo_path`** (no `..`, no absolute path).
3. Inputs: **`scenario_mode`** (`all` | `100x100` | `500x200` | `1000x500`), **`event_rate_per_machine`**, **`critical_event_ratio`**.

**Artifacts:**

| Artifact name | Contents |
| ------------- | -------- |
| `telemetry-storm-<scenario_mode>-<run_id>-<attempt>` | All `telemetry-storm-result*.json`, `telemetry-storm-suite-result.json`, redaction-safe `storm-suite.log` |
| `telemetry-storm-result` | Single `telemetry-storm-result.json` for **production scale gate** consumption (see below) |

On **success**, the workflow uploads a second artifact named exactly **`telemetry-storm-result`** (Deploy Production expects this name when using **`telemetry_storm_evidence_artifact_run_id`**). The JSON inside is:

- **`scenario_mode=all`:** copy of **`telemetry-storm-result-1000x500.json`** (use when **`fleet_scale_target=scale-1000`**).
- **Single preset:** copy of that preset’s result file (use when **`fleet_scale_target`** matches: `scale-100` ↔ 100×100, `scale-500` ↔ 500×200, `scale-1000` ↔ 1000×500).

If you run **`scenario_mode=all`** but only need evidence for **scale-100** or **scale-500**, either re-run with the matching single scenario or point **`telemetry_storm_evidence_repo_path`** at the corresponding `telemetry-storm-result-*.json` from the full-suite artifact zip (the production gate enforces **minimum** `machine_count` / `events_per_machine` vs `fleet_scale_target`, not an exact label match).

The job **summary** tab lists each scenario’s **`final_result`**, **`critical_lost`**, **`duplicate_critical_effects`**, **`db_pool_result`**, **`health_result`**, and **`restart_result`**. The workflow **fails** if the suite fails (no `telemetry-storm-result` gate bundle upload).

### Production (maintenance window only)

```bash
export CONFIRM_PROD_LOAD_TEST=true LOAD_TEST_ENV=production
# ... same MQTT and MACHINE_IDS_FILE as production devices ...
bash deployments/prod/scripts/telemetry_storm_load_test.sh
```

Preset scenarios: `SCENARIO_PRESET=100x100`, `500x200`, or `1000x500` (sets `MACHINE_COUNT` and `EVENTS_PER_MACHINE`). Tune `EVENT_RATE_PER_MACHINE`, `CRITICAL_EVENT_RATIO`, `WAVE_PARALLEL_LIMIT`, and gate env vars documented in the script header.

**Certification gate (execute runs):** With default `STRICT_ACCOUNTING=true`, the script **does not** claim an overall PASS unless it can scrape **before and after** Prometheus text from **mqtt-ingest** and **worker** (`METRICS_ENABLED=true` on both, or `MQTT_INGEST_METRICS_URL` and `WORKER_METRICS_URL`). It compares planned critical publishes to `avf_telemetry_ingest_received_total{channel="critical_no_drop"}` delta, MQTT dispatch OK count to total publishes, and requires **`avf_telemetry_idempotency_conflict_total` delta = 0** (duplicate identical replays must not produce conflicting business effects). Set `STRICT_ACCOUNTING=false` only for ad-hoc traffic generation when you accept unverified results.

**Duplicate replay drill:** `CRITICAL_DUPLICATE_REPLAY_PERCENT` (0–99) triggers a second **identical** publish for a deterministic subset of critical events; the worker conflict counter must stay zero.

**Artifact:** `RESULT_JSON_FILE` (default `telemetry-storm-result.json`) records scenario, counts, `critical_expected`, `critical_accepted` (ingest delta), `critical_lost`, `duplicate_detected` (true when idempotency conflicts > 0), `duplicate_critical_effects`, `max_lag`, `health_result`, `worker_health_result`, `db_pool_result`, `restart_result`, `final_result`, `completed_at_utc` / `finished_at_utc`, and `strict_accounting_failed_reason`. **Dry-run** still writes JSON with `overall_pass=false` / `final_result=fail` (no execute certification). Production and staging defaults require no MQTT credentials for dry-run.

<a id="fleet-scale-storm-gate"></a>

### Production CI fleet-scale gate (`deploy-prod.yml`)

GitHub **Deploy Production** adds `fleet_scale_target` (`pilot` default, or `scale-100` / `scale-500` / `scale-1000`). Above **pilot**, the workflow requires storm evidence before the deploy job runs:

- Supply **`telemetry_storm_evidence_repo_path`** (path in the repo checkout to `telemetry-storm-result.json`), **or**
- **`telemetry_storm_evidence_artifact_run_id`** of a completed workflow run that published an artifact named **`telemetry-storm-result`** (zip must contain `telemetry-storm-result.json`). A typical source is a successful run of **[`telemetry-storm-staging.yml`](../../.github/workflows/telemetry-storm-staging.yml)** (manual staging storm suite).

#### Minimum load vs target (strict)

Evidence must meet **at least** these dimensions (stronger runs, e.g. more machines or higher `events_per_machine`, also qualify):

| `fleet_scale_target` | Minimum `machine_count` | Minimum `events_per_machine` |
| -------------------- | ----------------------- | ----------------------------- |
| `pilot` | _(no storm evidence)_ | |
| `scale-100` | 100 | 100 |
| `scale-500` | 500 | 200 |
| `scale-1000` | 1000 | 500 |

So **`scale-1000` cannot pass** with only **100×100** or **500×200** evidence, and **stale** or **incomplete** JSON fails the gate before deploy.

#### Required evidence fields (non-bypass)

Validation is implemented in `deployments/prod/shared/scripts/validate_production_scale_storm_evidence.py`. For scale targets (when not bypassed), the JSON must include:

- **`completed_at_utc`** — non-empty ISO-8601 UTC timestamp (age checked against **`storm_evidence_max_age_days`**, default **7**; maps to env **`STORM_EVIDENCE_MAX_AGE_DAYS`** for local runs). Missing or unparseable timestamps **fail**. `finished_at_utc` alone is **not** accepted.
- **`scenario`** — non-empty string (e.g. preset label or `MACHINESxEVENTS`).
- **`machine_count`**, **`events_per_machine`** — integers meeting the table above.
- **`execute_load_test=true`**, **`dry_run=false`**
- **`final_result`** — exactly **`pass`** (legacy `overall_pass`-only shortcuts are not accepted).
- **`critical_lost`** — **0**
- **`duplicate_critical_effects`** — field **must be present** and **0**
- **`db_pool_result`**, **`health_result`**, **`restart_result`** — each **`pass`** (not merely `ok`)

The **`production-deployment-manifest`** artifact includes a snapshot object **`storm_evidence`** with these fields when the gate **passes**; it is **`null`** for pilot, rollback, or bypass.

**Bypass:** set **`allow_scale_gate_bypass`** (`ALLOW_SCALE_GATE_BYPASS=true` in env) and a non-empty **`scale_gate_bypass_reason`** (or **`BYPASS_REASON`**). The job summary shows an explicit **operator bypass** section, `::warning::` is emitted for non-pilot targets, and the manifest records **`storm_gate_bypassed=true`** and the reason. **Security, provenance, and health/smoke gates are unchanged.** Rollback mode does not apply the storm gate.

**Note:** The runnable workflow file is **`deploy-prod.yml`**. The repo also contains **`deploy-production.yml`** as a pointer-only workflow (no deploy); do not use it for rollouts.

Goal: simulate reconnect / replay storms under your MQTT auth and topic rules, then observe mqtt-ingest + worker metrics, JetStream consumer lag, API readiness, and Postgres pressure as printed by the script.
