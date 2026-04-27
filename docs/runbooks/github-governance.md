# GitHub governance — branch protection and environments

**Settings-only quick steps (branch rules + `production` environment):** [../operations/github-governance.md](../operations/github-governance.md)

**Maintainer index (pipelines, branches, and triage):** [cicd-release.md](./cicd-release.md).

This runbook describes how to configure **Repository rulesets** (primary in this repo), **classic branch protection** (fallback), and **GitHub Environments** for the AVF vending backend so enterprise CI/CD matches repository contracts (`deploy-prod.yml`, `deploy-develop.yml`, `security-release.yml`, etc.).

**Reality check:** `tools/verify_github_governance.py` can only **read** what the GitHub REST API exposes. It **lists active branch rulesets** via `GET /repos/.../rulesets` and verifies `main` and `develop` when an **active** ruleset’s `ref_name` conditions include those branches; if no such ruleset exists, it **falls back** to classic `GET /repos/.../branches/{branch}/protection`. The **repo owner (or org admin)** must use the **Settings** UI to apply the policies. When the token cannot read a surface, the API **404**s, or the payload omits fields, follow the checklists in this file and the manual block printed by the verifier.

**Run the verifier (same as CI wrapper):** `make verify-governance` or `bash scripts/ci/verify_github_governance.sh`

Offline self-test: `CHECK_MODE=offline bash scripts/ci/verify_github_governance.sh` (no API calls; validates CLI presence + planned checks).

Needs **`GOVERNANCE_AUDIT_TOKEN`**, **`GH_TOKEN`**, or **`GITHUB_TOKEN`** (read access to **rulesets**, **environments**, and classic **branch protection** as applicable; see [operations doc](../operations/github-governance.md)) and **`GITHUB_REPOSITORY=owner/repo`** (or `REPOSITORY=owner/repo`). Fails when policies are missing or the API response proves misconfiguration. If the response omits `protection_rules` or `deployment_branch_policy` but your org still enforces policy, use **`GITHUB_GOVERNANCE_WARN_ONLY=true`** to treat those items as **warnings** (not for gating merge CI), complete the [P0-4 manual checklist](#p0-4--manual-configuration-checklist-github-ui), then re-run with a token that can read environments.

## P0-4 — Manual configuration checklist (GitHub UI)

Use this when onboarding a repo or when `verify_github_governance` reports API **403/404** (rulesets) or missing fields. Nothing in this repository can set these; they are **Settings** only.

| Area | You must configure |
|------|-------------------|
| **Branch: `main`** | **Settings → Rules → Rulesets** (preferred): **active** branch ruleset with `ref_name` including **`main`**, with rules **pull_request** (≥1 approval, stale + last-push where available), **required_status_checks** (strict), **non_fast_forward**, **deletion**. Or **Settings → Branches** (classic) with the same **intent**. Checks: [Recommended required checks for `main`](#recommended-required-checks-for-main). |
| **Branch: `develop`** | Same as `main` for policy **class** (active ruleset or classic) with **develop**-scoped checks: [develop checks](#recommended-required-checks-for-develop). |
| **Environment `production`** | **Settings → Environments → `production`**. **Required reviewers** (≥1 user/team) — for a 2-person team, keep **1** required approval and use **Prevent self-review** for deployments where available. **Deployment branches: Selected branches** → **`main` only** (not “All branches”). |
| **Secrets for prod deploy** | **Settings → Environments → `production` → Environment secrets** (SSH, hosts, etc.). Do **not** commit production secrets to the repo. |
| **Required checks (names)** | Use the **Check name** column in the tables in this doc (the PR **Checks** tab often shows `Workflow / job`, e.g. `CI / Go CI Gates`). The **Repository rulesets** REST API stores each required check as a shorter `context` (often the job name only, e.g. `Go CI Gates`). The governance verifier (`tools/verify_github_governance.py`) accepts **either** form per the explicit alias list — see [Rulesets API vs PR Checks UI](#rulesets-api-vs-pr-checks-ui). |
| **Deploy workflow** | Production goes only through **[`.github/workflows/deploy-prod.yml`](../../.github/workflows/deploy-prod.yml)** (`workflow_dispatch` on `main`); it does not auto-run from **Security Release** completion. |

## Active GitHub Actions workflows in this repository

**Canonical deploy paths:** production rollouts and rollbacks go only through **[`.github/workflows/deploy-prod.yml`](../../.github/workflows/deploy-prod.yml)** (name **Deploy Production**). Staging (develop) goes only through **[`.github/workflows/deploy-develop.yml`](../../.github/workflows/deploy-develop.yml)** (name **Staging Deployment Contract**). There is **no** `deploy-staging.yml` — the job id `deploy-staging` is internal to that file only. **Do not** add a second staging or production deploy workflow that bypasses **Security Release**.

| Workflow file (`.github/workflows/`) | Display name (approx.) | Role |
| --- | --- | --- |
| `ci.yml` | CI | Tests, contracts, compose; **no** deploy. |
| `security.yml` | Security | Repo-level scans; **not** a deploy/verdict source. |
| `codeql.yml` | CodeQL | Code scanning when enabled. |
| `build-push.yml` | Build and Push Images | Images + release artifacts; chains from CI. |
| `security-release.yml` | Security Release | Image gate, **security-verdict**; after Build. |
| `deploy-develop.yml` | Staging Deployment Contract | Staging **only**; after successful Security Release on `develop`. |
| `deploy-prod.yml` | Deploy Production | **Only** file that can deploy/rollback **production** (`environment: production`). |
| `deploy-production.yml` | Legacy pointer (no deploy) | **Not** a deploy: notice-only, legacy filename. Use **`deploy-prod.yml`**. |
| `nightly-security.yml` | Nightly Security Rescan | Scheduled rescans; not merge/deploy gates. |
| `nightly-ops.yml` | Manual Ops Evidence Check | `workflow_dispatch` only (no `schedule`); ops/evidence, restore drill; **not** deploy. |
| `environment-separation-gates.yml` | Environment separation gates | Policy checks. |
| `enterprise-release-verify.yml` | Enterprise release verification | Static preflight; not a deploy. |
| `telemetry-storm-staging.yml` | Staging telemetry storm suite | **Manual** load/storm; not general staging app deploy. |
| `_reusable-build.yml` | Reusable Build and Push | Called by `build-push.yml`. |
| `_reusable-deploy.yml` | Reusable Resolve Immutable Image Refs | Called by Security Release and deploys. |

## Quick reference: block unsafe paths

### Block direct push to `main` (or reduce it to automation only)

1. **Repository** → **Settings** → **Repository rules** / **Rulesets** (or **Branches** for classic). Prefer an **active** **Repository ruleset** whose **branch** `ref_name` conditions include `refs/heads/main` (or `~DEFAULT_BRANCH` if that is `main`).
2. For **`main`** (or the default branch you treat as production):
   - **Require a pull request before merging** (so normal work merges only via PR).
   - **Require approvals** (at least **1**; increase per org policy).
   - **Require status checks to pass** — add the checks listed in [Recommended required checks for `main`](#recommended-required-checks-for-main) below.
   - **Require branch to be up to date** before merging (strict; matches `/protection` `strict: true` when using classic API).
   - **Do not allow bypassing** the required pull requests and status checks (no casual admin bypass, per your policy).
   - **Block force pushes**; **Block deletions**.
   - **Restrict who can push to matching branches** if policy allows: limit to release automation or teams; otherwise rely on **PR required** + approvals so ad-hoc direct push to `main` is not the default workflow.

If developers can still “push to main” in your org, you likely need an **org-level** or **team** rule—document who may bypass and why.

### Require production deploy approval (GitHub Deployment)

Production rollout uses **`environment: production`** in `.github/workflows/deploy-prod.yml`. Approvals are enforced by the GitHub **Environment** named exactly **`production`** (not the branch name).

1. **Settings** → **Environments** → **New environment** → name **`production`**.
2. **Deployment branches**: **Selected branches** → add **`main`** only (not “All branches”).  
3. **Required reviewers**: add at least one user or team. The **Deploy Production** workflow job waits in the **Actions** UI until a reviewer approves the deployment. This does **not** replace **Security Release** `verdict=pass` or digest-pinned images; it is an **additional** gate.
4. If the UI offers **Prevent self-review**, enable it so the person who triggered the run cannot be the only approver in scope (when your plan supports it).
5. Optional: **wait timer**; add **Environment secrets** (SSH, etc.) here — **never** commit them to the repo.

Repository-side verification (optional): with `GH_TOKEN` or `GITHUB_TOKEN` and `GITHUB_REPOSITORY=owner/repo`, run:

```bash
bash scripts/ci/verify_github_governance.sh
```

Set `ENFORCE_GITHUB_GOVERNANCE=true` in CI when you want missing tokens to **fail** the job (local runs without a token still exit 0 with a warning). To run governance checks after static workflow contracts, set `VERIFY_GITHUB_GOVERNANCE=true` when invoking `scripts/ci/verify_workflow_contracts.sh`.

Do not commit tokens. Use a fine-grained or classic PAT / GitHub Actions `GITHUB_TOKEN` with permission to read branch protection and environments.

**CI (`.github/workflows/ci.yml` — governance job):** If the default `github.token` returns **HTTP 403** for protection or environment APIs, add the repository secret **`GOVERNANCE_AUDIT_TOKEN`**: a **read-only** fine-grained PAT scoped to this repo only, with **Administration: Read** and the other read-only limits described under **“CI: optional `GOVERNANCE_AUDIT_TOKEN`”** in [docs/operations/github-governance.md](../operations/github-governance.md). When unset, CI falls back to `github.token`. Do not use write permissions for this check.

## Repository `GITHUB_TOKEN` permissions (Actions)

Every workflow under `.github/workflows/*.yml` declares an explicit top-level `permissions:` block. The repo enforces this in CI via `tools/verify_github_workflow_cicd_contract.py` (also invoked from `scripts/ci/verify_workflow_contracts.sh`): missing `permissions`, `write-all` / `read-all`, `contents: write`, or elevated scopes in the wrong workflow (for example `packages: write` outside image publish paths) will fail the check.

**Patterns in this repo**

- **CI, CodeQL, enterprise release verify, nightly ops, pointer workflows:** `contents: read` and `actions: read` or `actions: write` only where artifact upload / API use requires it; no `packages: write`.
- **Security (`security.yml`):** `actions: write` for scan artifact uploads; `security-events: write` for SARIF where used; dependency review uses a tighter job with `actions: read` and `pull-requests: read`.
- **CodeQL (`codeql.yml`):** runs on **PR** and **push** to `develop`/`main` plus **schedule**; the analyze job is gated by repository variable **`ENABLE_CODE_SCANNING`** (`'true'`). When not enabled, GitHub shows the job as **skipped** by design — not a missing check (see [Blocking vs informational](#blocking-vs-informational--org-gated-checks) below).
- **Build and Push Images:** default `contents: read` + `actions: read`; jobs that push GHCR images or attestations add `actions: write`, `packages: write`, `attestations: write`, and `id-token: write` on the callable build job only.
- **Security Release, staging/production deploy:** `actions: read` or `actions: write` so `workflow_run` and artifact download/upload work; `packages: read` and `attestations: read` as needed; `deployments: write` where jobs target GitHub Environments; production keeps `id-token: write` for the existing OIDC path.

### GitHub Actions: pin policy (governance, not org settings)

Repository workflows may use **tag- or version-based** `uses:` lines today. **Full commit SHA** pinning is **recommended** for high-impact flows and **required** only if you turn on **`ENFORCE_ACTION_SHA_PINNING=true`** in the environment that runs **`tools/verify_github_workflow_cicd_contract.py`** (fails on unpinned **third-party** actions; official `actions/*` is exempt for now). Default CI behavior is **warn** on third-party non-SHA pins. Details and a safe update checklist: [supply-chain-security.md#github-actions-version-pins](./supply-chain-security.md#github-actions-version-pins). **Do not** set **`ENFORCE_ACTION_SHA_PINNING`** globally until pins are in place, or the contract job will fail.

---

## 1. Protect `main`

Path: **Repository → Settings → Branches → Branch protection rules → Add rule** (or edit existing) for branch name pattern **`main`**.

Enable at minimum:

1. **Require a pull request before merging**  
   - Require approvals: **at least 1** (or per org policy).  
   - Optionally enable **Require review from Code Owners** if you use `CODEOWNERS`.

2. **Require status checks to pass before merging**  
   - Enable **Require branches to be up to date before merging** (strict).  
   - Add the checks listed in [Recommended required checks for `main`](#recommended-required-checks-for-main) below.

3. **Block force pushes** (do not allow force pushes).

4. **Block deletions** (do not allow branch deletion).

5. **Restrict who can push to matching branches** (recommended)  
   - Limit direct pushes to automation or release roles only, or rely on PR-only flow plus org rules.  
   The API verifier treats **required pull request reviews** or explicit **restrictions** as satisfying “no casual direct push to `main`”; adjust to match your org.

6. Save the rule.

---

## Security workflows: blocking vs informational / org-gated checks

Use this when configuring **required status checks** in branch protection. **Never** mark an intentionally skipped or informational job as if it were a hard merge gate for every context.

**Repo Security** (`.github/workflows/security.yml`, workflow name **Security**) runs on **`pull_request`**, **`push` to `develop`/`main`**, and **`workflow_dispatch`**. The three blocking jobs use the same `if:` and are **not** controlled by `vars` (so they are never “silently off” when the workflow runs).

| Category | What counts as enterprise-blocking | Details |
|----------|------------------------------------|---------|
| **Blocking — PRs to `develop` / `main`** | **CI** (script quality, Go, compose) + **Security** — **Go Vulnerability Scan**, **Secret Scan**, **Deployment and Config Scan** | Every open/updated PR must run these; failures block merge. Optional on PR: **Dependency Review** if `vars.ENABLE_DEPENDENCY_REVIEW == 'true'`. |
| **Blocking — pushes to `develop`** | **CI** (push) + same three **Security** jobs | On each push, those jobs must pass. **Not** a substitute for **Security Release** (deploy). |
| **Blocking — pushes to `main`** | **CI** (push) + same three **Security** jobs | Same as develop, plus (for rulesets) **Enterprise release verification** on `main` as in the [main checks table](#recommended-required-checks-for-main). **Security Release Signal** is post-merge (after **Build**), not a branch **required** status check. |
| **Informational, scheduled, or var-skipped (not a full pass on their own)** | See table below | Skipped = **intentional** when a repo var is unset; do not treat that skip as “all security green.” |

### Org-gated in-repo workflows (skip ≠ success)

| Workflow / job | Repository variable (exact string) | Behavior |
|------------------|------------------------------------|----------|
| **CodeQL** — *Analyze Go with CodeQL* | `vars.ENABLE_CODE_SCANNING == 'true'` | In `codeql.yml`, if the var is not **`'true'`**, the analyze job is **skipped**. That is a **documented** absence of SAST, **not** a passed CodeQL run. |
| **Security** — *Dependency Review* | `vars.ENABLE_DEPENDENCY_REVIEW == 'true'` | **Pull_request only** (action does not support `push`). When disabled, the job is **skipped** — not a pass on dependency review. |

A **green** `Security` workflow with Dependency Review or CodeQL **skipped** is still acceptable for branch protection **only** because the **three blocking jobs** ran and because your policy accepts the var-off state. **Security Release** and **Deploy** still require their own gates.

### Informational and out-of-band (do not conflate with repo Security on PR/push)

| Source | Role |
|--------|------|
| **Nightly Security Rescan** (`nightly-security.yml`) | **Scheduled** / manual rescans of `main` — fails the *nightly* workflow on govulncheck/gitleaks/Trivy if configured; **not** a substitute for the **Security** workflow on every PR/push. |
| **nightly-dependency-snapshot** | **`go list -u -m -json`** artifact for upgrade **visibility** — **informational** for release planning. It is **not** listed as a required check for daily merges. If the step’s command fails, the *nightly* job fails, but that does not map 1:1 to **security.yml** (see `nightly-security.yml` comments). |
| **Dependabot** | Opens update PRs on a **schedule**; it does **not** run **Dependency Review** and is **not** a replacement for the **Security** workflow. See `.github/dependabot.yml`. |

---

## 2. Protect `develop`

Path: **Settings → Branches → Branch protection rules** → pattern **`develop`**.

Enable:

1. **Require a pull request before merging** *or* **Require status checks** (the verifier requires at least one of these patterns; PR + checks is best practice).

2. **Require status checks** with strict where appropriate.

3. Add checks from [Recommended required checks for `develop`](#recommended-required-checks-for-develop).

4. **Block force pushes** and **Block deletions** per team policy (recommended for shared integration branches).

---

## 3. Environment: `production`

Path: **Repository → Settings → Environments** → **New environment** → name **`production`** (exact name; `deploy-prod.yml` uses `environment: production`).

Configure:

1. **Deployment branches**  
   - Choose **Selected branches** (not “All branches”).  
   - Add **`main`** as the only allowed branch (or **`refs/heads/main`** if the UI offers ref patterns).  
   - Production workflows assume **`main`**; `tools/verify_github_governance.py` checks branch policies when the API returns them.

2. **Required reviewers** (mandatory for enterprise safety)  
   - Open **Environment protection rules** → **Required reviewers** → **Add up to 6 people or teams**.  
   - Select at least one **user** or **team** who must click **Approve** on the **Deploy Production** deployment gate before SSH rollout steps run.  
   - This is **not** auto-approval: the workflow still requires **Security Release** `verdict=pass` and digest-pinned images; reviewers only unblock the GitHub **deployment** to `production`.  
   - Optional: add a **Wait timer** if policy requires a delay before approval counts.

3. **Environment secrets**  
   - Add deployment secrets here (SSH hosts, paths, etc.; not documented in this repo).

4. **Save**.

**Verification:** With `GH_TOKEN` / `GITHUB_TOKEN` and `GITHUB_REPOSITORY=owner/repo`, run **`make verify-governance`** (or `python tools/verify_github_governance.py`). Missing **production** environment, **required reviewers** (when the API lists rules), or **main-only** deployment policy fails by default. If the API omits `protection_rules` / `deployment_branch_policy`, the verifier **fails** with a **manual verification** message (or set `GITHUB_GOVERNANCE_WARN_ONLY=true` to warn while you fix token permissions). `ENFORCE_GITHUB_GOVERNANCE=true` also tightens **branch** protection checks (e.g. strict required checks on `main`).

**Rollback / LKG:** See [rollback-production.md](./rollback-production.md).

---

## 4. Environment: `staging`

Path: **Settings → Environments → New environment** → name **`staging`**.

This repo references `environment: staging` in staging-oriented workflows. Configure **deployment branches** for **`develop`** (and any branch your org allows for staging), plus optional **required reviewers** per policy.

If you intentionally omit a `staging` environment, document the exception in your internal ops docs; the verifier will **warn** if `staging` is missing.

---

## Recommended required checks for `main`

Add these as **required status checks** on `main` (in the **Checks** tab you will usually see `Workflow / job`; the rulesets API may list only the job-style **context** — both are valid if they match the [alias rules](#rulesets-api-vs-pr-checks-ui)):

**Blocking (merge)**

| Check name |
|------------|
| CI / Workflow and Script Quality |
| CI / GitHub repository governance |
| CI / Go CI Gates |
| CI / Docker Compose Config Validation |
| Security / Secret Scan |
| Security / Go Vulnerability Scan |
| Security / Deployment and Config Scan |
| Enterprise release verification / verify-enterprise-release |

### Post-merge: Security Release Signal (not a required branch check)

| Item | Notes |
|------|--------|
| **Security Release / Security Release Signal** | Runs after **Build and Push Images** (`workflow_run`). It provides **verdict, artifacts, and release evidence** on pushes to `main` — **not** a check that can be expected on every PR head commit, so the governance verifier does **not** require it in **required status checks** for `main`. Track it in **Actions** after merge and in deploy inputs / runbooks, not as a merge gate. |

**PR-only / when enabled (supply chain)**

| Check name |
|------------|
| Security / Dependency Review | Only when `vars.ENABLE_DEPENDENCY_REVIEW` is `'true'`; PRs only — see [Security workflows](#security-workflows-blocking-vs-informational--org-gated-checks). |

**Optional / org-gated (enable when policy allows)**

| Check name |
|------------|
| CodeQL / Analyze Go with CodeQL | Only when `vars.ENABLE_CODE_SCANNING` is `'true'`; otherwise the job is skipped by design. |

**Do not** add **Nightly Security Rescan** or **nightly-dependency-snapshot** as required checks for day-to-day merges.

---

## Recommended required checks for `develop`

**Blocking (merge)**

| Check name |
|------------|
| CI / Workflow and Script Quality |
| CI / GitHub repository governance |
| CI / Go CI Gates |
| CI / Docker Compose Config Validation |
| Security / Secret Scan |
| Security / Go Vulnerability Scan |
| Security / Deployment and Config Scan |

**PR-only / when enabled:** Security / Dependency Review (same rules as `main`).

**Optional / org-gated:** CodeQL / Analyze Go with CodeQL when `ENABLE_CODE_SCANNING` is enabled.

---

## Rulesets API vs PR Checks UI

- The **PR Checks** tab lists combined labels such as `CI / Go CI Gates` (workflow display name, a slash, then job name).
- **`GET /repos/{owner}/{repo}/rulesets/{id}`** returns `rules[].type == "required_status_checks"` and `parameters.required_status_checks[].context` using the **short** check context GitHub uses internally (e.g. `Go CI Gates`, `Secret Scan`, `verify-enterprise-release`). That is normal; it does not mean the wrong job is required.

**How to verify:** fetch the ruleset JSON and read the `context` strings, then compare them to the **accepted** names for each recommended check (full UI label and short API name are both valid when listed in `REQUIRED_STATUS_CHECK_ALIASES` in `tools/verify_github_governance.py`).

```bash
# Replace RULESET_ID with the numeric id from: gh api "repos/OWNER/REPO/rulesets?per_page=100"
gh api "repos/OWNER/REPO/rulesets/RULESET_ID" --jq '.rules[] | select(.type=="required_status_checks") | .parameters.required_status_checks'
```

**Alias examples (explicit; no fuzzy matching):** the verifier treats `CI / Go CI Gates` and `Go CI Gates` as the same policy requirement; `Security / Secret Scan` and `Secret Scan`; `Enterprise release verification / verify-enterprise-release` and `verify-enterprise-release`. See `REQUIRED_STATUS_CHECK_ALIASES` in `tools/verify_github_governance.py` for the full list.

## Rulesets vs classic branch protection

If the repository uses **repository rulesets** instead of classic **branch protection rules**, the REST endpoint `GET /repos/{owner}/{repo}/branches/{branch}/protection` may return **404** even when the branch is protected. In that case, either mirror the policy checks in rulesets or extend automation to use the [rulesets API](https://docs.github.com/en/rest/repos/rules).

---

## Related

- `scripts/ci/verify_workflow_contracts.sh` — static workflow graph contracts (no GitHub API), including **Security** blocking jobs and **CodeQL** triggers.  
- `tools/verify_github_workflow_cicd_contract.py` — same graph plus explicit **govulncheck PR+push**, **Dependency Review PR-only**, and **CodeQL** gates.  
- `tools/verify_github_governance.py` — implementation of API checks.
