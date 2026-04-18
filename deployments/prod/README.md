# Lean production deployment

This directory is the production profile for the existing `avf-vending-api` backend. It is intended for a **single Ubuntu 24.04 VPS** with roughly **2 vCPU, 8 GB RAM, and 100 GB NVMe**.

It reuses the main application image and keeps the stack intentionally small.

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

MQTT in this profile is **plaintext on port `1883`**. `mqtt.ldtv.dev` is still useful as a DNS name for operators and devices, but TLS for MQTT is a separate follow-up.

## DNS

Create these DNS `A` records before the first HTTPS deploy:

- `api.ldtv.dev` -> public IPv4 of the VPS
- `mqtt.ldtv.dev` -> public IPv4 of the VPS

`api.ldtv.dev` is required for Caddy/Let's Encrypt. `mqtt.ldtv.dev` is not reverse-proxied by Caddy in this repo; it is just the broker hostname pointing at the server.

## Network exposure

Public host ports:

- `80/tcp`
- `443/tcp`
- `1883/tcp`

Private/internal only:

- Postgres `5432`
- NATS `4222`
- NATS monitoring `8222`

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

- `IMAGE_TAG`: image label used by Compose and recorded in `.deploy/current_image_tag`
- `API_DOMAIN=api.ldtv.dev`
- `CADDY_ACME_EMAIL`: required for Let's Encrypt
- `POSTGRES_USER` / `POSTGRES_PASSWORD`
- `DATABASE_URL=postgres://...@postgres:5432/avf_vending?sslmode=disable`
- `NATS_URL=nats://nats:4222`
- `EMQX_DASHBOARD_USERNAME` / `EMQX_DASHBOARD_PASSWORD`
- `MQTT_USERNAME` / `MQTT_PASSWORD`
- `MQTT_CLIENT_ID_API` / `MQTT_CLIENT_ID_INGEST`
- `MQTT_BROKER_URL=tcp://emqx:1883`
- `HTTP_AUTH_JWT_SECRET`

Do not commit `.env.production`.

## Bootstrap a fresh Ubuntu 24.04 VPS

Run this as `root` or through `sudo -i`:

```bash
export AVF_DEPLOY_DIR=/opt/avf-vending/avf-vending-api/deployments/prod
bash "${AVF_DEPLOY_DIR}/scripts/bootstrap_vps.sh"
```

What `bootstrap_vps.sh` does:

- installs Docker Engine if missing
- installs baseline packages used by the runbook and scripts (`curl`, `git`, `jq`, `make`, `ufw`, ...)
- enables Docker
- creates a 2 GiB swap file if no swap exists
- applies a conservative UFW baseline for `OpenSSH`, `80`, `443`, and `1883`
- creates `backups/` and `.deploy/`
- installs and enables `avf-vending-prod.service`
- starts the systemd service immediately only if `.env.production` already exists

What it does not do:

- it does not clone the repo for you
- it does not disable SSH password auth
- it does not create real secrets automatically

## First deployment

After `.env.production` is ready:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
chmod +x scripts/*.sh
bash scripts/deploy_prod.sh
```

What `deploy_prod.sh` does:

1. validates `.env.production`
2. runs `docker compose config`
3. builds the application image and goose image
4. starts `postgres`, `nats`, and `emqx`
5. runs migrations with the one-shot `migrate` service
6. bootstraps the EMQX MQTT user
7. starts the long-lived stack: `postgres nats emqx api worker mqtt-ingest reconciler caddy`
8. records the deployed `IMAGE_TAG`
9. runs `scripts/healthcheck_prod.sh` unless `SKIP_SMOKE=1`

If migrations fail, deploy stops immediately.

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

If `.env.production` is missing, systemd start fails immediately by design.

## Day-2 operations

Deploy updated code:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/update_prod.sh
```

Skip `git pull` and just rebuild/redeploy:

```bash
GIT_PULL=0 bash scripts/update_prod.sh
```

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

- reads `.deploy/previous_image_tag`
- exports that tag for Compose
- restarts the long-lived services on the previous image
- updates `.deploy/current_image_tag`

What it does not do:

- it does not roll back the database schema
- it does not restore deleted data
- it only works if a previous tag was recorded by an earlier successful deploy

## Backup and restore

Create a logical Postgres backup:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/backup_postgres.sh
```

Restore from a backup:

```bash
cd /opt/avf-vending/avf-vending-api/deployments/prod
bash scripts/restore_postgres.sh backups/avf_vending_YYYYMMDDTHHMMSSZ.sql.gz
```

Restore behavior:

- stops API, worker, mqtt-ingest, reconciler, and caddy
- pipes the SQL dump into Postgres with `ON_ERROR_STOP=1`
- restarts the long-lived services

Repo-root shortcuts:

```bash
cd /opt/avf-vending/avf-vending-api
make prod-backup
make prod-restore FILE=deployments/prod/backups/avf_vending_YYYYMMDDTHHMMSSZ.sql.gz
```

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

This repository revision does **not** automate MQTT TLS for `mqtt.ldtv.dev`.

Current state:

- broker traffic is plaintext on `1883`
- Caddy only handles HTTPS for `api.ldtv.dev`
- EMQX TLS listeners and certificates would need to be added manually

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
make prod-restore FILE=deployments/prod/backups/avf_vending_YYYYMMDDTHHMMSSZ.sql.gz
make prod-smoke
```

## Common failure points

- DNS for `api.ldtv.dev` is missing or stale, so Caddy cannot complete ACME
- ports `80/443` are blocked upstream, so HTTPS never becomes ready
- `.env.production` is missing or still contains placeholder values
- `DATABASE_URL` does not match `POSTGRES_USER` / `POSTGRES_PASSWORD`
- EMQX dashboard credentials in `.env.production` do not match the initialized EMQX data volume
- `MQTT_USERNAME` / `MQTT_PASSWORD` were not bootstrapped successfully
- `1883/tcp` is blocked by UFW or upstream firewall rules
- readiness fails because `READINESS_STRICT=true` and Postgres is not actually healthy

## Secrets and local artifacts

The repo root `.gitignore` excludes:

- `deployments/prod/.env.production`
- `deployments/prod/.deploy/`
- `deployments/prod/backups/`
- local cert/key material and other restore artifacts under `deployments/prod/`

Only `.env.production.example` should be committed.
