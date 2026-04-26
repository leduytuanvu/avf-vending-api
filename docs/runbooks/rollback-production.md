# Production rollback and last-known-good (LKG)

**Context:** [cicd-release.md](./cicd-release.md) — §5 (rollback at a glance).

This runbook describes how **Deploy Production** (`.github/workflows/deploy-prod.yml`) records prior releases, how a **rollback candidate** is chosen, and what rollback **does not** do.

**Related:** [deploy-failure.md](./deploy-failure.md) (deploy triage), [security-release-failure.md](./security-release-failure.md) (release gate triage).

## Preconditions

- All automatic production deploys are triggered only after **Security Release** succeeds on **`main`** with **`verdict=pass`** (and related gates). Nothing here bypasses that chain.
- Canonical workflow file: **`deploy-prod.yml`** (display name **Deploy Production**). The pointer **`deploy-production.yml`** does not deploy.

## GitHub Environment `production`

Production jobs use `environment: production`. Operators must configure:

1. **Repository → Settings → Environments → production**
2. **Deployment branches** → **Selected branches** → **`main`** only.
3. **Required reviewers** → at least one user or team (see [github-governance.md](./github-governance.md) §3).

## How the rollback candidate (LKG) is selected

During **Deploy Production**, job **Resolve Previous Production Deployment** scans recent **successful** workflow runs for **`deploy-prod.yml`**:

- **Triggers included:** **`workflow_dispatch`** (manual deploy/rollback) and **`workflow_run`** (automatic deploy after **Security Release**). Both are searched; LKG is **not** limited to manual runs.
- **Exclusions:** the **current** run id; runs without a **`production-deployment-manifest`** artifact; manifests that fail validation (below).

For each candidate (newest successful runs first), it downloads **`production-deployment-manifest`** and validates:

| Rule | Reason |
|------|--------|
| `source_branch` is **`main`** (or `refs/heads/main`) | Production LKG must be from the release branch. |
| `action_mode` is **`deploy`** | **Rollback** runs must not be used as LKG (avoids chaining rollbacks incorrectly). |
| **`app_image_ref`** and **`goose_image_ref`** are digest-pinned (`...@sha256:...`, not `:latest`) | Rollback inputs must be immutable. |

The **first** run that satisfies all rules becomes **rollback_available=true** and the job summary prints digest-pinned **`rollback_app_image_ref`** / **`rollback_goose_image_ref`** for a manual **workflow_dispatch** in **rollback** mode.

If no run qualifies, **rollback_available=false** and the summary lists inspected runs and skip reasons (e.g. old manifest from **rollback** mode, wrong branch, or missing digests).

## Manual rollback (workflow_dispatch)

1. Open **Actions → Deploy Production → Run workflow** (branch **`main`**).
2. Set **action_mode** to **`rollback`**.
3. Set **release_tag** (e.g. `rollback-2026-04-26`).
4. Paste **`rollback_app_image_ref`** and **`rollback_goose_image_ref`** from the LKG summary (or from an archived manifest). Both must stay digest-pinned.
5. Submit; complete **Environment** approval if required.

**Security Release** is not re-run for rollback; you are repinning images to a prior known pair. Do not use rollback to skip a failed policy — fix forward with a new build and Security Release when appropriate.

## What rollback does and does not do

**Does:**

- Redeploys **app-node** (and related) containers to the **previous digest-pinned app and goose images** via existing rollback scripts (`rollback_app_node.sh`, etc.), consistent with `production-deployment-manifest` fields such as `migration_rollback_policy: never_automatic` and `auto_rollback_scope: app_and_goose_images_only`.

**Does not:**

- **Run database down migrations** or `goose down` automatically. Schema changes that already ran on production may require **manual DBA / recovery** (restore, forward fix, or controlled migration).
- **Auto-approve** production: GitHub **Environment** reviewers still apply.
- **Bypass Security Release** for normal promotion: new code still ships only through **Build → Security Release → Deploy** with **`verdict=pass`**.

If a deploy introduced a **destructive** or incompatible schema change, treat rollback as **operational recovery**: stop traffic, assess data, and follow your DR/runbook — not only an image pin.

## Related

- [github-governance.md](./github-governance.md) — production environment and reviewers.  
- [release-process.md](./release-process.md) — release artifacts and manifest chain.
