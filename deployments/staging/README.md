# Staging stack

- **Compose:** `docker-compose.staging.yml` (API, worker, mqtt-ingest, reconciler, postgres, nats, emqx, caddy, migrate)
- **Env file (server, not in git):** `.env.staging` — start from the repo root [`.env.staging.example`](../../.env.staging.example) and the guidance in [docs/runbooks/staging-release.md](../../docs/runbooks/staging-release.md)
- **Deploy:** `scripts/deploy_staging.sh` runs a database environment guard, then migrations, then the application stack
- **Smoke:** `scripts/smoke_staging.sh` wraps the top-level [scripts/smoke_staging.sh](../../scripts/smoke_staging.sh) and can derive `STAGING_BASE_URL` from `API_DOMAIN` in `.env.staging`

See [docs/deployment/environments.md](../../docs/deployment/environments.md) for the local vs staging vs production map.
