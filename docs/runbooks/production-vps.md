# Production VPS topology

This is the canonical production runbook for the current repository snapshot.

The primary production path is the split `2-VPS` layout under `deployments/prod/`:

- `app-node/`: stateless/runtime app-side concerns only
- `data-node/`: optional self-hosted data-plane fallback for `NATS` and `EMQX`
- `shared/`: shared config and release helpers

The old single-host path in `deployments/prod/docker-compose.prod.yml` is legacy, not primary production, and not recommended for the enterprise target deployment.

## Current VPS Snapshot

- VPS-A: `srv1582786.hstgr.cloud` / `72.62.244.94` / `ssh root@72.62.244.94`
- VPS-B: `srv1608106.hstgr.cloud` / `187.127.99.153` / `ssh root@187.127.99.153`
- Location: `Malaysia - Kuala Lumpur` for both VPS
- OS / plan: `Ubuntu 24.04 LTS` on `KVM 2`
- Capacity per VPS: `2 vCPU`, `8 GB RAM`, `100 GB disk`
- There is no dedicated develop VPS yet; the develop environment exists in GitHub workflow structure, but production currently runs on these 2 VPS only

## Roles

### App node

Run `deployments/prod/app-node/docker-compose.app-node.yml` on each production app VPS.

Services:

- `api`
- `worker`
- `reconciler`
- `mqtt-ingest`
- `caddy`
- `temporal-worker` only when explicitly enabled

Responsibilities:

- terminate public HTTP/TLS
- run prebuilt immutable app images from CI
- connect to remote or managed stateful services by env
- avoid hosting Postgres, Redis, object storage, NATS, or EMQX locally

### Data node

Run `deployments/prod/data-node/docker-compose.data-node.yml` only when you still self-host broker services in the interim topology.

Services:

- `nats`
- `emqx`

Responsibilities:

- host JetStream for the current worker and telemetry path when still self-hosted
- host public MQTT over TLS on `8883`
- keep broker admin and monitor ports private

MQTT listener posture for production:

- `8883/tcp` is the external/public production MQTT listener and must use TLS
- `1883/tcp` is private-network-only or loopback-only if retained for compatibility
- plaintext external MQTT is not an acceptable final production posture

## Managed dependency truth

For the current production direction, treat these as the default runtime contract:

- PostgreSQL: managed, reached through `DATABASE_URL`
- Redis: managed when enabled, reached through `REDIS_URL` or `REDIS_ADDR`
- object storage: managed S3-compatible, reached through `OBJECT_STORAGE_ENDPOINT` and bucket env
- NATS / EMQX: either managed endpoints or the optional fallback `data-node`

This differs from local development, where local containers may still be used for convenience. Production scripts in the split topology should not assume a local Postgres or Redis container exists.

The same separation applies to MQTT transport posture: local or private-network exceptions may still use plaintext for narrowly scoped compatibility, but the production public path is TLS-first on `8883`.

## Deployment steps

1. Prepare app-node env on each app VPS from `deployments/prod/app-node/.env.app-node.example`.
2. Keep shared values aligned across app nodes, but make `COMPOSE_PROJECT_NAME`, `MQTT_CLIENT_ID_API`, and `MQTT_CLIENT_ID_INGEST` unique per node.
3. Point `DATABASE_URL`, `NATS_URL`, `MQTT_BROKER_URL`, and optional `REDIS_URL` to managed services or the fallback data node.
4. Prepare `deployments/prod/data-node/.env.data-node` only if you still run the fallback broker plane.
5. Deploy the data node first when app nodes depend on its broker endpoints.
6. Deploy app node A, verify health, then deploy app node B, and verify health again.

The shared deploy entrypoints are:

- `deployments/prod/shared/scripts/release_app_cluster.sh`
- `deployments/prod/shared/scripts/healthcheck_app_node.sh`
- `deployments/prod/shared/scripts/release_data_node.sh`
- `deployments/prod/shared/scripts/healthcheck_data_node.sh`
- `deployments/prod/shared/scripts/check_managed_services.sh`
- `deployments/prod/shared/scripts/backup_managed_postgres.sh`
- `deployments/prod/shared/scripts/restore_managed_postgres.sh`
- `deployments/prod/shared/scripts/verify_runtime_assets.sh`
- `deployments/prod/shared/scripts/check_restore_readiness.sh`
- `deployments/prod/shared/scripts/smoke_http.sh`

The production workflow uses those shared scripts so operators and GitHub Actions follow the same rollout logic.

`check_managed_services.sh` performs client-side reachability checks for managed PostgreSQL, optional managed Redis, and optional S3-compatible object storage using the production env file. It is intentionally honest about scope: it validates network/service reachability from the app node, not provider-side SLOs or snapshot state.

`backup_managed_postgres.sh` and `restore_managed_postgres.sh` are also client-side operations against `DATABASE_URL`. They do not create or manage provider snapshots, point-in-time recovery, or cloud-native backup policies.

Nightly operational assurance is also intentionally non-destructive:

- promotion artifact readiness validates that the latest published build still exposes digest-pinned refs and recorded source input hashes
- runtime asset assurance validates shared production scripts, compose configs, and a checksum inventory for the committed runtime assets
- restore readiness validates tooling and managed backup/restore script contracts without performing a destructive restore

For the optional self-hosted MQTT fallback, `deployments/prod/shared/scripts/bootstrap_prereqs.sh data-node` now enforces that `ca.crt`, `server.crt`, and `server.key` are present when `EMQX_SSL_ENABLED=true`. That keeps the split production path aligned with the TLS-first listener model before rollout begins.

## Validation

From the repository root:

```bash
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml config
docker compose --env-file deployments/prod/data-node/.env.data-node.example -f deployments/prod/data-node/docker-compose.data-node.yml config
```

Run shell validation on changed deploy scripts as part of local verification:

```bash
bash -n deployments/prod/shared/scripts/bootstrap_prereqs.sh
bash -n deployments/prod/shared/scripts/lib_release.sh
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
bash -n deployments/prod/app-node/scripts/release_app_node.sh
bash -n deployments/prod/app-node/scripts/release_app_cluster.sh
bash -n deployments/prod/app-node/scripts/rollback_app_node.sh
bash -n deployments/prod/app-node/scripts/rollback_app_cluster.sh
bash -n deployments/prod/app-node/scripts/healthcheck_app_node.sh
bash -n deployments/prod/data-node/scripts/release_data_node.sh
bash -n deployments/prod/data-node/scripts/rollback_data_node.sh
bash -n deployments/prod/data-node/scripts/healthcheck_data_node.sh
bash -n deployments/prod/data-node/scripts/bootstrap_emqx_data_node.sh
```

## Legacy note

The following legacy single-host assets remain in place only for rollback compatibility:

- `deployments/prod/docker-compose.prod.yml`
- `deployments/prod/.env.production.example`
- `deployments/prod/scripts/*`

Do not treat them as the default production path for new rollouts. Some of those legacy scripts still assume a local single-host container stack; use the shared split-topology scripts above for production operations.
