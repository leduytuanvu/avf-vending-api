# Production rollback (incident)

Use this when you need a **controlled, digest-pinned rollback** in production, with **reviewers** and an **evidence trail**.

## Two mechanisms

1. **Evidence + validation (this repo):** `.github/workflows/rollback-prod.yml` — **Rollback (Production — Incident)** — `workflow_dispatch` only, **`environment: production`**, uploads **`production-rollback-evidence`**. It **does not SSH**; it resolves a prior manifest or explicit `ghcr.io/...@sha256:...` app/goose refs and validates them **before** any operator action that touches hosts.
2. **Execution:** `.github/workflows/deploy-prod.yml` — **Deploy Production** with **`action_mode: rollback`**, `rollback_app_image_ref`, and `rollback_goose_image_ref` set to the **same** digest-pinned refs you validated, **or** on-host scripts (e.g. `deployments/prod/app-node/scripts/rollback_app_node.sh` in your topology).

Legacy single-host reference: [deployments/prod/scripts/rollback_prod.sh](../deployments/prod/scripts/rollback_prod.sh) (requires `ALLOW_LEGACY_SINGLE_HOST=1`).

## When to use the incident workflow

- You need a **dedicated incident record** (ticket id, reason, `dry_run` preflight) and a JSON artifact for auditors.
- You already have a **previous successful deploy** with artifact **`production-deployment-manifest`**, *or* you know the **exact** previous app and goose image digests to roll back to.

## Inputs (Rollback incident workflow)

| Input | Rule |
| --- | --- |
| `previous_deployment_manifest_id` | Optional. Digits-only **run id** of a **successful** **Deploy Production** run that published **`production-deployment-manifest`**. If set, app/goose refs are read from the manifest JSON. |
| `rollback_target_app_image_ref` / `rollback_target_goose_image_ref` | Required if manifest id is empty. Must be `...@sha256:<64-hex>`, no `:latest`. |
| `incident_id` / `reason` | Required, non-secret operator context. |
| `dry_run` | Default `true`. Validates and uploads evidence; does not imply SSH or compose changes. |

## After preflight (not dry run)

1. Get **Environment** approval (configured on the `production` environment in **Settings → Environments**).
2. Run **Deploy Production** in **rollback** mode with the validated digests, or run the on-host rollback path you use for 2-VPS.
3. Attach artifact **`production-rollback-evidence`** to the incident ticket.

## What not to do

- Do not add `on.push` / `on.workflow_run` to the rollback incident workflow (CI enforces this).
- Do not use staging credentials or secrets for production rollback.

See also: [production-backup-restore-drill.md](production-backup-restore-drill.md), [../operations/two-vps-rolling-production-deploy.md](../operations/two-vps-rolling-production-deploy.md).
