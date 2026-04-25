# Staging release

## Prerequisites (secrets)

Configure the **staging** GitHub Environment and/or the server’s `deployments/staging/.env.staging` (never commit real values):

- `STAGING_DATABASE_URL` (or set `DATABASE_URL` from the same value)
- `STAGING_REDIS_URL`, `STAGING_NATS_URL`, `STAGING_MQTT_BROKER_URL`
- PSP and webhook **sandbox** keys only
- `APP_ENV=staging`, `PAYMENT_ENV=sandbox`, `PUBLIC_BASE_URL=https://staging-api.ldtv.dev`, `MQTT_TOPIC_PREFIX=avf-staging/devices`

## Deploy

The canonical server-side script is `deployments/staging/scripts/deploy_staging.sh` (used by the staging deploy workflow). It:

1. Validates compose.
2. Pulls digest-pinned `APP_IMAGE_REF` / `GOOSE_IMAGE_REF`.
3. Starts postgres/nats/emqx.
4. Runs **`scripts/verify_database_environment.sh`** (sourced from `.env.staging`) before Goose migrations.
5. Boots the app stack and optional smoke/health (unless `SKIP_SMOKE=1` on the agent).

## Migration safety

- The guard enforces `APP_ENV=staging`, rejects localhost unless `STAGING_ALLOW_LOCAL_DATABASE=true`, and rejects mixing staging DSNs with `PRODUCTION_DATABASE_URL` when both are set.
- Never point staging at the production DSN. Go config validation also enforces this when both URLs are in the process environment.

## Smoke tests

```bash
STAGING_BASE_URL=https://staging-api.ldtv.dev bash scripts/smoke_staging.sh
```

On the server, `deployments/staging/scripts/smoke_staging.sh` resolves `API_DOMAIN` from `.env.staging` and calls the same repository script.

## Rollback

Use your existing image ref rollback and compose procedures (see deployment evidence under `deployments/staging/.deploy` if your install records tags). The workflow uploads a `staging-deployment-verdict` artifact; treat failed deploys as “fix forward or re-run with last-known-good image refs per org policy”.

## Telemetry “storm” pre-scale

Before authorizing high fleet targets in production, run the staging storm suite to the **minimum dimension** for the tier (e.g. 100×100) — see [telemetry-production-rollout.md](./telemetry-production-rollout.md) and [`.github/workflows/telemetry-storm-staging.yml`](../../.github/workflows/telemetry-storm-staging.yml).

## Related

- [environment-separation-gates workflow](../../.github/workflows/environment-separation-gates.yml) — optional manual contract check with staging + production secrets.
- [environment-strategy.md](./environment-strategy.md)
