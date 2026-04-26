# Deploy failure triage (staging and production)

**Context:** [cicd-release.md](./cicd-release.md) (end-to-end flow).

This runbook assumes **Security Release** already produced a **`verdict=pass`** (or focuses on deploy-side failures). For gate failures **before** deploy authorization, see [security-release-failure.md](./security-release-failure.md).

## Staging (`deploy-develop.yml`)

**Workflow display name:** Staging Deployment Contract. **File:** `deploy-develop.yml` only (there is no `deploy-staging.yml`; job id `deploy-staging` is internal to this file).

1. Confirm the trigger was **Security Release** **`completed`** on branch **`develop`** (not repo **Security** or **Build**).
2. Check **resolve-staging-candidate** logs and **`validate_release_verdict.py`** output — non-pass verdicts **fail closed**.
3. If **`ENABLE_REAL_STAGING_DEPLOY`** is not enabled, the workflow may validate only; see workflow summary for **skipped** real deploy messaging.
4. Inspect **`staging-deployment-verdict`** (or equivalent) artifacts if uploaded; SSH/sync logs on the runner for **migration_preflight** and deploy scripts.

## Production (`deploy-prod.yml`)

**Workflow display name:** Deploy Production. **Canonical file:** `deploy-prod.yml` — **not** `deploy-production.yml` (pointer only).

1. Open the run **Summary** → **Release dashboard** (source SHA, branch, digests, security sub-verdicts, smoke rollup, rollback hint, deploy status).
2. **Automatic rollback:** if the workflow started app rollout and failure modes match policy, **deploy-prod** may invoke automatic **image-only** rollback when last-known-good digest-pinned refs exist — **no `goose down`**.
3. **Manual rollback:** use **`workflow_dispatch`** with **`action_mode=rollback`** and digest-pinned refs from **Resolve Previous Production Deployment** / **`production-deployment-manifest`** — see [rollback-production.md](./rollback-production.md).
4. **Migration issues:** migration preflight and first-node **`RUN_MIGRATION`** can fail the job; **rollback does not reverse schema**. See [migration-safety.md](./migration-safety.md).

## Optional notifications

Set repository variable **`NOTIFY_WEBHOOK_URL`** to a generic incoming webhook (Slack, Teams, etc.); **do not** commit the URL. **`scripts/deploy/notify_deployment_status.sh`** sends a small JSON body with **`status`**, **`environment`**, **`source_sha`**, **`source_branch`**, optional **`workflow_run_url`**, and **`message`**. The script **never logs the URL** or payload secrets.

| Workflow | When |
| --- | --- |
| **Security Release** | Only when the **Security Release Signal** job ends **`failure`** (separate `optional-notify-on-security-release-failure` job). |
| **Staging Deployment Contract** (`deploy-develop.yml`, job **Execute Staging Deployment**) | **`if: always()`** on that job — **success** / **failure** / **cancelled** (same as GitHub’s job `status`). |
| **Deploy Production** | **`if: always()`** on the **Deploy production release** job — both **`action_mode=deploy`** and **`action_mode=rollback`** (overall job result includes automatic rollback paths). |

If **`NOTIFY_WEBHOOK_URL`** is unset, the script prints **`notification skipped: NOTIFY_WEBHOOK_URL not configured`** and exits **0**. Webhook or **`curl` errors do not change the deploy workflow result** (notification is best-effort only).

## Related

- [rollback-production.md](./rollback-production.md) — LKG selection and manual rollback.  
- [staging-release.md](./staging-release.md) — staging server and secrets.  
- [post-deploy-smoke-tests.md](./post-deploy-smoke-tests.md) — smoke expectations.  
- [canary-rollout.md](./canary-rollout.md) — host-order canary behavior.
