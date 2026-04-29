# Deployment secrets and configuration

This document explains how **GitHub Actions secrets/variables** relate to **on-host** configuration for staging and production, and how the machine-readable contract enforces a complete inventory.

## Runtime secret matrix (DATABASE / Redis / NATS / MQTT / object storage / JWT / payment)

| Binding | Where it lives | Required? | GitHub Actions |
| --- | --- | --- | --- |
| `DATABASE_URL` | Staging or production **host `.env`** (path from `STAGING_REMOTE_ENV_FILE` / app-node example) | Required for app + migrations | **Never** as `secrets.DATABASE_URL` in deploy workflows — use `*.example` on-disk. Optional `PRODUCTION_DATABASE_URL` / `STAGING_DATABASE_URL` appear only in separation-gate tooling for fingerprint checks, **not** app SSH injection from `deploy-prod`. |
| `REDIS_URL` / `REDIS_ADDR` | Host `.env` | **Required** in `APP_ENV=production` unless `PRODUCTION_ALLOW_MISSING_REDIS=true`; otherwise as enabled by Redis feature flags | **Not** modeled as vanilla `secrets.REDIS_*` operator keys — keep on VPS. |
| `NATS_URL` | App nodes + optional data-node fallback | Required when telemetry/outbox pipelines are active | VPS only; distinguish staging vs prod cluster addresses. |
| `MQTT_BROKER_URL`, EMQX bootstrap (`MQTT_*`, `EMQX_*`) | Host `.env`; production TLS posture `8883` | Required for machine plane where enabled | VPS only; staging `MQTT_TOPIC_PREFIX` must differ from prod (`avf/staging/devices` vs `avf/devices`). |
| `OBJECT_STORAGE_*` (bucket, endpoint, keys, region) | Host `.env`; aliases `AWS_*`, `S3_*` documented in app-node example | Required when artifacts/media integrations are on | VPS only — same rule as DB: no raw operator key **names** in `deploy-develop` / `deploy-prod`. |
| `HTTP_AUTH_JWT_SECRET` | Host `.env` | Required (`APP_ENV=staging` or production) | VPS-only material. |
| Payment provider / PSP secrets (`PAYMENT_*`, webhook signing) | Host `.env` | Staging → **sandbox**; production → **live** | `PAYMENT_ENV` selects mode; staging PSP credentials must never be reused as production secrets. |

### VPS `.env` inventory (name, scope, required, format, rotation)

| Name | Environment | Required | Example format | Rotation |
| --- | --- | --- | --- | --- |
| `DATABASE_URL` | Staging + production host `.env` | Yes | `postgres://USER:PASSWORD@HOST:5432/DB?sslmode=require` | Rotate DB password at provider; update all app nodes; verify pool limits. |
| `REDIS_URL` or `REDIS_ADDR` | Staging + production | Production **yes** (unless `PRODUCTION_ALLOW_MISSING_REDIS=true`) | `rediss://default:PASSWORD@HOST:6379/0` or `HOST:6379` | Rolling credential rotation at cache provider; brief cache flush acceptable. |
| `NATS_URL` | Staging + production app/worker profiles | Yes when outbox/telemetry flags on | `nats://HOST:4222` | Cluster credential / TLS cert rotation per NATS ops runbook. |
| `MQTT_BROKER_URL` | Staging + production | Yes when device plane uses MQTT | Production public: `tls://HOST:8883`, `ssl://`, `mqtts://`, or `wss://` (not `tcp`/`mqtt` to non-loopback without TLS) | Broker auth password rotation in EMQX; update all clients. |
| `MQTT_USERNAME` / `MQTT_PASSWORD` | Staging + production | Yes for non-anonymous production brokers | User / password from EMQX | Rotate MQTT app password; roll clients. |
| `MQTT_TOPIC_PREFIX` | Staging + production | Yes when MQTT used | Staging: e.g. `avf/staging/devices`; production: `avf/devices` | Avoid changing in-place; plan fleet cutover. |
| `OBJECT_STORAGE_BUCKET` | Production when `API_ARTIFACTS_ENABLED` / object storage on | Conditional | DNS-safe bucket name | N/A (name); keys rotated separately. |
| `OBJECT_STORAGE_ENDPOINT` / `OBJECT_STORAGE_REGION` | With object storage | Conditional | `https://s3.example` / `ap-southeast-1` | Follow object storage vendor rotation. |
| `OBJECT_STORAGE_ACCESS_KEY` / `OBJECT_STORAGE_SECRET_KEY` | With object storage | Conditional | Vendor-specific access key id + secret | IAM-style key rotation with dual-sign period. |
| `OBJECT_STORAGE_PUBLIC_BASE_URL` | Production artifacts on | Conditional | `https://cdn.example/` base for public object URLs | When CDN or bucket URL changes, update and purge caches. |
| `HTTP_AUTH_JWT_SECRET` | Staging + production | Yes | ≥32 bytes random (HS256) | Staged rotation with `HTTP_AUTH_JWT_SECRET_PREVIOUS` if configured; force re-login. |
| `HTTP_AUTH_MODE` / `USER_JWT_MODE` | Production | Yes (explicit) | `hs256` or enterprise JWKS modes | Mode change is a rollout event (issuer/JWKS coordination). |
| `MACHINE_JWT_SECRET` | Staging + production with machine gRPC | Yes when `MACHINE_JWT_MODE=hs256` | ≥32 bytes random, distinct from admin secret | Rotate with kiosk/firmware update window; use `MACHINE_JWT_SECRET_PREVIOUS` if supported. |
| `MACHINE_JWT_MODE` | Production | Yes (explicit) | `hs256`, `ed25519`, etc. | Align with vending app build. |
| `AUTH_ISSUER` / `MACHINE_JWT_ISSUER` | Production HS256 | Yes | HTTPS issuer URL string | Must match token `iss`; coordinate with all issuers. |
| `COMMERCE_PAYMENT_WEBHOOK_SECRET` / `COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET` / `PAYMENT_WEBHOOK_SECRET` | Staging + production | Yes (signed webhooks) | Long random string | Coordinate with PSP dashboard secret rotation; support dual secret via JSON map if used. |
| `COMMERCE_PAYMENT_PROVIDER` | Staging + production | Production **yes** | Live key: `stripe`, `vnpay`, …; staging may use `sandbox` | N/A; changing key switches adapter behavior — coordinate PSP. |
| `PAYMENT_ENV` | Staging + production | Yes | Staging: `sandbox`; production: `live` | Environment flag only; must match PSP credentials. |
| `PAYMENT_*` PSP credentials | Host only | Per integration | PSP-specific | Per PSP security guidance; never reuse staging keys in prod. |
| `GHCR_PULL_TOKEN` / `GHCR_PULL_USERNAME` | VPS (optional) | If GHCR images private | PAT or read token + username | Rotate PAT; update on all nodes. |
| `GRPC_TLS_CERT_FILE` / `GRPC_TLS_KEY_FILE` | Optional on-node TLS | If not behind TLS proxy | Filesystem paths to PEM | Cert renewal before expiry; reload instances. |

**GitHub Actions** inventory (SSH, deploy paths, smoke tokens) remains in [`docs/contracts/deployment-secrets-contract.yml`](../contracts/deployment-secrets-contract.yml). Production workflow jobs that touch SSH use **`environment: production`** — configure secrets there so staging-only keys are not exposed to production deploy approvals.

**Develop “environment”**: there is **no** dedicated develop VPS today (`DEVELOP_HOST` is not part of this contract). Branch **`develop`** drives **staging** automation (`Staging Deployment Contract` workflow after Security Release); real staging SSH/deploy is gated by **`ENABLE_REAL_STAGING_DEPLOY`** / **`ALLOW_STAGING_CONTRACT_ONLY`** (see **`deploy-develop.yml`**). Production uses **`deploy-prod.yml`** on **`main`** with the protected **`production`** environment.

**Rollback** workflow `rollback-prod.yml` uses **`environment: production`** and digest pins only — it **must not** use staging SSH/DB **`STAGING_*` / `PREPROD_*`**.

Structured machine-readable mirror: **`docs/contracts/deployment-secrets-contract.yml`** (`runtime_binding_matrix`).

## Contract and automation

- **Authoritative list:** `docs/contracts/deployment-secrets-contract.yml` — all `secrets.*` and `vars.*` names used in:
  - `.github/workflows/deploy-develop.yml` (staging / pre-prod on `develop`)
  - `.github/workflows/deploy-prod.yml` (production, `main`, manual)
- **Verifier:** `scripts/ci/verify_deployment_config_contract.py` (runs in CI with `scripts/ci/verify_workflow_contracts.sh`).

The verifier fails if a workflow adds a new secret/variable name that is not documented in the contract, or if a workflow misuses a name that is reserved for the remote `.env` (for example `DATABASE_URL` as a **GitHub** secret — database URLs should live on the server). It never prints secret **values**; it only checks **identifiers**.

## Repository secrets vs environment secrets

| Mechanism | Where to configure (GitHub UI) | Typical use in this repo |
| --- | --- | --- |
| **Repository** secrets and variables | **Settings → Secrets and variables → Actions** (tabs **Secrets** / **Variables**) | Staging and shared fallbacks. `deploy-develop` does not attach a named GitHub *Environment*; it uses org/repo-level `vars` / `secrets` (for example `ENABLE_REAL_STAGING_DEPLOY`, `STAGING_SSH_*`). |
| **Environment** secrets and variables | **Settings → Environments →** select environment (for example `production`) → **Environment secrets** / **Environment variables** | Production: prefer production-only credentials here so they are not visible to workflows that are not environment-gated. The production workflow uses `environment: production` on deployment jobs. |

**Production secrets must be scoped to the `production` environment** when the credential is only needed for production (SSH keys, production-only smoke tokens, etc.). That limits exposure to runs that are approved for that environment.

**Staging secrets must not be reused for production** unless the team explicitly documents the exception (one key pair, one webhook URL, etc.) and the risk is accepted. The normal pattern is: **separate** repository secrets or environment-scoped values with a `STAGING_` / `PRODUCTION_` (or `PREPROD_` / `PROD_`) prefix, as reflected in the workflow `||` chains in `docs/contracts/deployment-secrets-contract.yml`.

## Exact UI path to configure secrets and variables

1. Open the repository on **GitHub.com**.
2. **Settings** (repo settings, not your profile).
3. **Secrets and variables** → **Actions**.
4. **Secrets** or **Variables**:
   - **New repository secret** / **New repository variable** for repo-wide values used by `deploy-develop` and by shared fallbacks in `deploy-prod` where the workflow reads repository-level names.
5. For **Environment**-scoped values:
   - **Settings** → **Environments** → **production** (create it if missing).
   - Add **Environment secrets** and **Environment variables** used only when the workflow runs with `environment: production`.

`GITHUB_TOKEN` is **not** a repository secret: GitHub injects it automatically. Workflows that pull from GHCR or call `gh` use it with the permissions set on the job (`permissions` block in the workflow file).

## `known_hosts` and SSH (staging and production)

Deploy workflows do **not** use `ssh-keyscan` at run time. They require a **pre-provisioned** `known_hosts` (or `known_hosts`-style) payload:

- In workflows this is `SSH_KNOWN_HOSTS_CONTENT` (or the staging/preprod / production variants), which is written to `~/.ssh/known_hosts` on the runner.
- The host key material should be the **host keys you expect** for your staging and production jump/VPS (typically from `ssh-keyscan -H host` run **once** in a safe admin context, then stored as a **secret**).

**Strict host checking** is enabled in the deploy scripts (see workflow `run` steps that set `StrictHostKeyChecking=yes`). Rotating a server host key requires updating the `known_hosts` secret and redeploying.

**SSH user, key, and port** are also supplied from secrets/variables (`STAGING_SSH_USER`, `PRODUCTION_SSH_USER`, `PRODUCTION_SSH_PRIVATE_KEY`, and alias names listed in the contract). The **on-host** `STAGING_REMOTE_ENV_FILE` / production `.env` paths are *paths on the remote machine*, not repository paths.

## Database URL and application secrets (on the host)

The **authoritative** place for `DATABASE_URL`, `POSTGRES_*`, `HTTP_AUTH_JWT_SECRET`, `PAYMENT_*`, and similar values is the **remote environment file** on the VPS, as documented in:

- `deployments/staging/.env.staging.example`
- `deployments/prod/.env.production.example` and node-specific `deployments/prod/*/.env*.example`

Copy the example, replace every placeholder, and **do not** commit the real file. The GitHub workflow only needs to know the **path** to that file on the server (e.g. `STAGING_REMOTE_ENV_FILE` for staging).

## Webhooks, monitoring, and payment-related keys

- **Notify / webhook URL:** `NOTIFY_WEBHOOK_URL` is a **variable** in `deploy-develop` (not a default secret name in the example `.env` files). If you add monitoring or PagerDuty URLs, add them to the same inventory pattern and update `deployment-secrets-contract.yml` if you introduce a new `vars.*` or `secrets.*` in the deploy workflows.
- **Payment:** Staging and production examples use `PAYMENT_ENV`, outbox settings, and optional `PAYMENT_WEBHOOK_SECRET` in **`.env` on the host**. These are listed under `vps_env_operator_keys` in the contract so they **cannot** be confused with GitHub Actions secret names in workflows.

## Secret rotation checklist (operators)

1. **Identify scope:** repository vs `production` environment; staging must not share production’s keys unless accepted.
2. **Generate a new value** (SSH key, token, webhook signing secret) in a secure channel; **do not** paste into chat or tickets.
3. **Update the GitHub secret/variable** (or remote `.env` for host-only material) in the correct **Settings** location above.
4. **Update `known_hosts`** if the server SSH host key changed.
5. **Re-run a controlled workflow** (staging deploy, or small production action) and confirm the job that consumes the secret still passes SSH / smoke / health checks.
6. **Remove or disable** the old key/token at the provider (SSH `authorized_keys`, GitHub **Deploy key** or PAT revocation, etc.).
7. **Update the machine-readable contract** if you add a new workflow identifier: `docs/contracts/deployment-secrets-contract.yml` and open a PR; CI will run `python scripts/ci/verify_deployment_config_contract.py`.

## Local checks

```bash
python3 scripts/ci/verify_deployment_config_contract.py
```

See also: `docs/contracts/deployment-secrets-contract.yml`, `docs/runbooks/staging-preprod.md`, and production runbooks under `docs/operations/`.
