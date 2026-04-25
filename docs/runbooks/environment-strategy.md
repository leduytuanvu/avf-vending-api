# Environment strategy (local, staging, production)

This service uses **four** `APP_ENV` values accepted by `internal/config` validation: **development**, **test** (CI / automation), **staging**, and **production**. Databases, caches, and messaging must stay **isolated** per environment so staging experiments and support traffic never target production data, and so CI/CD promotion remains auditable.

## Model

| Environment | `APP_ENV` | Database | Base URL (example) | MQTT prefix (example) | `PAYMENT_ENV` |
|-------------|-----------|----------|--------------------|------------------------|---------------|
| Local Docker | `development` | Local Postgres in `deployments/docker/docker-compose.yml` | n/a (dev) | `avf-dev/devices` (example) | `sandbox` (default safe) |
| Staging | `staging` | Dedicated Supabase/Postgres (GitHub: `STAGING_DATABASE_URL`) | `https://staging-api.ldtv.dev` | `avf-staging/devices` | `sandbox` only — `PAYMENT_ENV` must be set explicitly to `sandbox` (config rejects unset) |
| Production | `production` | Dedicated Postgres/Supabase (GitHub: `PRODUCTION_DATABASE_URL`) | `https://api.ldtv.dev` | `avf/devices` | `live` only |

**Why staging must not share production’s DB:** same schema is not the same as same blast radius. A bad migration, load test, or operator mistake against a shared DSN can corrupt or exfiltrate live commerce and telemetry. Staging also uses **payment sandbox** credentials, separate Redis/NATS, and a **separate** MQTT topic tree so device routing cannot cross into production.

## Supabase / managed Postgres

Create **one project or logical database per environment**. Store only **non-production** creds in staging secrets. URL-encode reserved characters in the **password** portion of `DATABASE_URL` (e.g. `@` → `%40`) before it appears in a URI. Never commit real URLs in git; use the repo’s `*.example` files and GitHub **Environment** secrets.

## GitHub

- **Staging** environment: `STAGING_DATABASE_URL`, `STAGING_REDIS_URL`, `STAGING_NATS_URL`, `STAGING_MQTT_BROKER_URL`, and PSP **sandbox** keys only.
- **Production** environment: `PRODUCTION_DATABASE_URL`, `PRODUCTION_REDIS_URL`, `PRODUCTION_NATS_URL`, `PRODUCTION_MQTT_BROKER_URL`, and PSP **live** keys only, plus your usual admin/JWT and SSH secrets.

Optional cross-checks: set `STAGING_DATABASE_URL` and `PRODUCTION_DATABASE_URL` in the same **manual** “environment separation” workflow so CI can assert they are not identical (see [`.github/workflows/environment-separation-gates.yml`](../../.github/workflows/environment-separation-gates.yml)). Operators may also set `STAGING_DATABASE_HOST` / `PRODUCTION_DATABASE_HOST` to fail fast if a DSN is pasted for the wrong environment.

## Verify database identity (redacted)

```bash
export APP_ENV=staging
export DATABASE_URL="…"   # from your secret store
bash scripts/verify_database_environment.sh
```

The script prints **scheme, host, port, database name, username, sslmode** and a **redacted** password; it does **not** print the full `DATABASE_URL`. Use it in scripts the same way as production `release.sh` and staging `deploy_staging.sh` (both call it before running Goose).

## Migrations

- **Local:** `make dev-migrate` (uses local Docker DSN and `verify_database_environment.sh` with `APP_ENV=development`).
- **Staging (shell):** `APP_ENV=staging` and `STAGING_DATABASE_URL` / `DATABASE_URL` set, then `make staging-migrate` or the server’s `deployments/staging/scripts/deploy_staging.sh` (which runs the guard before `compose run migrate`).
- **Production:** Migrations in `release.sh` run the guard, then `docker compose up migrate`. For interactive servers, set `CONFIRM_PRODUCTION_MIGRATION=true` (GitHub Actions sets `GITHUB_ACTIONS=true` and does not require the flag).

## Config validation in Go

`go run ./cmd/cli -validate-config` loads env and enforces the same rules as the running binary (e.g. staging cannot use the production `PUBLIC_BASE_URL` or production MQTT prefix). Run it in CI and before deploys when the full set of environment variables is available.

## Promote to production

1. A digest-pinned **staging** pass for the same image (see existing staging deploy and security-verdict flows).
2. `make verify-enterprise-release` (or the enterprise-release CI workflow) green on the release commit.
3. Production deploy using **only** `PRODUCTION_*` secrets and `APP_ENV=production`.
4. Never reuse `STAGING_DATABASE_URL` in production. After deploy, use existing smoke/health and rollback runbooks as needed.

## Related

- [local-dev.md](./local-dev.md) — Docker compose, migrate, test, run binaries.
- [staging-release.md](./staging-release.md) — Staging steps, migration guard, smoke, rollback, storm.
- [production-release-readiness.md](./production-release-readiness.md) — Production gates and evidence.
