# Staging / pre-prod (real VPS) setup

The workflow **Staging Deployment Contract** (`.github/workflows/deploy-develop.yml`) has two modes:

- **Contract-only** (`vars.ENABLE_REAL_STAGING_DEPLOY` is not `true`): validates the release chain and digest-pinned image refs. The job summary and artifact `staging-contract-outcome` state **`contract-only, not a real staging deployment`** — the app is **not** deployed.
- **Real pre-prod** (`vars.ENABLE_REAL_STAGING_DEPLOY` is `true`): requires the items below, then runs remote `deployments/staging/scripts/deploy_staging.sh` (including `docker compose config`, digest-pinned `docker compose pull`), `healthcheck_staging.sh` (`/health/ready`), and smoke checks. Evidence: `staging-deployment-verdict`, `staging-preprod-compose-runner-evidence`, `staging-smoke-evidence`, etc.

Production remains **manual-only** via `Deploy Production` (`deploy-prod.yml`); this document is only for staging/pre-prod.

## Checklist

### Host and DNS

- A dedicated Linux VPS (or equivalent) with a static IP or stable DNS name used as **`STAGING_HOST`** (or `PREPROD_HOST` alias in secrets/vars as wired in the workflow).
- **DNS**: Point your staging API hostname (e.g. `api.staging.example.com`) to that host if you use HTTPS smoke tests or public health URLs.
- **Firewall**: Open SSH (`STAGING_SSH_PORT`, default 22) from GitHub Actions egress only if you can restrict by source; otherwise use a strong key and `STAGING_SSH_KNOWN_HOSTS` host key pinning.
- **Deploy root** directory on the host (e.g. `/opt/avf-staging`) matching **`STAGING_DEPLOY_ROOT`**.

### SSH key and trust

- Generate an **ed25519** (or approved) key pair for **deploy only**; store the private key in **`STAGING_SSH_KEY`** (or alias secrets listed in the workflow).
- Add the public key to `authorized_keys` for **`STAGING_SSH_USER`** on the host.
- Capture **`ssh-keyscan -p <port> <host>`** output for the host and store the **full known_hosts line(s)** in **`STAGING_SSH_KNOWN_HOSTS`** (or `PREPROD_*` aliases). The workflow enforces `StrictHostKeyChecking`.

### GitHub secrets and variables

Set at least the following (see workflow for full secret name fallbacks `STAGING_*` / `PREPROD_*`):

| Item | Purpose |
|------|--------|
| `ENABLE_REAL_STAGING_DEPLOY` | Repository variable: must be `true` to run the real path. |
| `STAGING_HOST` / `STAGING_DEPLOY_ROOT` | Where to connect and where assets are extracted. |
| `STAGING_SSH_KEY` (+ optional port/user) | Non-interactive SSH. |
| `STAGING_SSH_KNOWN_HOSTS` | Pinned host keys. |
| `STAGING_REMOTE_ENV_FILE` | Path to the remote `.env` for staging (default under deploy root). |
| Smoke / public URL | e.g. `STAGING_SMOKE_BASE_URL` or `STAGING_PUBLIC_BASE_URL` / `STAGING_API_READY_URL` for runner smoke after deploy. |

Optional: `NOTIFY_WEBHOOK_URL` for deployment notifications (staging is not production).

### Environment variables (remote)

- Maintain a real **`deployments/staging/.env.staging`** (or path in `STAGING_REMOTE_ENV_FILE`) on the host with no `CHANGE_ME` / `REPLACE_ME` / placeholder hostnames. The workflow **fails** if placeholders remain.
- Align DB URL, API keys, and feature flags with a **non-production** data set.

### Database

- Provision a **staging** database (isolated from production). Point `DATABASE_URL` (or your app’s DSN) in the remote env file to that instance.
- Run migrations as defined for staging in `deploy_staging.sh` / project docs. Review destructive migration policy (`ALLOW_DESTRUCTIVE_MIGRATIONS`) if applicable.

### Redis, EMQX, NATS, and other dependencies

- If `docker-compose.staging.yml` includes **Redis**, **EMQX**, **NATS**, or others, ensure each service is reachable from the app container on the staging network; set credentials and hostnames in the remote env (not example placeholders).
- Confirm ports are only exposed as needed (often internal Docker network + reverse proxy for HTTPS).

### Health checks

- The API must respond on **`/health/ready`** as exercised by `deployments/staging/scripts/healthcheck_staging.sh` (container and/or public URL depending on script).
- Fix failures before considering the staging run successful; the workflow records `healthcheck_outcome` in `staging-deployment-verdict.json`.

### Smoke tests

- Configure **`STAGING_SMOKE_BASE_URL`** (or `STAGING_PUBLIC_BASE_URL` / `STAGING_API_READY_URL`) so `scripts/deploy/staging_smoke_evidence.sh` can run from the GitHub runner after deploy.
- Optional authenticated smoke: `STAGING_SMOKE_AUTH_TOKEN` and `STAGING_SMOKE_AUTH_READ_PATH` as used by the smoke script.

## CI / evidence artifacts (real mode)

- **Runner**: `docker compose … config` with `deployments/staging/.env.staging.example` (artifact `staging-preprod-compose-runner-evidence` — resolved compose SHA).
- **Host**: `deploy_staging.sh` runs compose config, digest-pinned pull, and stack up.
- **Post-deploy**: smoke JSON/MD under `smoke-reports/`, uploaded as `staging-smoke-evidence`.

## Related files

- `.github/workflows/deploy-develop.yml` — staging / develop contract and deploy jobs.
- `deployments/staging/scripts/deploy_staging.sh` — remote orchestration.
- `deployments/staging/docker-compose.staging.yml` — staging stack definition.
- [Deployment runbook: staging → production](deployment-staging-to-prod-gate.md) — collect `staging_evidence_id` and run **Deploy Production** with strict image-digest matching.
