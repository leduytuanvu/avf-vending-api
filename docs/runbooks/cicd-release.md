# CI/CD and release operation (maintainer guide)

This document is the **entry point** for how GitHub Actions tie together for the AVF vending **API** backend. Workflow **file names** and **display names** match `.github/workflows/*.y` as of the last doc refresh. Deeper detail: [release-process.md](./release-process.md) (artifacts), [github-governance.md](./github-governance.md) (GitHub **Settings**), [supply-chain-security.md](./supply-chain-security.md) (images, SBOM, signing, action pins), [CI_CD_FINAL_AUDIT.md](../../CI_CD_FINAL_AUDIT.md) (enterprise audit).

---

## 1. Expected PR flow: feature → `develop`

1. Open a **pull request** into **`develop`** (or **`main`**, per team policy) from a feature branch.
2. **CI** (`.github/workflows/ci.yml`, name **CI**) runs: tests, `make ci-gates`–equivalent checks, workflow contract checks, migration sanity, compose validation, and related gates.
3. **Security** (`.github/workflows/security.yml`, name **Security**) runs on the PR: **Go Vulnerability Scan** (govulncheck-PR), **Secret Scan**, **Deployment and Config Scan**. **Dependency Review** runs **only** on PRs when the repository variable **`ENABLE_DEPENDENCY_REVIEW`** is **`'true'`**.
4. **CodeQL** (`.github/workflows/codeql.yml`, name **CodeQL**) runs on the PR when **`ENABLE_CODE_SCANNING`** is **`'true'`**; if skipped, that is an **org-gated** skip, not a substitute for the three blocking **Security** jobs.
5. **No** **Build and Push Images**, **no** **Security Release**, and **no** staging/production **deploy** are triggered from the PR head alone. Images and release artifacts are produced only after a **push** to `develop` or `main` (or a qualified manual dispatch) once **CI** succeeds, per `build-push.yml` rules.

**If the PR is red:** fix failing jobs; re-run. See §4 (CI / Security fail).

---

## 2. Expected `develop` flow: push to `develop`

After merge to **`develop`** (or a direct push, if your policy allows):

1. **CI** (push) and **Security** (push) run in parallel.
2. **Build and Push Images** (`.github/workflows/build-push.yml`, name **Build and Push Images**) runs when a successful **CI** `workflow_run` completes for an **eligible** push to **`develop`** (and release-candidate rules are satisfied). It publishes **digest-pinned** image refs to GHCR and uploads artifacts such as **image-metadata**, **promotion-manifest**, **immutable-image-contract**, **release-candidate**, **sbom-reports**, and **cosign-signing-evidence** (as implemented in that workflow).
3. **Security Release** (`.github/workflows/security-release.yml`, name **Security Release**) runs when **Build and Push Images** **completes** (`workflow_run` types: **completed**). It resolves image refs and Build artifacts, runs image scans and the release gate, and publishes **`security-verdict`**.
4. **Staging Deployment Contract** (`.github/workflows/deploy-develop.yml`, name **Staging Deployment Contract**) runs when **Security Release** completes on branch **`develop`**. It consumes the **`security-verdict`** artifact and only proceeds to real remote staging steps when the verdict is **pass** and other workflow gates are met (including org toggles such as **`ENABLE_REAL_STAGING_DEPLOY`**). There is **no** separate `deploy-staging.yml`; staging is **only** this file.

**Happy path (conceptual):** `CI` → `Security` → `Build and Push Images` → `Security Release` → `Staging` (as gated).

---

## 3. Expected `main` flow: `develop` → `main` → production

1. **Merge `develop` into `main`** via a **pull request** (normal promotion). Direct pushes to **`main`** may be restricted by [branch protection](./github-governance.md) in your org.
2. On merge (push to **`main`**): **CI** and **Security** (push) run, then the same **Build** → **Security Release** chain as on **`develop`**, with triggers filtered to **`main`**.
3. For **`main`**, **Security Release** can also generate a **release manifest** (artifact **release-manifest**) for production candidates, as described in [release-process.md](./release-process.md).
4. **Deploy Production** (`.github/workflows/deploy-prod.yml`, name **Deploy Production**) is the **only** workflow that deploys to the **production** environment for GitHub-based rollouts. It runs only on **`workflow_dispatch`** on **`main`**. It is **not** auto-triggered by **Security Release** or **merge to main**. The operator must supply the Build run id, Security Release run id, and digest-pinned image refs; gates require **`verdict: pass`**, **`source_branch: main`**, and related evidence. **GitHub Environments** can require **manual approval** for the **`production`** job before the deploy proceeds.
5. The legacy file **`deploy-production.yml`** is a **no-op pointer** (notice only; no deploy). Use **`deploy-prod.yml`**.

**Do not** use repo **Security** (`security.yml`) or **Build** alone as a deploy approval: only **Security Release** + **`security-verdict`** with **`verdict: pass`** authorizes the deploy workflows’ gates, as implemented.

---

## 4. Common failure triage (where to look)

| Stage | What failed | First steps | Runbook |
|-------|-------------|-------------|---------|
| **CI** | **CI** job red | Logs in **CI** workflow; fix tests, `ci-gates`, contracts, compose, OpenAPI drift. | [ci.yml](../../.github/workflows/ci.yml) job names in Actions |
| **Security** (repo) | **Security** job red (govulncheck, secret scan, config scan) | Fix code/deps or misconfiguration; re-run. Dependency Review / CodeQL skipped is separate (see [github-governance.md](./github-governance.md)). | [security.yml](../../.github/workflows/security.yml) |
| **Build** | **Build and Push Images** failed or skipped | Upstream **CI** must be green; check release-candidate / gate jobs; Build logs. | `build-push.yml` |
| **Security Release** | Red workflow or **`verdict: fail` / `skipped` / `no-candidate`** | Download **`security-verdict`**; read **`failure_reasons`**; job summary. Non-pass → deploy workflows **fail closed**. | [security-release-failure.md](./security-release-failure.md) |
| **Deploy** (staging / prod) | **Staging Deployment Contract** or **Deploy Production** red after a pass verdict | SSH/preflight/smoke per job; **automatic** image rollback in prod is **policy- and LKG-dependent**. | [deploy-failure.md](./deploy-failure.md) |
| **Smoke** | Staging post-deploy smoke failed | **staging-smoke-evidence** artifact; required URLs / auth configured in repo for staging. | [deploy-failure.md](./deploy-failure.md), [post-deploy-smoke-tests.md](./post-deploy-smoke-tests.md) if present |

Out-of-band workflows: **Nightly Security Rescan** (scheduled + manual) and **Manual Ops Evidence Check** (`nightly-ops.yml`, `workflow_dispatch` only — no `schedule`) are **out of band** for the merge/deploy chain; they do not replace **Security** on PR/push or **Security Release** for promotion.

---

## 5. Rollback (production)

- **What rollback does:** repins **app** and **goose** containers to a **previous digest-pinned** image pair using the same deploy/rollback scripts as **Deploy Production**, when in **rollback** `action_mode` with validated inputs. See [rollback-production.md](./rollback-production.md) for LKG selection and **manual** `workflow_dispatch` steps.
- **What rollback does not do:** it does **not** run **database “down” migrations** or automatically reverse schema. Destructive or forward-only schema changes need **operational** recovery (restore, DBA, forward fix) — not only an image pin.
- **Migrations:** migration preflight and first-node apply can **fail a deploy**; that is separate from app container rollback. See [rollback-production.md](./rollback-production.md) and [migration-safety.md](./migration-safety.md) if present.

---

## 6. GitHub UI requirements (manual configuration)

The repository **code** does not set branch rules or environment rules. A **GitHub org/repo admin** must configure:

- **Branch protection** (or **rulesets**) for **`develop`** and **`main`**: required reviews, required **status checks** aligned with your merge policy, no casual bypass where policy disallows. See [github-governance.md](./github-governance.md) (recommended check names, blocking vs org-gated jobs).
- **Environments:** **`production`** (and **`staging`** if used) with **deployment branch** restrictions (**`main`** for production) and **required reviewers** for production deploys. Same runbook, §3.
- **Repository variables** used by workflows (e.g. enablement flags for CodeQL, Dependency Review, staging deploy, signing/provenance policy) must be set in **Settings →** **Actions** to match your org policy. Exact names are documented in each workflow and in [github-governance.md](./github-governance.md).

`tools/verify_github_governance.py` (invoked by `scripts/ci/verify_github_governance.sh`) can **read** the API to compare against this documentation when a token and **`GITHUB_REPOSITORY`** are available; it does not apply settings for you.

---

## 7. Security and supply chain (as implemented in workflows)

- **Digest-pinned images:** production and staging deploy paths require **immutable** `...@sha256:...` app/goose references resolved from **Build** artifacts, not **mutable tags alone**. **Security Release** and deploy validators enforce this. Details: [supply-chain-security.md](./supply-chain-security.md), [security-release-failure.md](./security-release-failure.md).
- **SBOM:** **Build** publishes **CycloneDX** files under the **sbom-reports** artifact; **Security Release** can record SBOM-related metadata in the verdict; **`SBOM_POLICY`** (repository variable) controls warn vs fail if SBOM chain is missing. [supply-chain-security.md](./supply-chain-security.md#sbom_policy-repository-variable).
- **Provenance / attestations:** **`PROVENANCE_ENFORCEMENT`** and **`ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK`** (repository variables) follow the behavior in **Security Release**, **`_reusable-deploy.yml`** (**Reusable Resolve Immutable Image Refs**), and deploy gates. [supply-chain-security.md](./supply-chain-security.md#provenance_enforcement-and-allow_private_repo_provenance_fallback).
- **Action pinning policy:** the offline verifier **warns** on **third-party** `uses:` not pinned to a commit **SHA**; optional **`ENFORCE_ACTION_SHA_PINNING=true`** can fail the contract when your org is ready. Official **`actions/*`** and **`github/codeql-action/*`** are exempt in enforce mode, as documented. [supply-chain-security.md](./supply-chain-security.md#github-actions-version-pins).

**Cosign** image signing and verification are part of the same chain; see the supply-chain runbook and workflow summaries.

---

## 8. Final audit and related documents

- **Enterprise readiness, manual tasks, limitations, and optional improvements** are recorded in [CI_CD_FINAL_AUDIT.md](../../CI_CD_FINAL_AUDIT.md). Re-validate that file after **material workflow or policy changes**.

---

## Quick reference: active workflow files

| File | Display name (typical) | Role |
|------|------------------------|------|
| `ci.yml` | CI | PR + push checks |
| `security.yml` | Security | Scans; not the deploy verdict |
| `codeql.yml` | CodeQL | If enabled |
| `build-push.yml` | Build and Push Images | After CI; images + promotion artifacts |
| `security-release.yml` | Security Release | Image gate, **security-verdict** |
| `deploy-develop.yml` | Staging Deployment Contract | Staging after **Security Release** on `develop` |
| `deploy-prod.yml` | Deploy Production | **Manual** `workflow_dispatch` only — **only** file for GitHub **production** deploy/rollback |
| `deploy-production.yml` | Legacy pointer | **No deploy** — filename compatibility only |
| `nightly-security.yml` | Nightly Security Rescan | Scheduled + manual; not merge/deploy gate |
| `nightly-ops.yml` | Manual Ops Evidence Check | `workflow_dispatch` only; out-of-band evidence |

Full table: [github-governance.md — Active GitHub Actions workflows](./github-governance.md#active-github-actions-workflows-in-this-repository).
