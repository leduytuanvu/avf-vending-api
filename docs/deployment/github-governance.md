# GitHub repository governance (Settings-only)

This page lists **exact UI steps** to configure **Repository rulesets** (and optionally **classic branch protection**) for `main` and `develop`, plus the **`production` environment**, so that manual production deploys and required CI gates match enterprise policy. **Nothing in this repository can create these settings** — only org/repo admins in the GitHub **Settings** UI (or org rulesets) can do so.

**Validation:** `scripts/ci/verify_github_governance.sh` (uses `gh api` via `tools/verify_github_governance.py`) **reads** the GitHub REST API, **preferring** repository **rulesets** and **falling back** to classic `.../branches/.../protection` when no active ruleset covers a ref. If checks fail, the **Deploy Production** workflow is **not** enterprise-ready until GitHub is configured (no application code change fixes missing GitHub settings).

Deeper runbook (workflow names, check matrices, `GITHUB_TOKEN` patterns): [docs/runbooks/github-governance.md](../runbooks/github-governance.md).

## CI: optional `GOVERNANCE_AUDIT_TOKEN` (repository secret)

**GitHub repository governance** in `.github/workflows/ci.yml` sets `GOVERNANCE_AUDIT_TOKEN` from `secrets.GOVERNANCE_AUDIT_TOKEN` and sets `GITHUB_TOKEN` to the same when that secret is present, otherwise to the default `github.token` (see expression `secrets.GOVERNANCE_AUDIT_TOKEN || github.token` in the workflow). The **shell script and Python verifier** prefer `GOVERNANCE_AUDIT_TOKEN` in the environment, then `GH_TOKEN` / `GITHUB_TOKEN`. Configure a **dedicated** read-only token so the job can list **repository rulesets**, read **environments**, and (when used) **classic** branch protection without avoidable **HTTP 403** from a short-lived `GITHUB_TOKEN` on some events.

**Fine-grained personal access token** (create in GitHub **Settings → Developer settings**; **do not** commit the value):

| Permission | Level |
| --- | --- |
| Repository access | **Only** this repository (`avf-vending-api`) |
| Administration | **Read-only** (rulesets, classic protection metadata) |
| Actions | **Read-only** (when the API path or `gh` needs it) |
| Contents / Code | **Read-only** (when the API or `gh` requires it) |
| Metadata | **Read** (default) |
| All other access | **No access** (no write scopes) |

No write scopes are required for this check. Store the token only as the repository secret **`GOVERNANCE_AUDIT_TOKEN`** in **Settings → Secrets and variables → Actions** (or org-level if your policy allows).

## Branches: protect `main` and `develop` (rulesets first)

1. **Preferred:** **Repository** → **Settings** → **Code and automation** → **Repository rules** (or **Rules** → **Rulesets**). Add **active** **branch** rulesets that **include** `refs/heads/main` and `refs/heads/develop` (or equivalent `ref_name` patterns). Each ruleset that protects these branches should include, at minimum, rules of type: **pull_request** (with required approvals), **required_status_checks** (strict), **non_fast_forward** (block force push), and **deletion** (block branch deletion). The CI verifier calls the **Repository rulesets** API first, then **classic** branch protection if no matching active ruleset exists. **UI vs API check names:** the PR **Checks** list may show `CI / Go CI Gates` while `GET /repos/.../rulesets/{id}` returns `required_status_checks[].context` values like `Go CI Gates`. That is expected; the verifier matches against both forms (see [Rulesets API vs PR Checks UI](../runbooks/github-governance.md#rulesets-api-vs-pr-checks-ui) in the runbook). To audit: `gh api repos/<owner>/<repo>/rulesets/<id>` and compare `context` strings to the accepted aliases in `tools/verify_github_governance.py` (`REQUIRED_STATUS_CHECK_ALIASES`).
2. **Alternative** — **Settings** → **Branches** → **Branch protection rules** (classic). The verifier uses `GET /repos/{owner}/{repo}/branches/{branch}/protection` as a **fallback** when rulesets are absent or do not target the branch. **Do not** rely on **evaluate**-mode rulesets for merge gating; rulesets should be **active** for enforcement.

### `main`

1. In **rulesets** (preferred) or **classic** protection, target **`main`** (e.g. `refs/heads/main` in ruleset `ref_name` conditions).
2. **Require a pull request before merging**; **required approvals: at least 1** (for a 2-person team, keep **1** — do not reduce reviewers to “opt out” of review; use **Prevent self-review** for production deployments instead of lowering the count). Enable **dismiss stale reviews on new push** and **last push approval** when the UI offers them.
3. **Required status checks** — add the job checks your policy needs (see the runbook’s tables for **Workflow / job** names), with **up to date / strict** behavior. On **`main`**, include **Enterprise release verification**; do **not** list **Security Release / Security Release Signal** as a **required** ruleset check — that job is **post-merge** (after **Build**). See the runbook subsection *Post-merge: Security Release Signal* under [Recommended required checks for `main`](../runbooks/github-governance.md#recommended-required-checks-for-main).
4. **Block force pushes** and **branch deletion** (ruleset: `non_fast_forward` and `deletion`).
5. Optionally **restrict who can push** to matching branches, in addition to the PR flow.

### `develop`

1. Add an **active** ruleset (or classic rule) for **`develop`** with the same **class** of controls as `main` (PRs, ≥1 approval, required checks, no force push, no delete), with required check names as in the runbook.

## Environments: `production`

1. **Settings** → **Environments** → **New environment** → name exactly **`production`** (the **Deploy Production** workflow uses `environment: production`).

### Required reviewers (mandatory for enterprise readiness)

- Under **Environment protection rules**, add **Required reviewers** and assign at least **one** user or team.
- If the UI offers **Prevent self-review**, enable it (recommended for 2-person teams: keep **1** required approver, but block the deploy actor from self-approving the **environment** approval alone).
- A deployment to `production` will **wait in the Actions UI** until a reviewer approves (in addition to any workflow inputs and `security-verdict`).

### Deployment branches: `main` only

- **Deployment branches**: choose **Selected branches** (or **Protected branches** only if that is *main-only* in your org).
- Allow only **`main`** / `refs/heads/main` — the verifier **fails** if custom branch policies list patterns other than those. **Do not** use **All branches** for production in an enterprise bar.

## Secrets: production

- **Settings** → **Environments** → **production** → **Environment secrets** (SSH keys, host names, non-repo credentials).
- **Do not** store production-only secrets in repository-level **Settings → Secrets and variables** if policy requires environment-scoped access control — use the **`production`** environment for deploy secrets.

## Run the verifier locally

1. [Install the GitHub CLI](https://cli.github.com/) (`gh`).
2. `gh auth login` (or export a read-only **fine-grained** PAT with ruleset + environment read — see the table under **CI: optional `GOVERNANCE_AUDIT_TOKEN`**).
3. From the repo root:

```bash
export GITHUB_REPOSITORY=owner/repo   # or REPOSITORY=owner/repo
export GH_TOKEN=                      # set to a read-only PAT with repo+admin read (see above); never commit
# or: export GITHUB_TOKEN=…
bash scripts/ci/verify_github_governance.sh
# Offline self-test (no network):
CHECK_MODE=offline bash scripts/ci/verify_github_governance.sh
```

On success, the script prints **`GOVERNANCE_CHECK: PASS`**. If the API is unreadable (403) and you are testing from a **pull request** with a short-lived token, you may get **`SKIPPED`** or failures — re-run on **`main`** / **`develop`** after merge or with **workflow_dispatch**, or see the runbook for `GITHUB_GOVERNANCE_WARN_ONLY` when the environment payload is incomplete in the API.

## CI behavior (this repository)

- **`.github/workflows/ci.yml`** runs the governance check on **push** to `main`/`develop`, **same-repo pull requests**, and **workflow_dispatch**, but **skips** **fork** pull requests (result **SKIPPED**, does not fail the run).
- Optional env vars used by the Python layer: `ENFORCE_GITHUB_GOVERNANCE` (stricter `main` strict/404 handling), `GITHUB_GOVERNANCE_WARN_ONLY` (downgrade some environment field gaps) — see [github-governance runbook](../runbooks/github-governance.md).
