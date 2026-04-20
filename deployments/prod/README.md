# Lean production deployment

This directory is the production profile for the existing `avf-vending-api` backend. It is intended for a **single Ubuntu 24.04 VPS** with roughly **2 vCPU, 8 GB RAM, and 100 GB NVMe**.

**Architecture (default):** GitHub Actions builds Linux images and pushes them to **GHCR**. The VPS **only pulls images and restarts containers** via `docker compose` and `scripts/release.sh` — there is **no** `docker compose build` for app/goose on the server and **no** Go toolchain required on the VPS for releases.

> **Deprecated as the default path:** treating the VPS as a place to compile the API or `docker build` application images. If you maintain a custom fork workflow, keep it out of the default documented runbook; align with image refs in `.env.production` instead.

## CI/CD foundation

This repo has GitHub Actions for quality gates, security, optional standalone image publishing, staging, and **production deploy**.

- `ci.yml` runs on pull requests and pushes to `main` (`make ci-gates`, etc.).
- `security.yml` runs dependency review and `govulncheck`.
- `build-push.yml` (optional parallel path) can build/push app + goose images on `main` / tags; provenance/SBOM may be enabled there.
- `deploy-staging.yml` deploys prebuilt artifacts to the staging VPS (GitHub `staging` environment).
- **`deploy-prod.yml`** (GitHub `production` environment): on **push to `main`** or **`workflow_dispatch`**, it **builds and pushes** the app image (`deployments/prod/Dockerfile`) and goose image (`deployments/prod/Dockerfile.goose`) to GHCR, then **syncs the required `deployments/prod` runtime assets** to the VPS, **SSHs to the VPS**, and runs `release.sh deploy` with `RELEASE_TAG=sha-<GITHUB_SHA>`. **`VPS_DEPLOY_PATH` must be the deploy root on the server** (the directory that contains `deployments/prod/`); it does **not** need to be a git checkout. Equivalent manual command on the server:

  ```bash
  bash "${VPS_DEPLOY_PATH}/deployments/prod/scripts/release.sh" deploy sha-<full-git-sha>
  ```

  The VPS does **not** compile source; it pulls tags and runs compose + smoke checks.

Published tags for `main` builds typically include `sha-<commit>`, `main`, and `latest` on both image repositories (see workflow for exact naming).

Intended promotion path:

1. PR CI runs `ci.yml`
2. Merge to `main` runs `deploy-prod.yml` (build/push + production SSH) when that workflow is enabled, **or** use `build-push.yml` plus a manual `release.sh deploy <tag>` on the VPS
3. `deploy-staging.yml` can track `main` for staging validation
4. Production is gated by the `production` environment (approvals optional)

## GitHub Actions production deploy (`deploy-prod.yml`)

The workflow runs in the **`production`** environment (use branch protection / required reviewers as needed).

**Jobs:**

1. **build-and-push** — checkout, Buildx, login to GHCR with `GITHUB_TOKEN`, push  
   `ghcr.io/<lowercase-github-repository>` and `ghcr.io/<lowercase-github-repository>-goose` (same naming as `build-push.yml`), with tags `sha-<sha>`, and on `main` also `main` and `latest` on both images.
2. **deploy-prod** — checks out the repo on the GitHub runner, validates deploy secrets, syncs the required `deployments/prod` runtime assets to the VPS (compose, scripts, Caddyfile, EMQX base config, docs/examples), then SSHes to the VPS, exports `IMAGE_REGISTRY`, `APP_IMAGE_REPOSITORY`, `GOOSE_IMAGE_REPOSITORY`, `RELEASE_TAG`, `EMQX_API_KEY`, `EMQX_API_SECRET`, runs `docker login` when pull secrets are set, and finally runs  
   `bash "$VPS_DEPLOY_PATH/deployments/prod/scripts/release.sh" deploy "$RELEASE_TAG"`.

**Repository or `production` environment secrets** (names must match the workflow):

| Secret | Purpose |
|--------|---------|
| `VPS_HOST` | SSH hostname or IP |
| `VPS_SSH_PORT` | SSH port (e.g. `22`) |
| `VPS_USER` | SSH login user |
| `VPS_SSH_PRIVATE_KEY` | PEM private key for SSH |
| `VPS_DEPLOY_PATH` | Deploy root on the server containing `deployments/prod` (e.g. `/opt/avf-vending/avf-vending-api`); it does not need to be a git checkout |
| `EMQX_API_KEY` | Required: EMQX management API key used for `/api/v5/*` bootstrap on the VPS |
| `EMQX_API_SECRET` | Required: EMQX management API secret paired with `EMQX_API_KEY` |
| `GHCR_PULL_USERNAME` | Optional: `docker login ghcr.io` user on the VPS when images are **private** |
| `GHCR_PULL_TOKEN` | Optional: PAT or token with `read:packages` (set **both** with username, or **neither** for public packages — matches `release.sh`) |

The workflow never runs `go build` or `docker compose build` **on the VPS** for app/goose images.

## GitHub Actions staging deployment

Staging uses the same built artifacts as production, but deploys them through `.github/workflows/deploy-staging.yml` and separate staging deployment assets under `deployments/staging/`.

- GitHub environment: `staging`
- Required staging secrets: `STAGING_HOST`, `STAGING_PORT`, `STAGING_USER`, `STAGING_SSH_KEY`
- Automatic path: push to `main` deploys `sha-<commit>` for that merge
- Manual path: `workflow_dispatch` can deploy a chosen tag such as `sha-<commit>` or `v1.2.3`
- Runtime files on the staging VPS are expected at `deployments/staging/.env.staging`, `deployments/staging/docker-compose.staging.yml`, `deployments/staging/scripts/deploy_staging.sh`, and `deployments/staging/scripts/healthcheck_staging.sh`

## What this profile includes

- `caddy` for public HTTPS on `api.ldtv.dev`
- `api` (`cmd/api`)
- `worker` (`cmd/worker`)
- `mqtt-ingest` (`cmd/mqtt-ingest`)
- `reconciler` (`cmd/reconciler`)
- `postgres` (internal only)
- `nats` with JetStream (internal only)
- `emqx` single-node MQTT broker
- `migrate` one-shot goose container for schema migrations

## What it intentionally excludes

- ClickHouse, Temporal, MinIO, Redis, Prometheus, Grafana, Loki, and other always-on heavy services
- Public Postgres or public NATS
- Public EMQX dashboard
- Automated MQTT TLS on `8883`

For backend artifacts, this profile expects an **external S3-compatible object store** when artifact APIs are enabled. It does not bundle MinIO or another object-store service into the production stack.

Observability is now available as an **optional overlay**, not part of the core always-on base stack. That keeps the default production runtime smaller and lets operators enable metrics/log aggregation only when the VPS has enough headroom.

MQTT in this profile now treats **plaintext `1883` as internal-only** inside the Docker network. The intended public broker path is **TLS on `8883`**, with certificate enablement documented under `deployments/prod/emqx/README.md`.

## DNS

Create these DNS `A` records before the first HTTPS deploy:

- `api.ldtv.dev` -> public IPv4 of the VPS
- `mqtt.ldtv.dev` -> public IPv4 of the VPS

`api.ldtv.dev` is required for Caddy/Let's Encrypt. `mqtt.ldtv.dev` is not reverse-proxied by Caddy in this repo; it is just the broker hostname pointing at the server.

## Network exposure

Public host ports:

- `80/tcp`
- `443/tcp`
- `8883/tcp` (MQTT TLS path once the EMQX TLS listener is enabled)

Private/internal only:

- Postgres `5432`
- NATS `4222`
- NATS monitoring `8222`
- MQTT plaintext `1883` on the Docker internal network only

Loopback only on the VPS:

- EMQX dashboard `127.0.0.1:18083`

## Files and assumptions

Example checkout path used below:

```bash
/opt/avf-vending/avf-vending-api
```

This production directory is then:

```bash
/opt/avf-vending/avf-vending-api/deployments/prod
```

All commands below assume you run them from `deployments/prod` unless noted otherwise.

## Pre-rollout checks (telemetry + compose)

Run these **before** increasing fleet traffic or merging deployment changes. Paths below use the repository root; on the VPS that is typically `/opt/avf-vending/avf-vending-api`.

1. **Compose must render** (substitution and `:?` placeholders satisfied in your real `.env.production`):

   ```bash
   docker compose --env-file deployments/prod/.env.production -f deployments/prod/docker-compose.prod.yml config
   ```

2. **Telemetry production rules** (`APP_ENV=production` requires `NATS_URL`; forbids `TELEMETRY_LEGACY_POSTGRES_INGEST=true`). Exits non-zero on failure:

   ```bash
   bash deployments/prod/scripts/validate_prod_telemetry.sh
   ```

   Optional: `STRICT_METRICS_WARNINGS=1 bash deployments/prod/scripts/validate_prod_telemetry.sh` fails if `METRICS_ENABLED` is not `true` (use in CI when you require metrics on).

3. **Go build** (optional; on a **developer or CI machine** only — not on the VPS release path):

   ```bash
   go build ./...
   ```

4. **Post-deploy smoke** (containers must be running; from `deployments/prod`):

   ```bash
   bash scripts/healthcheck_prod.sh
   ```

   From the repo root you can use `make prod-smoke-full` (runs validate, then healthcheck) if your working directory is `deployments/prod` and `.env.production` exists there—see `Makefile` targets `prod-validate-telemetry`, `prod-compose-config`, `prod-smoke`, `prod-smoke-full`.

**Runbooks:** [telemetry-production-rollout.md](../../docs/runbooks/telemetry-production-rollout.md), [telemetry-jetstream-resilience.md](../../docs/runbooks/telemetry-jetstream-resilience.md). **Burst observation checklist:** `bash scripts/telemetry_load_smoke.sh`.

## Environment setup

Create the real env file from the committed template:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
cp .env.production.example .env.production
```

Fill in every `CHANGE_ME_*` value before deploying.

Important values:

- `IMAGE_REGISTRY`: registry host, for example `ghcr.io`
- `APP_IMAGE_REPOSITORY`: GHCR path segment for the **app** image (e.g. `myorg/avf-vending-api`), matching CI pushes
- `GOOSE_IMAGE_REPOSITORY`: GHCR path segment for the **goose** image (e.g. `myorg/avf-vending-api-goose`, i.e. `<APP_IMAGE_REPOSITORY>-goose`), matching `build-push.yml` / `deploy-prod.yml`
- `APP_IMAGE_TAG` / `GOOSE_IMAGE_TAG`: immutable tags for app vs migrate images (often the same `sha-<commit>`); `release.sh deploy [app [goose]]` writes these
- `IMAGE_TAG` (optional): legacy convenience; if `APP_IMAGE_TAG` or `GOOSE_IMAGE_TAG` is unset, deploy helpers fall back to `IMAGE_TAG` for that slot. It does **not** need to match `APP_IMAGE_TAG`
- `PUBLIC_BASE_URL`: public HTTPS base URL for your API (documentation / integrations; example: `https://api.example.com`)
- `API_DOMAIN=api.ldtv.dev`
- `CADDY_ACME_EMAIL`: required for Let's Encrypt
- `POSTGRES_USER` / `POSTGRES_PASSWORD`
- `DATABASE_URL=postgres://...@postgres:5432/avf_vending?sslmode=disable`
- `NATS_URL=nats://nats:4222` (required for `APP_ENV=production`; see **Telemetry safety** block in `.env.production.example`)
- `TELEMETRY_*` / `TELEMETRY_LEGACY_POSTGRES_INGEST=false` — single grouped section in `.env.production.example`; do not enable legacy Postgres telemetry ingest in production
- `EMQX_DASHBOARD_USERNAME` / `EMQX_DASHBOARD_PASSWORD` / `EMQX_NODE_COOKIE`
- `EMQX_API_KEY` / `EMQX_API_SECRET` (must match `emqx/default_api_key.conf`; see EMQX dashboard section)
- `MQTT_USERNAME` / `MQTT_PASSWORD`
- `MQTT_CLIENT_ID_API` for the API process when you want an in-process publisher identity
- `MQTT_CLIENT_ID_INGEST` for `mqtt-ingest`; if omitted, Compose falls back to `avf-prod-mqtt-ingest`
- `MQTT_BROKER_URL=tcp://emqx:1883`
- `API_REQUIRE_MQTT_PUBLISHER=false` when the API publisher is optional in this deployment; set it to `true` only when API startup must fail if the in-process MQTT publisher cannot initialize
- `HTTP_AUTH_JWT_SECRET`
- if `API_ARTIFACTS_ENABLED=true`, also set `S3_BUCKET`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and either `AWS_REGION` or `S3_REGION`
- for non-AWS providers, also set `S3_ENDPOINT` and enable `S3_USE_PATH_STYLE` only when the provider requires it

Do not commit `.env.production`.

For validation or dry-run commands, you can temporarily override the default runtime env file with `PROD_ENV_FILE`, but normal production usage should keep the default `.env.production`.

## Object storage for artifacts

Artifact APIs are disabled by default. When `API_ARTIFACTS_ENABLED=true`, production expects a real S3-compatible bucket and credentials; the API does not provision buckets or object-store users for you.

Required object-store env:

- `S3_BUCKET`
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_REGION` or `S3_REGION`

Additional provider-specific env:

- `S3_ENDPOINT` for non-AWS S3-compatible storage
- `S3_USE_PATH_STYLE=true` only when your provider requires path-style addressing

Operational expectations:

- create the bucket ahead of time
- scope credentials to the required bucket/prefix only
- keep object-store credentials out of git and out of screenshots/runbooks
- leave `API_ARTIFACTS_ENABLED=false` unless you actually want the admin artifact routes mounted

## Bootstrap a fresh Ubuntu 24.04 VPS

Run the bootstrap helper as `root` or through `sudo -i`, but treat that as a one-time host preparation step. Normal deploy, smoke-check, and incident-response commands should run from the repo checkout as your deploy user where practical.

```bash
export AVF_DEPLOY_DIR=/opt/avf-vending/avf-vending-api/deployments/prod
bash "${AVF_DEPLOY_DIR}/scripts/bootstrap_vps.sh"
```

What `bootstrap_vps.sh` does:

- installs Docker Engine if missing
- installs baseline packages used by the runbook and scripts (`curl`, `git`, `jq`, `make`, `ufw`, ...)
- enables Docker
- creates a 2 GiB swap file if no swap exists
- applies a conservative UFW baseline for `OpenSSH`, `80`, `443`, and `8883`
- creates `backups/` and `.deploy/`
- tries to make `backups/` and `.deploy/` writable by the inferred deploy user instead of leaving them root-owned
- installs and enables `avf-vending-prod.service`
- starts the systemd service immediately only if `.env.production` already exists

What it does not do:

- it does not clone the repo for you
- it does not disable SSH password auth
- it does not create real secrets automatically
- it does not replace normal deploy-user operational hygiene with root-only workflows

Firewall expectation:

- `OpenSSH`, `80/tcp`, `443/tcp`, and `8883/tcp` are opened by default
- if you use a non-standard SSH port, adjust UFW separately before or immediately after bootstrap
- Postgres, NATS, and the EMQX dashboard remain non-public in this profile

## First deployment (image-only)

See also: [GHCR image-only deploy / rollback](../../docs/runbooks/prod-ghcr-image-only-deploy.md).

After `.env.production` is filled out and images for your chosen tag exist in GHCR:

**Preferred (canonical):** `scripts/release.sh` — same entrypoint GitHub Actions uses over SSH.

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
# Example: deploy the tag produced by CI for this commit (replace with your real tag)
bash scripts/release.sh deploy sha-0123456789abcdef0123456789abcdef01234567
```

`release.sh deploy [app_tag [goose_tag]]` (or no args to re-apply tags already in `.env.production`):

1. Validates required env (including `IMAGE_*`, `DATABASE_URL`, etc.)
2. Writes `IMAGE_REGISTRY`, `APP_IMAGE_REPOSITORY`, `GOOSE_IMAGE_REPOSITORY`, `APP_IMAGE_TAG`, `GOOSE_IMAGE_TAG`, and sets legacy `IMAGE_TAG` to the **app** tag (exported CI vars override file values when set)
3. Optionally runs `docker login ghcr.io` when `GHCR_PULL_USERNAME` and `GHCR_PULL_TOKEN` are set in the shell
4. Runs `docker compose config`, `docker compose pull` for app/goose-related services, starts data plane, runs **`docker compose up migrate`**, runs `emqx_bootstrap.sh`, brings up the long-lived stack, then **polls the rollout gate** (postgres, nats, emqx, api, worker, mqtt-ingest, reconciler, caddy): each container must be **running**, and if Docker defines a healthcheck it must reach **healthy** before the wait ends. Env: `ROLLUP_HEALTH_WAIT_SECS` (default **180**, clamped **30–3600** so `0` cannot instant-fail), `ROLLUP_HEALTH_POLL_SECS` (default **5**, minimum **1**). On timeout the script prints `docker compose ps`, per-container `docker inspect` state/health, and log tails for failing gate services, then exits non-zero.
5. Runs `healthcheck_prod.sh` unless `SKIP_SMOKE=1` (that script **also** polls the same eight containers for Docker health with `STACK_DOCKER_HEALTH_WAIT_SECS` / `STACK_DOCKER_HEALTH_POLL_SECS`, same defaults and clamp, before one-shot verification and internal/public checks).
6. Records `.deploy/current_*` / `previous_*` image tags (app and goose separately) plus `history.log`

If migrations fail, the script exits before switching the full app stack.

**Legacy (still supported, not the primary doc path):** `bash scripts/deploy_prod.sh` performs a similar image-only pull + migrate + stack rollout. Prefer `release.sh` for new automation and operator training.

**Render compose before you trust a new env file:**

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
docker compose --env-file .env.production -f docker-compose.prod.yml config >/dev/null && echo OK
```

MQTT deployment note:

- `mqtt-ingest` is always part of the long-lived production stack and always needs a concrete MQTT client ID
- Compose injects `MQTT_CLIENT_ID` into `mqtt-ingest` from `MQTT_CLIENT_ID_INGEST`, with a fallback of `avf-prod-mqtt-ingest`
- the API's in-process MQTT publisher is a separate concern controlled by `API_REQUIRE_MQTT_PUBLISHER`
- when `API_REQUIRE_MQTT_PUBLISHER=false`, the API can still boot even if the publisher cannot initialize; when `true`, API startup fails fast on publisher bootstrap errors

## Optional observability overlay

The core production runtime remains `deployments/prod/docker-compose.prod.yml`. Observability lives in the additive overlay `deployments/prod/docker-compose.observability.yml` plus config under `deployments/prod/observability/`.

What the overlay adds today:

- `prometheus` for scraping real application/runtime metrics that the repo already exposes
- `grafana` with provisioned Prometheus/Loki datasources and existing AVF dashboards
- `loki` for aggregated container logs
- `promtail` for shipping Docker json-file logs into Loki

What it intentionally does **not** add yet:

- Tempo or another production trace backend
- custom synthetic metrics that the app does not already emit
- changes to the base deployment scripts or base Compose topology

Important behavior:

- the overlay turns on `METRICS_ENABLED=true` for `api`, `worker`, `reconciler`, and `mqtt-ingest`
- it also overrides worker/reconciler/mqtt-ingest metrics listeners to `0.0.0.0` on the internal Docker network so Prometheus can scrape them
- because those env overrides live in the overlay, start or refresh the overlay with **both** compose files

Start or refresh the overlay:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
docker compose --env-file .env.production -f docker-compose.prod.yml -f docker-compose.observability.yml up -d api worker mqtt-ingest reconciler prometheus loki promtail grafana
```

Render and inspect the combined config:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
docker compose --env-file .env.production -f docker-compose.prod.yml -f docker-compose.observability.yml config
```

Overlay operator ports:

- Grafana: `127.0.0.1:3000`
- Prometheus: `127.0.0.1:9090`
- Loki API: `127.0.0.1:3100`

Suggested access pattern is SSH tunneling from your laptop rather than publishing these services broadly:

```bash
ssh -L 3000:127.0.0.1:3000 -L 9090:127.0.0.1:9090 -L 3100:127.0.0.1:3100 ubuntu@YOUR_VPS
```

Overlay credentials / env notes:

- set `GRAFANA_ADMIN_USER` and `GRAFANA_ADMIN_PASSWORD` in `.env.production` (or export them in the shell before running the combined compose command)
- the overlay reuses the existing app env, image tags, and internal service names from the base production stack
- dashboards and datasources are provisioned from `deployments/prod/observability/grafana/provisioning/`

Signals wired safely today:

- API `/metrics` on the main HTTP server when `METRICS_ENABLED=true`
- worker, reconciler, and mqtt-ingest Prometheus endpoints on internal-only listeners
- Docker container logs collected by Promtail from the host Docker json log files

## Migrations

Schema migrations come from repo-root `migrations/*.sql`.

Important behavior:

- **`scripts/release.sh deploy`** and **`scripts/deploy_prod.sh`** both run migrations (goose one-shot) before / as part of bringing the application stack to the new images
- `make prod-up` and the systemd service do **not** run migrations by themselves
- `make prod-migrate` runs migrations only
- migration failure is fatal to `release.sh deploy` / `deploy_prod.sh`

Run migrations manually if needed:

```bash
cd /opt/avf-vending/avf-vending-api
make prod-migrate
```

Honest limitation:

- **`scripts/release.sh rollback`** and **`scripts/rollback_prod.sh`** roll back the **image tag only**
- they do **not** downgrade the database schema
- if you need to undo a bad schema deploy, restore from backup or ship a forward-fix migration

Also note that `migrations/00003_seed_dev.sql` is still in the repo's migration chain. On a brand-new production database it may insert deterministic demo data unless that migration history is changed separately.

## Verification

Preflight config render:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
docker compose --env-file .env.production -f docker-compose.prod.yml config >/dev/null
```

Inspect compose state:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
docker compose --env-file .env.production -f docker-compose.prod.yml ps
```

Internal API live check from inside the running container:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/live | grep -qx ok'
```

Public HTTPS live check:

```bash
curl -fsS "https://${API_DOMAIN}/health/live"
```

Main verification helper:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/healthcheck_prod.sh
```

The health script checks:

- `docker compose ps`
- all core containers are running
- Postgres responds to `pg_isready`
- NATS responds on its internal health endpoint
- EMQX reports broker status
- API `/health/live` and `/health/ready` inside the container
- public HTTPS `https://${API_DOMAIN}/health/live`
- public HTTPS `https://${API_DOMAIN}/health/ready`
- when a check fails, recent container status/log context and an operator hint are printed to make post-deploy debugging faster
- if the internal checks pass but the public HTTPS checks fail, treat that as DNS/TLS/routing investigation first; use `SKIP_PUBLIC_HTTPS=1 bash scripts/healthcheck_prod.sh` while external DNS or ACME issuance is still converging

Repo-root shortcut:

```bash
cd /opt/avf-vending/avf-vending-api
make prod-smoke
```

## systemd auto-start

Bootstrap installs `deployments/prod/systemd/avf-vending-prod.service`.

It starts this long-lived service set on boot:

```bash
postgres nats emqx api worker mqtt-ingest reconciler caddy
```

Useful commands:

```bash
sudo systemctl start avf-vending-prod
sudo systemctl stop avf-vending-prod
sudo systemctl restart avf-vending-prod
sudo systemctl status avf-vending-prod
sudo journalctl -u avf-vending-prod -f
```

Operational notes:

- the unit is intentionally a `oneshot` wrapper around `docker compose up -d` / `down`
- `WorkingDirectory` is the production directory itself, installed from `deployments/prod/systemd/avf-vending-prod.service`
- startup fails fast if `.env.production` or `docker-compose.prod.yml` is missing
- if `.env.production` is missing, systemd start fails immediately by design

## Day-2 operations

**Deploy updated images (recommended):**

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/release.sh deploy sha-<new-commit-sha>
```

`release.sh deploy` updates image tags in `.env.production`, validates the synced prod runtime assets, renders `emqx/default_api_key.conf` from `EMQX_API_*`, force-recreates EMQX, preflights `/api/v5/status`, bootstraps the MQTT user, restarts the stack, and runs smoke checks — **no source build on the VPS**.

**Alternate:** `bash scripts/update_prod.sh` remains a thin wrapper around the same image-pull / compose flow; it does not `git pull`, compile Go, or `docker build` app images on the server.

If you edit tags by hand, set `APP_IMAGE_TAG` and `GOOSE_IMAGE_TAG` (or rely on `IMAGE_TAG` as fallback for missing slots), then run `release.sh deploy` with no arguments, `bash scripts/update_prod.sh`, or `release.sh deploy <app_tag> [goose_tag]` so pulls and health checks still run in order.

Restart the long-lived stack without running migrations:

```bash
cd /opt/avf-vending/avf-vending-api
make prod-up
make prod-restart
make prod-down
```

Inspect status and logs (compose-wide or per service):

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/release.sh status
bash scripts/release.sh logs           # all long-lived services, tail 200
bash scripts/release.sh logs api 400   # api only, last 400 lines
```

From the repo root you can still use `make prod-status` / `make prod-logs` as convenience wrappers.

## Rollback

**Preferred:** `release.sh rollback` restores tags from `.deploy/previous_app_image_tag` and `.deploy/previous_goose_image_tag` (when present; otherwise falls back to legacy `.deploy/previous_image_tag` for both), or pass explicit tags:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/release.sh rollback
bash scripts/release.sh rollback sha-<known-good-app>
bash scripts/release.sh rollback sha-<known-good-app> sha-<known-good-goose>
```

What it does:

- updates `.env.production` so `APP_IMAGE_TAG`, `GOOSE_IMAGE_TAG`, and legacy `IMAGE_TAG` (set to the app tag) target the rollback selection
- optional GHCR login when pull secrets are present in the environment
- `docker compose pull` + `up -d` for the long-lived stack, container checks, and `healthcheck_prod.sh` (unless `SKIP_SMOKE=1`)
- appends a line to `.deploy/history.log`

What it does not do:

- it does not roll back the database schema
- it does not restore deleted data
- default `rollback` without tags requires a prior successful `release.sh deploy` that recorded `.deploy/previous_app_image_tag` or legacy `.deploy/previous_image_tag`

**Legacy:** `bash scripts/rollback_prod.sh` performs a similar **image-only** rollback using the same `.deploy/previous_image_tag` convention. Prefer `release.sh rollback` for consistency with CI.

The practical distinction:

- **`release.sh rollback`** / `rollback_prod.sh` are **image rollback helpers only**
- `backup_postgres.sh` creates the database recovery artifact
- `restore_postgres.sh` is the **destructive data recovery path** when schema/data must be put back from backup

## Backup and restore

Create a logical Postgres backup:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/backup_postgres.sh
```

Safe backup preflight without writing a dump:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
DRY_RUN=1 bash scripts/backup_postgres.sh
```

Backup behavior:

- reads `PROD_BACKUP_DIR` from `.env.production`
- validates Compose config and Postgres readiness before writing
- writes a timestamped gzip SQL dump to a temporary file first, then moves it into place only after gzip integrity checks pass
- prints the resolved output path and final size for operator confirmation

Restore from a backup:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/restore_postgres.sh --yes backups/avf_vending_YYYYMMDDTHHMMSSZ.sql.gz
```

Safe restore preflight:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/restore_postgres.sh --preflight backups/avf_vending_YYYYMMDDTHHMMSSZ.sql.gz
```

Restore behavior:

- requires explicit `--yes` confirmation for destructive execution
- `--preflight` validates the dump, Compose config, and Postgres readiness without stopping services or changing data
- stops API, worker, mqtt-ingest, reconciler, and caddy
- pipes the SQL dump into Postgres with `ON_ERROR_STOP=1`
- restarts the long-lived services
- if restore fails after writers are stopped, you must inspect the database state and restart services manually when it is safe

Repo-root shortcuts:

```bash
cd /opt/avf-vending/avf-vending-api
make prod-backup
make prod-restore FILE=deployments/prod/backups/avf_vending_YYYYMMDDTHHMMSSZ.sql.gz CONFIRM=YES
```

When backups are strongly recommended:

- immediately before any deploy that includes schema migrations or other irreversible data-shape changes
- before attempting a manual restore or other destructive database maintenance
- before high-risk operational work where an image rollback would not be enough

Restore caveats:

- restoring data is not the same as rolling back containers
- a backup restore may reintroduce older schema/data assumptions from the dump time
- if the running image expects newer schema than the restored dump provides, you may need a forward-fix migration or a coordinated image/tag rollback plan
- `migrations/00003_seed_dev.sql` remains in the migration chain, so be careful when reasoning about very old or freshly initialized databases

## EMQX dashboard and credentials

EMQX uses **three separate credential classes** in this deployment:

1. **Dashboard (web UI)** — `EMQX_DASHBOARD_USERNAME` / `EMQX_DASHBOARD_PASSWORD` in `.env.production` feed Compose defaults for the loopback dashboard only. They are **not** read by `scripts/emqx_bootstrap.sh`.

2. **Management REST API** — `EMQX_API_KEY` / `EMQX_API_SECRET` in `.env.production` are used by `scripts/emqx_bootstrap.sh` as **HTTP Basic** authentication (`-u key:secret`) against `/api/v5/*`. The broker loads API keys from **`emqx/default_api_key.conf`** (`api_key.bootstrap_file` in [`emqx/base.hocon`](emqx/base.hocon)). During GitHub Actions deploys, `release.sh` renders `emqx/default_api_key.conf` on the VPS from `EMQX_API_KEY` / `EMQX_API_SECRET`, `chmod 600`s it, force-recreates `avf-prod-emqx`, preflights `http://127.0.0.1:18083/api/v5/status`, and only then runs `emqx_bootstrap.sh`. Do **not** commit `default_api_key.conf`.

3. **MQTT application / devices** — `MQTT_USERNAME` / `MQTT_PASSWORD` are the built-in-database MQTT users; `emqx_bootstrap.sh` creates them idempotently via the management API.

The dashboard is intentionally not public.

SSH tunnel from your laptop:

```bash
ssh -L 18083:127.0.0.1:18083 ubuntu@YOUR_VPS
```

Then open:

```text
http://127.0.0.1:18083
```

## MQTT TLS status

This repository revision prepares EMQX for MQTT TLS, but it does **not** provision or rotate broker certificates for `mqtt.ldtv.dev` automatically.

Current state:

- broker traffic inside the Docker network remains plaintext on `1883`
- Caddy only handles HTTPS for `api.ldtv.dev`
- public MQTT TLS on `8883` still requires operators to place real certificates and enable the listener

See `deployments/prod/emqx/README.md` before attempting that change.

## Makefile helpers

From the repo root:

```bash
make prod-up
make prod-down
make prod-restart
make prod-status
make prod-logs
make prod-migrate
make prod-deploy
make prod-backup
make prod-restore FILE=deployments/prod/backups/avf_vending_YYYYMMDDTHHMMSSZ.sql.gz CONFIRM=YES
make prod-smoke
```

`make prod-deploy` invokes `deployments/prod/scripts/deploy_prod.sh` (wrapper → `release.sh`). For parity with GitHub Actions, from **`deployments/prod`** on the VPS run **`bash scripts/release.sh deploy <tag>`** (same path shape as CI under `"${VPS_DEPLOY_PATH}/deployments/prod/scripts/release.sh"`). The GitHub workflow syncs the runtime deploy assets first, then `release.sh` renders `emqx/default_api_key.conf`, force-recreates EMQX, preflights `/api/v5/status`, and only then bootstraps the MQTT user.

## Common failure points

- DNS for `api.ldtv.dev` is missing or stale, so Caddy cannot complete ACME
- ports `80/443` are blocked upstream, so HTTPS never becomes ready
- `.env.production` is missing or still contains placeholder values
- `DATABASE_URL` does not match `POSTGRES_USER` / `POSTGRES_PASSWORD`
- GitHub Actions did not sync the latest `deployments/prod` runtime assets to the VPS, so `release.sh`, `docker-compose.prod.yml`, or `emqx/base.hocon` are stale
- `release.sh` could not render `emqx/default_api_key.conf` on the VPS (permissions, missing directory, or bad `EMQX_API_*`)
- `EMQX_API_KEY` / `EMQX_API_SECRET` supplied by GitHub Actions or `.env.production` do not match the generated `emqx/default_api_key.conf` (HTTP **401** on EMQX preflight / bootstrap)
- EMQX management API unreachable (broker down, or loopback `18083` not reachable from the host running `emqx_bootstrap.sh`)
- EMQX was not recreated after `base.hocon` or `default_api_key.conf` changed, so `/opt/emqx/etc/default_api_key.conf` inside `avf-prod-emqx` is stale
- `EMQX_DASHBOARD_USERNAME` / `EMQX_DASHBOARD_PASSWORD` do not match the initialized EMQX data volume (dashboard login only; unrelated to bootstrap API auth)
- `MQTT_USERNAME` / `MQTT_PASSWORD` were not bootstrapped successfully after fixing API credentials
- `8883/tcp` is blocked by UFW or upstream firewall rules when testing MQTT TLS exposure
- readiness fails because `READINESS_STRICT=true` and Postgres is not actually healthy

## Secrets and local artifacts

The repo root `.gitignore` excludes:

- `deployments/prod/.env.production`
- `deployments/prod/.deploy/`
- `deployments/prod/backups/`
- local cert/key material and other restore artifacts under `deployments/prod/`

Only `.env.production.example` and `emqx/default_api_key.conf.example` should be committed; keep `emqx/default_api_key.conf` (secrets) out of git.
