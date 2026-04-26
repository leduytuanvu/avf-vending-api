# GitHub repository governance (Settings-only)

This page lists **exact UI steps** to configure **branch protection** and the **`production` environment** so that manual production deploys and required CI gates match enterprise policy. **Nothing in this repository can create these settings** — only org/repo admins in the GitHub **Settings** UI (or org rulesets) can do so.

**Validation:** `scripts/ci/verify_github_governance.sh` (uses `gh api` via `tools/verify_github_governance.py`) **reads** the REST API. If checks fail, the **Deploy Production** workflow is **not** enterprise-ready until GitHub is configured (no application code change fixes missing GitHub settings).

Deeper runbook (workflow names, check matrices, `GITHUB_TOKEN` patterns): [docs/runbooks/github-governance.md](../runbooks/github-governance.md).

## Branches: protect `main` and `develop`

1. Open **Repository** → **Settings** → **Branches** (or **Code and automation** → **Branches** in the new layout).
2. **Branch protection rules** (or a **Repository ruleset** that maps to the same policy — the verifier reads **classic** `GET /repos/{owner}/{repo}/branches/{branch}/protection`; if you use only rulesets, the API may return **404** and the script will ask you to confirm manually).

### `main`

1. **Add** (or edit) a rule for pattern **`main`**.
2. Enable **Require a pull request before merging** (PRs required; merges are not unreviewed one-click direct commits).
3. **Require approvals**: at least **1** (or your org default).
4. **Require status checks to pass before merging** — add the job checks your policy needs (see the runbook’s tables for exact **Workflow / job** names).
5. **Require branches to be up to date before merging** (strict; the API reports `strict: true` for classic protection).
6. **Do not** allow **force pushes**; **do not** allow **deletions** of the branch.
7. Optionally **Restrict who can push** to matching branches (automation/teams only), in addition to PR flow.

### `develop`

1. **Add** (or edit) a rule for pattern **`develop`**.
2. Require **pull requests** and/or **required status checks** per team policy; block **force pushes** and **deletions** (recommended for a shared integration branch).

## Environments: `production`

1. **Settings** → **Environments** → **New environment** → name exactly **`production`** (the **Deploy Production** workflow uses `environment: production`).

### Required reviewers (mandatory for enterprise readiness)

- Under **Environment protection rules**, add **Required reviewers** and assign at least **one** user or team.
- If the UI offers **Prevent self-review**, enable it where policy allows.
- A deployment to `production` will **wait in the Actions UI** until a reviewer approves (in addition to any workflow inputs and `security-verdict`).

### Deployment branches: `main` only

- **Deployment branches**: choose **Selected branches** (or **Protected branches** only if that matches *main-only* in your org).
- Add only **`main`** (or a ruleset that resolves to the same: production deploys must not run from arbitrary topic branches). **Do not** use **All branches** for production if you need an enterprise bar.

## Secrets: production

- **Settings** → **Environments** → **production** → **Environment secrets** (SSH keys, host names, non-repo credentials).
- **Do not** store production-only secrets in repository-level **Settings → Secrets and variables** if policy requires environment-scoped access control — use the **`production`** environment for deploy secrets.

## Run the verifier locally

1. [Install the GitHub CLI](https://cli.github.com/) (`gh`).
2. `gh auth login` (or export a PAT with `repo` read access to branch protection and environments).
3. From the repo root:

```bash
export GITHUB_REPOSITORY=owner/repo   # or REPOSITORY=owner/repo
export GH_TOKEN=ghp_...                # or GITHUB_TOKEN
bash scripts/ci/verify_github_governance.sh
# Offline self-test (no network):
CHECK_MODE=offline bash scripts/ci/verify_github_governance.sh
```

On success, the script prints **`GOVERNANCE_CHECK: PASS`**. If the API is unreadable (403) and you are testing from a **pull request** with a short-lived token, you may get **`SKIPPED`** or failures — re-run on **`main`** / **`develop`** after merge or with **workflow_dispatch**, or see the runbook for `GITHUB_GOVERNANCE_WARN_ONLY` when the environment payload is incomplete in the API.

## CI behavior (this repository)

- **`.github/workflows/ci.yml`** runs the governance check on **push** to `main`/`develop`, **same-repo pull requests**, and **workflow_dispatch**, but **skips** **fork** pull requests (result **SKIPPED**, does not fail the run).
- Optional env vars used by the Python layer: `ENFORCE_GITHUB_GOVERNANCE` (stricter `main` strict/404 handling), `GITHUB_GOVERNANCE_WARN_ONLY` (downgrade some environment field gaps) — see [github-governance runbook](../runbooks/github-governance.md).
