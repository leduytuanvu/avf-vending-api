# Staging release

## Canonical GitHub workflow

- **File:** [`.github/workflows/deploy-develop.yml`](../../.github/workflows/deploy-develop.yml) (display name **Staging Deployment Contract**).
- **Trigger:** successful **Security Release** `workflow_run` on branch **`develop`** only — not **Build**, not repo **Security** (`security.yml`).
- There is **no** separate `deploy-staging.yml`; the internal job id `deploy-staging` is part of the above workflow only. Adding a parallel staging deploy workflow would violate the enterprise contract.

### Common failure modes

- **Security Release** on `develop` did not **`conclusion: success`** → blocking job **fail-when-security-release-not-successful** runs; no staging deploy.
- **`verdict` not `pass`** in **`security-verdict`** → **`validate_release_verdict.py`** fails closed; see [security-release-failure.md](./security-release-failure.md).
- **`source_branch` not `develop`** or **automatic staging** requires **`source_event: push`** → candidate rejected by policy.
- **`ENABLE_REAL_STAGING_DEPLOY`** false → validation-only path; see workflow summary.

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

### CI evidence after staging deploy (GitHub Actions)

When **`vars.ENABLE_REAL_STAGING_DEPLOY == 'true'`**, **Deploy** → **Execute Staging Deployment** runs **`scripts/deploy/staging_smoke_evidence.sh`** after remote deploy + health. That script:

- Runs **`tools/smoke_test.py`** (required: **`/health/live`**, **`/health/ready`**, **`/version`**; optional: DB/Redis/MQTT/heartbeat/payment-mock/admin GET when env/vars are set — otherwise **`skip`** with a reason, not **pass**).
- Writes **`staging-smoke-evidence`** (JSON + `staging-smoke-summary.md`) and a **pass / fail / skip** table in the job summary.
- A failed required check **fails the staging job**. Configure **`STAGING_SMOKE_BASE_URL`** (or public base / `STAGING_API_READY_URL` origin) so the runner can reach the API over HTTPS.

Production release reviews may reference this artifact; production still requires **Security Release on `main`** and **`deploy-prod.yml`** — there is no automatic promotion from develop smoke alone.

## Rollback

Use your existing image ref rollback and compose procedures (see deployment evidence under `deployments/staging/.deploy` if your install records tags). The workflow uploads a `staging-deployment-verdict` artifact; treat failed deploys as “fix forward or re-run with last-known-good image refs per org policy”.

## Telemetry “storm” pre-scale

Before authorizing high fleet targets in production, run the staging storm suite to the **minimum dimension** for the tier (e.g. 100×100) — see [telemetry-production-rollout.md](./telemetry-production-rollout.md) and [`.github/workflows/telemetry-storm-staging.yml`](../../.github/workflows/telemetry-storm-staging.yml).

## Related

- [deploy-failure.md](./deploy-failure.md) — staging deploy triage on the runner.
- [environment-separation-gates workflow](../../.github/workflows/environment-separation-gates.yml) — optional manual contract check with staging + production secrets.
- [environment-strategy.md](./environment-strategy.md)
