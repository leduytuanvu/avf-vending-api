# Production app node

This directory is the primary app-side production runtime for the current `2-VPS` deployment topology.

Run the same compose file on both production app VPSes:

- `api`
- `worker`
- `reconciler`
- `mqtt-ingest`
- `caddy`
- `temporal-worker` only when you enable the `temporal` profile and already use Temporal in this repo

What is intentionally not here:

- PostgreSQL
- Redis
- object storage
- EMQX
- NATS

Those dependencies are expected to be managed services or remote endpoints supplied by env. That keeps the app-node image/runtime contract stable when you later move from self-hosted fallback services to managed equivalents.

## Files

- `docker-compose.app-node.yml`: app-side compose for each VPS
- `.env.app-node.example`: env template for each app VPS
- `../shared/Caddyfile`: shared reverse-proxy config
- `../shared/env-matrix.md`: app-node vs data-node env ownership
- `../shared/network-matrix.md`: edge and internal port exposure matrix
- `../shared/scripts/release_app_cluster.sh`: shared multi-host rollout entrypoint used by operators and CI
- `../shared/scripts/healthcheck_app_node.sh`: shared remote healthcheck wrapper used by operators and CI
- `../shared/scripts/check_managed_services.sh`: managed PostgreSQL / Redis / S3-compatible reachability checks
- `../shared/scripts/backup_managed_postgres.sh`: client-side `pg_dump` helper for managed production Postgres
- `../shared/scripts/restore_managed_postgres.sh`: guarded `pg_restore` helper for managed production Postgres
- `scripts/release_app_node.sh`: node-local rolling release entrypoint
- `scripts/release_app_cluster.sh`: operator-side two-node rollout wrapper over SSH
- `scripts/healthcheck_app_node.sh`: explicit post-deploy readiness/version checks
- `scripts/rollback_app_node.sh`: node-local rollback to previous snapshot or explicit refs
- `scripts/rollback_app_cluster.sh`: operator-side two-node rollback wrapper over SSH

## Deploy shape

1. Copy `.env.app-node.example` to `.env.app-node` on app node A and app node B.
2. Keep shared values the same on both nodes.
3. Change node-specific values that must be unique, especially `COMPOSE_PROJECT_NAME`, `MQTT_CLIENT_ID_API`, and `MQTT_CLIENT_ID_INGEST`.
4. Point `DATABASE_URL`, `REDIS_URL`, `NATS_URL`, and `MQTT_BROKER_URL` to managed services or the fallback data node.
5. Prefer `APP_BASE_URL=https://api.ldtv.dev`, Supabase for `DATABASE_URL`, and `OBJECT_STORAGE_BUCKET=avf-vending-prod-assets`.
6. If you use Temporal-backed flows, set `TEMPORAL_ENABLED=true`, fill the Temporal env, and start with `--profile temporal`.
7. Run schema migrations as a one-shot profile with the existing goose image when needed.

For the split production topology, app-node health checks treat managed PostgreSQL, managed Redis, and managed S3-compatible object storage as the primary dependency model. They do not assume local Postgres or Redis containers exist on the app VPS.

## Edge contract

The app node exposes only HTTP(S):

- `80/tcp` for ACME HTTP challenge handling and optional redirect handling
- `443/tcp` for the public API HTTPS surface on `API_DOMAIN`

Hardening defaults in `../shared/Caddyfile` are intentional:

- Caddy admin API is disabled
- request body size is capped by `CADDY_MAX_REQUEST_BODY`
- upstream proxy headers are overwritten explicitly
- upstream and server timeouts are set explicitly
- security headers are applied on the public API surface

There is no separate public admin HTTPS vhost in this repo today. Keep `HTTP_OPS_ADDR` bound to loopback and keep any broker/admin dashboards private.

## Validation

From this directory:

```bash
docker compose --env-file .env.app-node.example -f docker-compose.app-node.yml config
docker compose --env-file .env.app-node.example -f docker-compose.app-node.yml --profile temporal config
docker compose --env-file .env.app-node.example -f docker-compose.app-node.yml --profile migration config
bash ../shared/scripts/bootstrap_prereqs.sh app-node
bash ../shared/scripts/check_managed_services.sh .env.app-node.example
bash scripts/healthcheck_app_node.sh
```

## Release flow

Local node release from the app VPS:

```bash
bash ../shared/scripts/bootstrap_prereqs.sh app-node
RUN_MIGRATION=1 bash scripts/release_app_node.sh ghcr.io/<owner>/<repo>@sha256:<app-digest> ghcr.io/<owner>/<repo>-goose@sha256:<goose-digest>
```

Two-node rolling release from an operator host:

```bash
export APP_NODE_HOSTS="app-node-a app-node-b"
export PRODUCTION_DEPLOY_ROOT=/opt/avf-vending-api
bash ../shared/scripts/release_app_cluster.sh ghcr.io/<owner>/<repo>@sha256:<app-digest> ghcr.io/<owner>/<repo>-goose@sha256:<goose-digest>
```

Default drain strategy is intentionally simple and deterministic: each node stops `caddy`, updates the local app services while out of rotation, verifies `/health/ready`, then starts `caddy` again before the cluster wrapper proceeds to the next node. This assumes your load balancer stops routing to a node once its edge proxy becomes unhealthy or unavailable.

Health endpoints remain upstream application endpoints reached through Caddy on the API hostname. They exist for load balancer and rollout verification, not as a second admin surface.

## Rollback

Rollback the current node to the previous snapshotted compose/env revision:

```bash
bash scripts/rollback_app_node.sh
```

Rollback both app nodes sequentially:

```bash
export APP_NODE_HOSTS="app-node-a app-node-b"
bash scripts/rollback_app_cluster.sh
```

## Notes

- `deployments/prod/docker-compose.prod.yml` is retained only as a legacy single-host rollback path.
- The legacy single-host path is not the primary production path and is not recommended for the enterprise target deployment.
- This layout reuses the existing application image and does not add a new runtime stack.
- Raw MQTT/TLS is not handled by Caddy. If you self-host MQTT in the fallback topology, expose it separately on the data node at `mqtt.ldtv.dev:8883`.
