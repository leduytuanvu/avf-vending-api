# Telemetry production rollout (100–1000 machines)

This runbook complements [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md) with **fleet ramp checks**, **overload detection before outage**, **mitigations**, **rollback**, and **safe tuning**.
It now assumes the split production topology under `deployments/prod/app-node` and `deployments/prod/data-node`; references to the legacy single-host stack are rollback-only.

## Before increasing fleet size (100 → 300 → 1000)

1. **Config and compose**
   - For the current primary production path, run `docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml config` and `docker compose --env-file deployments/prod/data-node/.env.data-node.example -f deployments/prod/data-node/docker-compose.data-node.yml config` when the fallback broker plane is enabled.
   - Use `bash deployments/prod/scripts/validate_prod_telemetry.sh` and the legacy `deployments/prod/docker-compose.prod.yml` validation flow only if you are intentionally rehearsing or maintaining the rollback-only single-host path.
2. **Images and data plane**
   - On the active 2-VPS path, deploy **immutable** image refs (`APP_IMAGE_REF` / `GOOSE_IMAGE_REF`) and record the previous refs for rollback. The tag-based flow in [prod-ghcr-image-only-deploy.md](./prod-ghcr-image-only-deploy.md) applies only to the rollback-only legacy single-host path.
   - Run migrations through the current app-node rollout policy before or with the telemetry rollout.
3. **NATS JetStream disk**
   - Ensure the `nats_data` volume has headroom for `TELEMETRY_STREAM_MAX_BYTES` and burst retention; monitor NATS container logs and host disk.
4. **Postgres**
   - Review `DATABASE_MAX_CONNS` vs API + worker + mqtt-ingest pools; increasing fleet size raises sustained write load on telemetry projection.
5. **EMQX**
   - Confirm connection and auth limits for expected concurrent devices; verify `MQTT_BROKER_URL` and TLS listener capacity if you terminate TLS on EMQX.
6. **Observability**
   - For fleets **≥ ~100 devices**, set `METRICS_ENABLED=true` on **worker** and **mqtt-ingest**.
   - On the 2-VPS path, layer `deployments/prod/docker-compose.observability.yml` with `deployments/prod/app-node/docker-compose.app-node.yml` on the app node that hosts the monitoring stack, or scrape the same private ops ports from your existing Prometheus deployment.
   - Without metrics, overload is harder to see before user-visible lag.
7. **API / Caddy**
   - HTTP health is orthogonal to telemetry volume but still matters for operator tooling; confirm `READINESS_STRICT` behavior matches your expectations.

## How to detect telemetry overload before outage

Watch **both** broker-side backlog and worker-side processing (see metric table in [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md)):

| Signal | Interpretation |
| --- | --- |
| Rising `avf_telemetry_consumer_lag` | JetStream `NumPending` growing — publish or fetch/consume imbalance. |
| High `avf_telemetry_projection_backlog` | Projection semaphore saturated — Postgres or handler work not keeping up. |
| `avf_telemetry_projection_flush_seconds` p99 up | Slow batches — often DB or lock contention. |
| Nonzero `avf_mqtt_ingest_dispatch_total` with error-like labels / log spikes | Ingress-side rejects or publish failures — check mqtt-ingest logs and `TELEMETRY_*` bounds. |
| Worker `GET /health/ready` → **503** (when `TELEMETRY_READINESS_*` > 0) | Controlled overload signal — orchestration can surface this before silent data loss. |

Also watch **Postgres**: active sessions, slow queries on telemetry paths, and disk IO saturation.

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

Use the scripted checklist and host-side `curl` loops in:

- `deployments/prod/scripts/telemetry_load_smoke.sh`

Goal: simulate bursty device telemetry (within your MQTT auth and topic rules), then observe queue depth, rate-limited/dropped behavior, consumer lag, and DB pressure indicators as described in that script.
