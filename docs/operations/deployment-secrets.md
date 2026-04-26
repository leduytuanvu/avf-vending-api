# Deployment secrets and configuration

This document explains how **GitHub Actions secrets/variables** relate to **on-host** configuration for staging and production, and how the machine-readable contract enforces a complete inventory.

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
