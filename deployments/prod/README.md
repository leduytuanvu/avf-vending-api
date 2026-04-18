# Lean production deployment

This directory is the production profile for the existing `avf-vending-api` backend. It is intended for a **single Ubuntu 24.04 VPS** with roughly **2 vCPU, 8 GB RAM, and 100 GB NVMe**.

It reuses the main application image and keeps the stack intentionally small.

## CI/CD foundation

This repo now has a minimal GitHub Actions foundation for quality gates, artifact publishing, security checks, staging validation, and manual production promotion.

- `ci.yml` runs on every pull request and every push to `main`. It uses the Go version from `go.mod`, runs `make ci-gates`, and validates `deployments/docker/docker-compose.yml` with `docker compose config`.
- `security.yml` runs dependency-review on pull requests and `govulncheck` on the repository so supply-chain issues show up before production promotion.
- `build-push.yml` runs on pushes to `main`, tags matching `v*`, and manual dispatch. It builds and pushes the production app image from `deployments/prod/Dockerfile` and the migration image from `deployments/prod/Dockerfile.goose` to GHCR.
- `build-push.yml` now also emits image provenance and SBOM attestations for the pushed production images.
- `deploy-staging.yml` deploys the same prebuilt artifacts to the staging VPS. After a successful `build-push.yml` run for `main`, it deploys the matching `sha-<commit>` automatically; on `workflow_dispatch` it can deploy a chosen image tag through the GitHub `staging` environment.
- `deploy-prod.yml` runs on `workflow_dispatch` only. It promotes a chosen prebuilt image tag to the production VPS over SSH, updates the runtime tag values in `deployments/prod/.env.production` on the server, logs in to GHCR, runs `deployments/prod/scripts/deploy_prod.sh`, and then runs `deployments/prod/scripts/healthcheck_prod.sh`.
- Published image tags include `sha-<commit>`, `main` for the main branch, and the Git tag name when the workflow is triggered from a version tag.
- Production runtime now expects those prebuilt registry artifacts. `deployments/prod/docker-compose.prod.yml` no longer builds the app or goose images locally on the server.
- Production deployment automation is intentionally simple in this pass: a manual GitHub Actions promotion that deploys already-built artifacts to the VPS.

Intended promotion path:

1. PR CI runs `ci.yml`
2. merge to `main` runs security checks, then builds and pushes artifacts
3. `deploy-staging.yml` deploys the matching artifact to staging
4. after staging validation, `deploy-prod.yml` manually promotes a chosen tag to production

## GitHub Actions production promotion

Manual production deploys now go through `.github/workflows/deploy-prod.yml`.

- Trigger it with `workflow_dispatch` and provide `image_tag`.
- The workflow runs in the GitHub `production` environment, so environment protection rules and approvals can gate the deploy.
- Expected `image_tag` values are the same artifact tags produced by CI, typically `sha-<commit>` for an immutable promotion or a release tag like `v1.2.3`.
- The workflow assumes the VPS checkout path is `/opt/avf-vending/avf-vending-api`, logs in to `ghcr.io`, updates `APP_IMAGE_TAG`, `GOOSE_IMAGE_TAG`, and legacy `IMAGE_TAG` in `deployments/prod/.env.production`, then runs the existing deploy and healthcheck scripts on the server.

Required GitHub `production` environment secrets:

- `PROD_HOST`
- `PROD_PORT`
- `PROD_USER`
- `PROD_SSH_KEY`

This workflow promotes prebuilt artifacts only. It does not build source code on the VPS.

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

## Environment setup

Create the real env file from the committed template:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
cp .env.production.example .env.production
```

Fill in every `CHANGE_ME_*` value before deploying.

Important values:

- `IMAGE_REGISTRY`: registry host, for example `ghcr.io`
- `APP_IMAGE_REPOSITORY`: app repository path, for example `avf/avf-vending-api`
- `APP_IMAGE_TAG`: immutable app artifact tag to deploy, for example `sha-<commit>` or `v1.2.3`
- `GOOSE_IMAGE_REPOSITORY`: goose repository path, for example `avf/avf-vending-api-goose`
- `GOOSE_IMAGE_TAG`: goose artifact tag to deploy
- `IMAGE_TAG`: temporary legacy bookkeeping value for older helper scripts; keep it aligned with `APP_IMAGE_TAG` until those scripts are updated
- `API_DOMAIN=api.ldtv.dev`
- `CADDY_ACME_EMAIL`: required for Let's Encrypt
- `POSTGRES_USER` / `POSTGRES_PASSWORD`
- `DATABASE_URL=postgres://...@postgres:5432/avf_vending?sslmode=disable`
- `NATS_URL=nats://nats:4222`
- `EMQX_DASHBOARD_USERNAME` / `EMQX_DASHBOARD_PASSWORD` / `EMQX_NODE_COOKIE`
- `MQTT_USERNAME` / `MQTT_PASSWORD`
- `MQTT_CLIENT_ID_API` / `MQTT_CLIENT_ID_INGEST`
- `MQTT_BROKER_URL=tcp://emqx:1883`
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

## First deployment

After `.env.production` is ready and the referenced images have already been published to your registry:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/deploy_prod.sh
```

Artifact-based rollout flow:

1. validates `.env.production`
2. resolves image references from `IMAGE_REGISTRY`, `APP_IMAGE_REPOSITORY`, `APP_IMAGE_TAG`, `GOOSE_IMAGE_REPOSITORY`, and `GOOSE_IMAGE_TAG`
3. fails early if the required image variables are missing, or if legacy `IMAGE_TAG` does not match `APP_IMAGE_TAG`
4. pulls the prebuilt application image and goose image from the registry
5. starts `postgres`, `nats`, and `emqx`
6. runs migrations with the one-shot `migrate` service
7. bootstraps the EMQX MQTT user
8. starts the long-lived stack: `postgres nats emqx api worker mqtt-ingest reconciler caddy`
9. records deployment state under `.deploy/` after a successful rollout and smoke check
10. runs `scripts/healthcheck_prod.sh` unless `SKIP_SMOKE=1`

If migrations fail, deploy stops immediately.

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

- `scripts/deploy_prod.sh` runs migrations before starting the application stack
- `make prod-up` and the systemd service do **not** run migrations
- `make prod-migrate` runs migrations only
- migration failure is fatal to deploy

Run migrations manually if needed:

```bash
cd /opt/avf-vending/avf-vending-api
make prod-migrate
```

Honest limitation:

- `scripts/rollback_prod.sh` rolls back the **image tag only**
- it does **not** downgrade the database schema
- if you need to undo a bad schema deploy, restore from backup or ship a forward-fix migration

Also note that `migrations/00003_seed_dev.sql` is still in the repo's migration chain. On a brand-new production database it may insert deterministic demo data unless that migration history is changed separately.

## Verification

Main verification command:

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
- public HTTPS `https://api.ldtv.dev/health/live`
- public HTTPS `https://api.ldtv.dev/health/ready`
- when a check fails, recent container status/log context and an operator hint are printed to make post-deploy debugging faster

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

Deploy updated images:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/update_prod.sh
```

If you are rotating to a different artifact tag, update `APP_IMAGE_TAG` and `GOOSE_IMAGE_TAG` in `.env.production` first, then rerun `bash scripts/update_prod.sh`. `update_prod.sh` is now a thin wrapper around the artifact-based deploy flow; it does not `git pull`, build source, or rebuild images on the server.

Restart the long-lived stack without running migrations:

```bash
cd /opt/avf-vending/avf-vending-api
make prod-up
make prod-restart
make prod-down
```

Inspect status and logs:

```bash
cd /opt/avf-vending/avf-vending-api
make prod-status
make prod-logs
```

## Rollback

Rollback command:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/rollback_prod.sh
```

What it does:

- reads the previously recorded legacy image tag state from `.deploy/previous_image_tag`
- still uses the older single-tag helper flow
- updates `.env.production` so `APP_IMAGE_TAG`, `GOOSE_IMAGE_TAG`, and legacy `IMAGE_TAG` all move back to the recorded previous tag
- works best when normal deploys keep app and goose on the same artifact tag

What it does not do:

- it does not roll back the database schema
- it does not restore deleted data
- it only works if a previous tag was recorded by an earlier successful deploy
- it assumes a single rollback tag for both app and goose images

The practical distinction:

- `rollback_prod.sh` is an **image rollback helper only**
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

## EMQX dashboard

The dashboard is intentionally not public.

SSH tunnel from your laptop:

```bash
ssh -L 18083:127.0.0.1:18083 ubuntu@YOUR_VPS
```

Then open:

```text
http://127.0.0.1:18083
```

The MQTT application user is created by `scripts/emqx_bootstrap.sh` using the dashboard API and the credentials from `.env.production`.

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

## Common failure points

- DNS for `api.ldtv.dev` is missing or stale, so Caddy cannot complete ACME
- ports `80/443` are blocked upstream, so HTTPS never becomes ready
- `.env.production` is missing or still contains placeholder values
- `DATABASE_URL` does not match `POSTGRES_USER` / `POSTGRES_PASSWORD`
- EMQX dashboard credentials in `.env.production` do not match the initialized EMQX data volume
- `MQTT_USERNAME` / `MQTT_PASSWORD` were not bootstrapped successfully
- `8883/tcp` is blocked by UFW or upstream firewall rules when testing MQTT TLS exposure
- readiness fails because `READINESS_STRICT=true` and Postgres is not actually healthy

## Secrets and local artifacts

The repo root `.gitignore` excludes:

- `deployments/prod/.env.production`
- `deployments/prod/.deploy/`
- `deployments/prod/backups/`
- local cert/key material and other restore artifacts under `deployments/prod/`

Only `.env.production.example` should be committed.
