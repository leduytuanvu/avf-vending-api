# Staging / pre-prod release evidence gate

Enterprise production deploys on two VPS nodes require **machine-readable evidence** from a **real** staging or pre-prod stack before operators may run **Deploy Production**. Contract-only Staging runs (no remote host) are **not** sufficient for `promotion_eligible: true`.

## Required configuration for real staging

Set repository variable `ENABLE_REAL_STAGING_DEPLOY` to `true` and configure the following. Names accept either a `STAGING_*` or `PREPROD_*` prefix (see workflow `validate-staging-prerequisites`).

| Variable / secret | Purpose |
| --- | --- |
| `STAGING_HOST` or `PREPROD_HOST` | Staging host hostname or IP |
| `STAGING_SSH_USER` or `PREPROD_SSH_USER` | SSH user for deploy |
| `STAGING_SSH_KEY` or `PREPROD_SSH_KEY` | Private key material for SSH (secret) |
| `STAGING_DEPLOY_ROOT` or `PREPROD_DEPLOY_ROOT` | Remote path for compose / deploy layout |
| `STAGING_REMOTE_ENV_FILE` or `PREPROD_REMOTE_ENV_FILE` | Path to the remote env file used by the stack (no default; must be set explicitly) |
| `STAGING_SSH_KNOWN_HOSTS` (or preprod alias) | `ssh-keyscan` line(s) for host key pinning |

If real staging is enabled but any required value is missing, the **Staging Deployment Contract** workflow **fails fast** instead of succeeding with a no-op.

## How to run staging

1. Complete **Build and Push Images** and **Security Release** on the normal release chain; **Staging Deployment Contract** runs after a successful Security Release on `develop`.
2. With `ENABLE_REAL_STAGING_DEPLOY=true` and secrets present, the workflow performs a **real_staging** remote deploy, health checks, and smoke, then uploads artifacts including **`staging-deploy-evidence`**.

## Staging evidence artifact

| Artifact name | File path inside zip | Use |
| --- | --- | --- |
| `staging-deploy-evidence` | `staging-evidence/staging-deploy-evidence.json` | **Canonical** input for production promotion gating |
| `staging-release-evidence` | `deployment-evidence/staging-release-evidence.json` | Legacy / supplemental fields |

## How to find the staging evidence run id

1. Open **Actions** → **Staging Deployment Contract** (workflow from `deploy-develop.yml`).
2. Open the run that completed **successfully** for the same images you will deploy to production.
3. Copy the run id from the URL:  
   `https://github.com/<org>/<repo>/actions/runs/<RUN_ID>`

That numeric **`<RUN_ID>`** is the value for **`staging_evidence_id`** (same meaning as *staging evidence run id* / *staging_evidence_run_id* in runbooks).

## Using evidence in Deploy Production

Run **Deploy Production** from the **main** branch (manual **workflow_dispatch**). For `action_mode=deploy`:

- Set **`staging_evidence_id`** to the Staging run id from the previous section.
- Ensure **`app_image_ref`** and **`goose_image_ref`** digests **match** the evidence file (same `sha256` as recorded on staging).
- Optional: adjust **`staging_evidence_max_age_hours`** (default **168**) if evidence must be newer than one week.

The workflow **downloads** the `staging-deploy-evidence` artifact from that run and **validates JSON before any production SSH**. If validation fails, the workflow stops with an error (no production rollout).

### Temporary bypass

**`allow_missing_staging_evidence`** (default `false`) may be set to `true` only with a non-empty **`missing_staging_evidence_reason`**. This **skips** evidence download; use only when the staging gate is temporarily unavailable, and remove the bypass when pre-prod is back.

## Interpreting `promotion_eligible`

In `staging-evidence/staging-deploy-evidence.json`:

| `promotion_eligible` | Meaning |
| --- | --- |
| `true` | **real_staging** path: remote deploy attempted and succeeded, health and smoke **success**, `deployment_mode` is `real_staging`, `host_count` ≥ 1. **Deploy Production** can accept this (subject to repository match, digest match, and freshness). |
| `false` | **contract_only** (staging disabled) or **real_staging** that did not pass the full gate. **`reason` / `promotion_ineligible_reason`** explains why. **Do not** use for production unless using the explicit bypass above. |

Production validation additionally requires `schema_version: staging-deploy-evidence-v1`, matching **`repository`**, `promotion_eligible: true`, `deployment_mode: real_staging`, `smoke_result` and `readiness_result` of `success`, and `deployed_at_utc` within the configured max age.

## Related runbooks

- `docs/runbooks/staging-preprod.md` (if present) — operator procedures for the staging host and compose layout.
