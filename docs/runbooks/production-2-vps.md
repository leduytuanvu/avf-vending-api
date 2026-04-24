# Production: 2-VPS deployment topology

This runbook describes the current primary production layout.
The legacy single-VPS path in `deployments/prod/docker-compose.prod.yml` remains only for rollback compatibility and is not the default production deployment path.

Current production snapshot for this repo:

- API domain: `api.ldtv.dev`
- MQTT domain: `mqtt.ldtv.dev`
- app node deploy root: `/opt/avf-vending-api`
- PostgreSQL provider: Supabase (`DATABASE_URL`)
- object storage bucket: `avf-vending-prod-assets`
- Grafana is intentionally not part of this pass
- VPS-A: `srv1582786.hstgr.cloud` / `72.62.244.94` / `ssh root@72.62.244.94`
- VPS-B: `srv1608106.hstgr.cloud` / `187.127.99.153` / `ssh root@187.127.99.153`
- Both current production VPS are in `Malaysia - Kuala Lumpur`
- Both current production VPS run `Ubuntu 24.04 LTS` on the `KVM 2` plan (`2 vCPU`, `8 GB RAM`, `100 GB disk`)
- There is no dedicated develop VPS yet; the develop environment exists in GitHub workflow structure, but production currently uses only these 2 VPS

## Roles

### App node

Run `deployments/prod/app-node/docker-compose.app-node.yml` on each production app VPS.

Services:

- `caddy`
- `api`
- `worker`
- `reconciler`
- `mqtt-ingest`
- `temporal-worker` only when the Temporal profile is enabled

Responsibilities:

- terminate public HTTP/TLS
- run the existing Go binaries already built by CI
- connect to managed or remote stateful services by env only
- do not terminate MQTT/TCP

### API documentation (Swagger UI)

Production env templates set `HTTP_SWAGGER_UI_ENABLED=true`. Public URLs on the API host:

- Swagger UI: `https://api.ldtv.dev/swagger/index.html`
- Raw OpenAPI JSON: `https://api.ldtv.dev/swagger/doc.json`

These endpoints are **documentation only**. **`/v1/*`** routes still require a valid **`Authorization: Bearer <JWT>`** (and remain subject to the same auth middleware as always).

### Data node

Run `deployments/prod/data-node/docker-compose.data-node.yml` only when you still need a self-hosted fallback plane.

Services:

- `nats`
- `emqx`

Responsibilities:

- host JetStream for the current worker/telemetry path
- host MQTT only as a fallback until a managed broker is adopted
- terminate raw MQTT/TLS directly on `8883` when the fallback broker is enabled

## Preferred target state

Use managed services where possible:

- PostgreSQL: managed
- Redis: managed when required by enabled features
- object storage: managed S3-compatible
- MQTT: managed later, but keep `MQTT_BROKER_URL` stable so the app stack does not change
- NATS: self-host fallback today, replaceable later by changing `NATS_URL`

## Rollout notes

1. Keep `deployments/prod/docker-compose.prod.yml` untouched so single-VPS rollback stays available.
2. Copy `app-node/.env.app-node.example` to `.env.app-node` on both app VPSes.
3. Change per-node values on each app VPS, especially `COMPOSE_PROJECT_NAME`, `MQTT_CLIENT_ID_API`, and `MQTT_CLIENT_ID_INGEST`.
4. Copy `data-node/.env.data-node.example` to `.env.data-node` only if you are running the fallback broker plane.
5. Point app nodes at managed endpoints first; use the data node only for the broker endpoints you still self-host.
6. If you already use Temporal-backed workflows, enable the `temporal` profile on app nodes and set the `TEMPORAL_*` env.
7. Keep app-node B disabled unless you have verified managed Postgres / Supabase pool capacity for the combined `api`, `worker`, `reconciler`, `mqtt-ingest`, and optional `temporal-worker` pools across both app nodes.

## GitHub production deploy: fleet scale storm gate

The production workflow (`.github/workflows/deploy-prod.yml`, **Deploy Production** in the Actions UI) defaults to `fleet_scale_target=pilot` and does **not** require telemetry storm evidence. For **scale-100**, **scale-500**, or **scale-1000**, the workflow requires recent `telemetry-storm-result.json` evidence (repo path or an Actions artifact named `telemetry-storm-result`) validated by `deployments/prod/shared/scripts/validate_production_scale_storm_evidence.py`, unless `allow_scale_gate_bypass` is set with a non-empty `scale_gate_bypass_reason`. Rollback mode skips this gate. See [telemetry-production-rollout.md](./telemetry-production-rollout.md) for storm runs and evidence shape.

## Last-known-good (LKG) and rollback readiness

Every **Deploy Production** run uploads a **`production-deployment-manifest`** artifact (JSON). After a **successful** deploy, that manifest records the digest-pinned **`app_image_ref`** and **`goose_image_ref`** that are now live, plus **`source_commit_sha`**, **`release_tag`**, **`deployed_at_utc`**, **`run_id`**, and **`run_url`**.

Before the next deploy, the workflow **Resolve Previous Production Deployment** job scans recent successful runs for that artifact, validates digest pinning, and sets **`rollback_available`**. The job summary includes copy-paste values for **manual** `workflow_dispatch` rollback. If no manifest is found (for example expired artifact), automatic rollback after a mid-rollout failure is **blocked** until you restore refs from an external archive or complete a clean deploy.

- **Automatic rollback** (deploy mode only): repins **app + goose** images on affected app nodes via `rollback_app_node.sh` with **`RUN_MIGRATION=0`** — **no migration down**.
- Full operator steps: [production-rollback.md](./production-rollback.md).

## Enterprise release evidence pack

To bundle **last-known-good**, digest-pinned refs, static verify, monitoring readiness, and storm results for a promotion review, use **`deployments/prod/scripts/build_release_evidence_pack.sh`** with the deployment manifest artifact and other JSON inputs. See [production-release-readiness.md — Enterprise release evidence pack](./production-release-readiness.md#enterprise-release-evidence-pack).

## PostgreSQL pool sizing (managed pooler / Supabase)

**Estimated client connections from this stack**

`total_connections ≈` (sum over each process binary of **effective max pool size** for that binary) × (**number of app nodes** that run a full copy of that stack).

- **effective max** for a process is the matching `*_DATABASE_MAX_CONNS` when set (`API_DATABASE_MAX_CONNS`, `WORKER_DATABASE_MAX_CONNS`, …); otherwise `DATABASE_MAX_CONNS`.
- Each **app node** runs one replica of `api`, `worker`, `reconciler`, `mqtt-ingest`, and optionally `temporal-worker` when the Temporal profile is enabled — count each process once per node.
- **Target:** keep `total_connections` **below roughly 60–70%** of your provider’s session-mode pool limit (for example Supabase `MaxClientsInSessionMode`) **before** enabling a second app node or colocating the app stack on the data-node host.

On startup every Go binary logs effective pool settings at **info** (`postgres_pool_effective` via `log/slog`) with `process`, `max_conns`, `min_conns`, and idle/lifetime — **never** the database URL or password.

Operational checks:

- Seed env from `deployments/prod/app-node/.env.app-node.example` (safe defaults) or run `deployments/prod/shared/scripts/render_rollout_env.sh`.
- Run `deployments/prod/shared/scripts/validate_production_deploy_inputs.sh` with your real `.env.app-node` before rollout; it warns on `ENABLE_APP_NODE_B` and enforces explicit acknowledgement when `COLOCATE_APP_WITH_DATA_NODE=true`.

## Edge and exposure model

API HTTPS:

- public DNS: `api.ldtv.dev`
- listener: `caddy` on each app node
- public ports: `80` and `443`
- health endpoints stay on the API HTTP surface and are intended for load balancer and rollout checks

Admin HTTPS:

- there is no separate public admin vhost in this backend repo by default
- keep API ops listeners on `HTTP_OPS_ADDR` loopback only
- keep EMQX dashboard/API and NATS monitor private, ideally reachable only by SSH tunnel, VPN, or private network

MQTT:

- if you self-host the fallback broker, publish a separate hostname such as `mqtt.ldtv.dev`
- terminate raw MQTT/TLS directly in EMQX on `8883`
- do not assume the HTTP reverse proxy handles MQTT/TCP
- keep plaintext `1883` loopback-only or private-only

## Port matrix

### App node public edge

- `22/tcp`: SSH, restricted
- `80/tcp`: ACME and optional redirect handling
- `443/tcp`: public API HTTPS

### App node internal-only

- `8080/tcp`: API upstream behind Caddy
- `8081/tcp`: API ops listener (`HTTP_OPS_ADDR`); Prometheus scrapes **`/metrics` here** by default in production, not on `:8080` — see `production-metrics-scraping.md`
- `9091/tcp`: worker metrics/health
- `9092/tcp`: reconciler metrics/health
- `9093/tcp`: mqtt-ingest metrics/health
- `9094/tcp`: temporal-worker metrics/health when enabled

### Data node public edge

- `22/tcp`: SSH, restricted
- `8883/tcp`: public MQTT over TLS when using the fallback data node

### Data node internal-only

- `4222/tcp`: NATS client traffic from app nodes
- `1883/tcp`: plaintext MQTT, private only
- `8222/tcp`: NATS monitor
- `18083/tcp`: EMQX dashboard/API

Managed services remain internal-only as well:

- PostgreSQL `5432/tcp`
- Redis `6379/tcp` or provider TLS port
- object storage private/provider endpoints as applicable

## HTTP hardening defaults

The shared `caddy` config now makes these defaults explicit:

- Caddy admin API disabled
- server and upstream timeouts set explicitly
- request body size capped by `CADDY_MAX_REQUEST_BODY`
- upstream forwarding headers overwritten explicitly
- security headers applied on the public HTTP surface

There is no CORS policy added here because this repo does not define a new public browser-origin requirement in the deployment assets alone.

## GitHub Actions rollout

The production GitHub Actions path stays split into:

- build/test/image publish first
- deploy later

The production deploy job now runs in this order:

1. resolve immutable image refs
2. validate the new app-node/data-node compose assets
3. wait for the `production` environment approval gate
4. sync app-node assets to app node A and app node B
5. optionally sync and deploy the data-node fallback stack when explicitly requested in manual dispatch
6. deploy app node A
7. run app node A health checks
8. deploy app node B
9. run app node B health checks
10. upload a production deployment manifest artifact

For the current 2-VPS snapshot, prefer a single active app node until you have evidence that managed Postgres pool capacity is sufficient for both app nodes.

## Required GitHub environment configuration

Recommended `production` environment variables:

- `PRODUCTION_DEPLOY_ROOT`
- `PROD_APP_A_HOST`
- `PROD_APP_B_HOST`
- `PROD_DATA_NODE_HOST` only if you use the fallback data node
- `PROD_SSH_PORT`
- `PROD_SSH_USER`
- `PRODUCTION_ENABLE_TEMPORAL_PROFILE` when the app-node rollout should include the Temporal profile

Required `production` environment secrets:

- `PROD_SSH_KEY`
- `GHCR_PULL_USERNAME`
- `GHCR_PULL_TOKEN`

The workflow also accepts the older `PRODUCTION_APP_NODE_A_HOST`, `PRODUCTION_APP_NODE_B_HOST`, `PRODUCTION_DATA_NODE_HOST`, `PRODUCTION_SSH_USER`, `PRODUCTION_SSH_PRIVATE_KEY`, and `PRODUCTION_SSH_PORT` names for compatibility, but the `PROD_*` names above are the intended steady state for this snapshot.

Application/runtime secrets such as `DATABASE_URL`, `REDIS_URL`, object storage credentials, `MQTT_*`, SMTP credentials, and payment webhook secrets belong in the real `.env.app-node` / `.env.data-node` files on the target hosts. They are not read directly by the GitHub production deploy workflow.

## Rollback

Manual rollback remains script-driven:

```bash
cd /opt/avf-vending-api/deployments/prod/app-node
bash scripts/rollback_app_node.sh

cd /opt/avf-vending-api/deployments/prod/data-node
bash scripts/rollback_data_node.sh
```

For a cluster-wide app rollback, run `bash scripts/rollback_app_cluster.sh` from an operator host with `APP_NODE_HOSTS` set.

For the full operational checklists around cutover, rollback, backups, and incidents, also use:

- `docs/runbooks/production-cutover-rollback.md`
- `docs/runbooks/production-backup-restore-dr.md`
- `docs/runbooks/production-day-2-incidents.md`

## Validation

From the repository root:

```bash
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml config
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml --profile temporal config
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml --profile migration config
docker compose --env-file deployments/prod/data-node/.env.data-node.example -f deployments/prod/data-node/docker-compose.data-node.yml config
```

## Related

- `deployments/prod/README.md`
- `deployments/prod/app-node/README.md`
- `deployments/prod/data-node/README.md`
- `deployments/prod/shared/env-matrix.md`
- `deployments/prod/shared/network-matrix.md`
- `docs/runbooks/production-cutover-rollback.md`
- `docs/runbooks/production-backup-restore-dr.md`
- `docs/runbooks/production-day-2-incidents.md`
