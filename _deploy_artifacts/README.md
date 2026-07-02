# Production deploy candidate

This bundle is produced by **Security Release** after **verdict=pass** on **`main`** only.
It does **not** deploy production and contains **no secrets**.

> **Replace TODO_STAGING_EVIDENCE_RUN_ID with a successful Staging Deployment Contract run id before production deploy. Do not run production deploy with this placeholder.**

## Files

| File | Purpose |
|------|---------|
| `production-deploy-inputs.json` | `workflow_dispatch` inputs for **Deploy Production** (`deploy-prod.yml`). Safe to pass to `gh workflow run ... --json`. |
| `production-deploy-inputs.env` | Same values as `KEY=value` for review. |
| `production-deploy-candidate-metadata.json` | Machine-readable **semantic** `source_event` (promotion-manifest) vs **`trigger_workflow_event`** (Build GitHub API wrapper), plus ids and digest refs — not passed to `gh workflow run`. |
| `deploy-production-gh-command.sh` | Wrapper around **`gh workflow run`**; exits non‑zero if **`staging_evidence_id`** is TODO or empty without intentional bypass fields. |

## Before dispatch

1. Replace **`staging_evidence_id`** in `production-deploy-inputs.json` (successful **Staging Deployment Contract** run id). **`allow_missing_staging_evidence`** stays **false** here — use explicit bypass inputs only with **`missing_staging_evidence_reason`** when policy allows (not the normal path).
2. Optionally replace **`release_tag`** (`v20260629-1e3fca8`).
3. Confirm **`build_run_id`** matches **`security-verdict.source_build_run_id`** and **`security_release_run_id`** is this Security Release run (not the Build run).

## CLI example

From a clone of this repository (authenticated `gh`), after exporting `REPO_ROOT`:

```bash
bash deploy-production-gh-command.sh
```

Or:

```bash
gh workflow run "Deploy Production" --ref main --json < production-deploy-inputs.json
```
