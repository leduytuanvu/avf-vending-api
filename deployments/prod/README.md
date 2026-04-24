# Production Deployment Layout

The primary production path for this repository is the split `2-VPS` layout under `deployments/prod/`:

- `app-node/`: stateless app runtime for each production app VPS
- `data-node/`: optional self-hosted data-plane fallback for `NATS` and `EMQX`
- `shared/`: shared proxy config, env ownership, and release helpers

This is the current recommended production topology for this snapshot. It replaces the old "one VPS runs everything" layout as the primary path.

## Start Here

- `app-node/README.md`
- `data-node/README.md`
- `shared/env-matrix.md`
- `shared/network-matrix.md`
- `../../docs/runbooks/production-vps.md`
- `../../docs/runbooks/production-cutover-rollback.md`
- `../../docs/runbooks/production-backup-restore-dr.md`
- `../../docs/runbooks/production-day-2-incidents.md`

## Responsibility Split

### App node

Run `app-node/docker-compose.app-node.yml` on each app VPS.

Contains only app-side runtime concerns:

- `api`
- `worker`
- `reconciler`
- `mqtt-ingest`
- `caddy`
- `temporal-worker` only when explicitly enabled

Does not host Postgres, Redis, object storage, NATS, or EMQX.

Production truth for app dependencies:

- PostgreSQL: managed, reached through `DATABASE_URL`
- Redis: managed when enabled, reached through `REDIS_URL` or `REDIS_ADDR`
- object storage: managed S3-compatible, reached through `OBJECT_STORAGE_ENDPOINT` and bucket env
- NATS / EMQX: either managed endpoints or the optional `data-node` fallback

### Data node

Run `data-node/docker-compose.data-node.yml` only when you still self-host broker services in the interim topology.

Contains only fallback data-plane services:

- `nats`
- `emqx`

Does not host app processes, Postgres, Redis, or object storage.

### Shared

Shared assets live under `shared/`:

- `Caddyfile`
- `env-matrix.md`
- `network-matrix.md`
- `scripts/bootstrap_prereqs.sh`
- `scripts/lib_release.sh`
- `scripts/release_app_cluster.sh`
- `scripts/healthcheck_app_node.sh`
- `scripts/release_data_node.sh`
- `scripts/healthcheck_data_node.sh`
- `scripts/check_managed_services.sh`
- `scripts/backup_managed_postgres.sh`
- `scripts/restore_managed_postgres.sh`
- `scripts/verify_runtime_assets.sh`
- `scripts/check_restore_readiness.sh`
- `scripts/smoke_http.sh`

## Legacy Single-Host Path

The old single-host production assets are retained only for rollback and operator reference:

- `docker-compose.prod.yml`
- `.env.production.example`
- `scripts/deploy_prod.sh`
- `scripts/update_prod.sh`
- `scripts/rollback_prod.sh`
- `scripts/healthcheck_prod.sh`
- `scripts/release.sh`

These files are:

- legacy
- not the primary production path
- not recommended for the enterprise target deployment

That legacy path still contains container-centric assumptions such as local Postgres health/restore flows. Do not use those scripts for the split production topology.

See `legacy/README.md` for the intended status of those assets.

## Validation

From the repository root:

```bash
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml config
docker compose --env-file deployments/prod/data-node/.env.data-node.example -f deployments/prod/data-node/docker-compose.data-node.yml config
bash -n deployments/prod/shared/scripts/check_managed_services.sh
bash -n deployments/prod/shared/scripts/backup_managed_postgres.sh
bash -n deployments/prod/shared/scripts/restore_managed_postgres.sh
bash -n deployments/prod/shared/scripts/verify_runtime_assets.sh
bash -n deployments/prod/shared/scripts/check_restore_readiness.sh
bash -n deployments/prod/shared/scripts/smoke_http.sh
bash -n deployments/prod/shared/scripts/release_app_cluster.sh
bash -n deployments/prod/shared/scripts/healthcheck_app_node.sh
bash -n deployments/prod/shared/scripts/release_data_node.sh
bash -n deployments/prod/shared/scripts/healthcheck_data_node.sh
bash -n deployments/prod/scripts/telemetry_storm_load_test.sh
bash -n deployments/prod/scripts/telemetry_load_smoke.sh
```

## Telemetry storm load test

Automated MQTT publish load for offline-replay / fleet storm validation (staging by default; production requires explicit confirmation):

- `deployments/prod/scripts/telemetry_storm_load_test.sh`
- Wrapper (same script): `deployments/prod/scripts/telemetry_load_smoke.sh`

Dry-run (no broker credentials):

```bash
DRY_RUN=true SCENARIO_PRESET=100x100 bash deployments/prod/scripts/telemetry_storm_load_test.sh
```

## Notes

- App nodes expose only `80/443` for the public HTTP surface.
- Data nodes expose raw MQTT/TLS on `8883` as the public production MQTT path when the fallback broker is enabled.
- MQTT is not proxied through the HTTP reverse proxy.
- Plain MQTT on `1883` is private-network-only or loopback-only if kept for compatibility; it is not an acceptable public production listener.
- NATS, metrics, ops, and broker admin ports stay private.
- App-node health checks now include managed PostgreSQL, optional managed Redis, and optional S3-compatible object storage reachability checks using environment-provided connection settings.
- Managed Postgres backup and restore flows are client-side `pg_dump` / `pg_restore` operations against `DATABASE_URL`; they are not provider snapshot orchestration.
- Nightly ops performs non-destructive promotion-artifact, runtime-asset, and restore-readiness checks using the shared production helper scripts.
- The production GitHub Actions workflow now rolls app node A, verifies health, then rolls app node B. Data-node rollout remains a separate explicit step.
- CI and Security workflows must remain green before merge.
