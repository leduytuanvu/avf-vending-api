# Deployment layout by environment

| Environment | Branch / automation | Path / artifact | Config on server |
|-------------|--------------------|----------------|------------------|
| **Local** | — | `deployments/docker/docker-compose.yml` | Copy [`.env.local.example`](../../.env.local.example) to `.env` locally (gitignored) |
| **Develop** | Branch **`develop`** only — **no dedicated VPS**. Staging tooling runs via **Staging Deployment Contract** after Security Release; **`vars.ENABLE_REAL_STAGING_DEPLOY`** gates real SSH to pre-prod. | Does not provision a separate topology from staging | Not applicable as its own fleet — use staging/pre-prod or local |
| **Staging** | `develop` chain | `deployments/staging/**` (compose, Caddy, EMQX, scripts) | `deployments/staging/.env.staging` — see [deployments staging example](../../deployments/staging/.env.staging.example) and repo [`.env.staging.example`](../../.env.staging.example) |
| **Production** (**primary**: two **`app-node`** VPS rolling + optional **`data-node`**, or managed backends) | Manual **Deploy Production** on **`main`** | `deployments/prod/**` (`app-node/`, `data-node/`, `shared/`); legacy `docker-compose.prod.yml` is non-primary | `deployments/prod/*.env*` (never committed); see `*.example` under `deployments/prod/` |

Narrative runbooks: [environment-strategy.md](../runbooks/environment-strategy.md), [local-dev.md](../runbooks/local-dev.md), [staging-release.md](../runbooks/staging-release.md).

**Production public images** under `deployments/prod/**` (Dockerfile `FROM` lines and `docker-compose*.yml` `image:` for third-party services) are **digest-pinned** (`@sha256:…`); application and goose images stay `${APP_IMAGE_REF}` / `${GOOSE_IMAGE_REF}` for digest-pinned promotion. To change a public tag, resolve the new multi-arch **index** digest with `docker buildx imagetools inspect <image:tag>` and update the file (CI enforces the pin in `tools/verify_github_workflow_cicd_contract.py`).

## Production fail-fast configuration

At `APP_ENV=production`, `internal/config` rejects unsafe combinations before the process accepts traffic: `PAYMENT_ENV=live` with a non-sandbox `COMMERCE_PAYMENT_PROVIDER`, TLS-oriented public MQTT (or explicit `MQTT_TLS_ENABLED` when the URL scheme is ambiguous), `REDIS_ADDR` / `REDIS_URL` unless `PRODUCTION_ALLOW_MISSING_REDIS=true`, mandatory `NATS_URL` when NATS/outbox flags default on, object storage fields when artifacts/media storage is enabled, production JWT modes and secrets, and legacy machine HTTP only with `MACHINE_REST_LEGACY_ALLOW_IN_PRODUCTION=true`. Operators should mirror the checked keys from [`.env.production.example`](../../.env.production.example) and [`deployments/prod/**/.env*.example`](../../deployments/prod/). Full secret names and rotation notes: [`docs/operations/deployment-secrets.md`](../operations/deployment-secrets.md).
