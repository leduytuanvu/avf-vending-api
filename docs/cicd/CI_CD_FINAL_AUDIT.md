# CI/CD final enterprise audit — AVF vending backend

**Audit date:** 2026-04-26  
**Auditor role:** Repository CI/CD coherence, enterprise safety, and internal consistency (workflows, scripts, tools, runbooks — **no** Go business logic or database schema reviewed in this document).

**Scope:** `.github/workflows`, `scripts/ci`, `scripts/security`, `scripts/release`, `scripts/deploy`, `tools`, `docs/runbooks`, `.gitignore`.

**Maintainer index** (pipelines, required **Settings** in GitHub, triage): [docs/runbooks/cicd-release.md](docs/runbooks/cicd-release.md).

**Contract automation:** `scripts/ci/verify_workflow_contracts.sh` (shell) and `tools/verify_github_workflow_cicd_contract.py` (Python) enforce the same enterprise graph, explicit `permissions`, no deploy on `on.pull_request`, deploy-only-from-Security-Release, `security-verdict` from **security-release.yml** only, digest-pinned deploy refs, and related rules; violations print `CONTRACT VIOLATION: .github/workflows/…` with **Violated contract** and **Expected behavior** lines.

---

## Executive summary

The repository implements a **single promotion chain** for releasable artifacts: **CI → repo Security → Build and Push Images → Security Release → (optional) staging deploy; production is manual-only**. **Security Release** is the **only** workflow that publishes **`security-verdict`** and thus the **only** source of that verdict for **`deploy-develop.yml`**; **`deploy-prod.yml`** (production) is **`workflow_dispatch` only** and **does not** auto-run after **Security Release**, but it **requires** a passing **`security-verdict`** and matching Build/Security Release run ids in **inputs** (same contract). Repo-level **Security** (`security.yml`) performs scans but **never** substitutes for that verdict. **Non-pass** verdicts (**skipped**, **no-candidate**, **fail**) **do not** authorize deploy; deploy workflows **fail closed** on **skipped** / **no-candidate** / **fail** (enforced by workflow logic and offline contract scripts).

**Overall enterprise readiness: 9 / 10** — one point reserved for **manual** GitHub configuration (branch protection rules, environment approvers, and protected storage for credentials and other sensitive values in **Settings**) that cannot be fully proven from the repo without an authenticated governance run.

**Code cannot fully enforce GitHub UI settings by itself** — this repository can define workflows (for example `environment: production` on the **Deploy Production** job), run offline contract verifiers, and run **read-only** governance checks with `tools/verify_github_governance.py`, but **only a repo or org admin** can lock down **Settings → Branches** and **Settings → Environments** in GitHub. See [docs/runbooks/github-governance.md](docs/runbooks/github-governance.md) and the “Remaining manual” / “Limitations” sections below.

---

## Architecture (text diagram)

```
Pull request → develop | main
├── CI (ci.yml): tests, workflow contracts, migrations sanity, compose, …
├── Security (security.yml): govulncheck (PR+push), secret scan, config scan;
│   Dependency Review = PR only, gated by vars.ENABLE_DEPENDENCY_REVIEW
└── No Build / no Security Release / no deploy from PR-only merge queue alone
    (Build chains from CI via workflow_run on push to develop|main, not from PR head in isolation)

Push to develop | main
├── CI (push)
├── Security (push) — blocking scans; not a deploy trigger
├── Build and Push Images — on successful CI workflow_run (push) + release_candidate gate;
│   publishes images, SBOM, signing evidence, promotion artifacts
└── Security Release — on completed Build; writes security-verdict (+ release-manifest on main when applicable)

After Security Release (workflow_run completed + success where required)
├── deploy-develop.yml — ONLY if triggering run is Security Release on branch develop;
│   consumes security-verdict; requires verdict=pass, source_branch=develop, digest pins, …
└── deploy-prod.yml — ONLY automatic path if Security Release on branch main;
    requires verdict=pass, source_branch=main, digest pins, production environment, …
    Manual: workflow_dispatch on main (rollback/deploy) with explicit confirmations and digest inputs

Does NOT appear in the promotion chain for deploy authorization
├── security.yml as verdict source (no security-verdict artifact)
├── nightly-security.yml (scheduled rescan) / nightly-ops.yml (Manual Ops Evidence Check, manual only) → out-of-band; no production deploy
├── deploy-production.yml → pointer only (no deploy)
└── enterprise-release-verify.yml → extra verification; not Security Release, not deploy
```

---

## Enterprise readiness score

| Area | Score | Evidence (repo) |
|------|-------|------------------|
| **Chain integrity** | **Strong** | Single **Security Release** gate; `verify_workflow_contracts.sh` + `verify_github_workflow_cicd_contract.py` enforce graph. |
| **Deploy authorization** | **Strong** | Deploy workflows listen to **Security Release** only; no `security.yml` workflow_run consumer. |
| **Supply chain** | **Strong** | Digest-pinned deploy refs; Build **`release-candidate.json`** + **`sbom-reports`**; **`SBOM_POLICY`** (`warn` \| `enforce`) in Security Release; cosign + provenance in verdict and deploy gates. |
| **Operational clarity** | **Strong** | Runbooks for failure/rollback/staging; `CI_CD_FINAL_AUDIT.md`; optional notify hook (no secret printed). |
| **Governance automation** | **Good** | Offline permission checks in Python verifier; API governance in `verify_github_governance.py` when token present. |
| **Hygiene** | **Strong** | `.gitignore` for caches and local CI outputs; no `__pycache__` / `*.pyc` in tree at audit. |

---

## Audit requirements (verified)

### 1. PR flow (feature/* → develop or main)

| Requirement | Status | Notes |
|-------------|--------|--------|
| CI runs on PR | **OK** | `ci.yml`: `pull_request` to `develop` and `main`. |
| Security PR scans | **OK** | `security.yml`: `pull_request` + govulncheck-pr, secret-scan, config-scan; Dependency Review PR-only + `ENABLE_DEPENDENCY_REVIEW`. |
| Build only if intentionally configured | **OK** | `build-push.yml`: no `on.push` to workflow; `workflow_run` from **CI** + branch/develop|main + release_candidate gate — not “every PR”. |
| No deploy on PR | **OK** | `deploy-develop.yml` / `deploy-prod.yml`: no `pull_request` trigger; contracts forbid production PR triggers. |

### 2. Develop flow (push develop)

| Requirement | Status | Notes |
|-------------|--------|--------|
| CI push | **OK** | `ci.yml` includes `push` to `develop`. |
| Security push | **OK** | `security.yml` includes `push` to `develop`. |
| Build and Push Images | **OK** | After eligible **CI** `workflow_run` (push) per `build-push.yml`. |
| Security Release | **OK** | `workflow_run` from **Build and Push Images** completed. |
| Staging only if verdict pass | **OK** | `validate_release_verdict.py` + workflow gates; non-pass fails closed. |
| source_branch develop | **OK** | Staging candidate validation requires develop (workflow + script). |

### 3. Main flow (push main)

| Requirement | Status | Notes |
|-------------|--------|--------|
| CI + Security push | **OK** | Same pattern as develop for `main`. |
| Build + Security Release | **OK** | Same chain; branches filtered in triggers. |
| Deploy Production only if verdict pass (auto) | **OK** | `deploy-prod.yml` resolves verdict and rejects skipped/no-candidate/fail. |
| source_branch main | **OK** | Automatic production resolver requires `main`. |
| `environment: production` | **OK** | `deploy-prod` job declares production environment. |

### 4. Security Release

| Requirement | Status | Notes |
|-------------|--------|--------|
| Single SoT for deploy authorization | **OK** | Only workflow uploading **`security-verdict`** for deploy consumers. |
| Emits security-verdict when possible | **OK** | Signal + upload steps; emergency path if file missing before enforce. |
| Deterministic modes | **OK** | `write_security_verdict.py` **CONTRACT_VERDICT_MODES** + contract checker. |
| skipped / no-candidate → workflow may succeed, not deployable | **OK** | Enforce step exits 0 for pass/skipped/no-candidate; deploy workflows reject skipped/no-candidate. |
| fail → workflow failure | **OK** | Enforce exits non-zero on **fail** verdict. |

### 5. Deploy workflows

| Requirement | Status | Notes |
|-------------|--------|--------|
| Never consume verdict from `security.yml` | **OK** | No `actions/workflows/security.yml/runs` in deploy workflows (contract enforced). |
| Only Security Release security-verdict | **OK** | `workflow_run` from **Security Release** + artifact download by Security Release run id. |
| verdict pass | **OK** | Validators + inline gates. |
| source_sha / source_branch | **OK** | Validated vs verdict and/or promotion context. |
| Digest-pinned refs | **OK** | Policy in deploy + `need_digest_pinned` / scripts. |
| Reject skipped/no-candidate/fail | **OK** | Explicit in workflows + `validate_release_verdict.py`. |
| Health / smoke | **OK** | Production rollout includes smoke steps. Staging (`deploy-develop.yml`) runs **`scripts/deploy/staging_smoke_evidence.sh`** after deploy (required HTTP checks in `tools/smoke_test.py`; optional probes **skip** with reason if unset) and uploads **`staging-smoke-evidence`**. Staging evidence is for operators — **not** a production gate and **not** an automatic promote-to-prod path. |
| Rollback candidate (LKG) | **OK** | Documented in `docs/runbooks/rollback-production.md`; manifest rules in deploy-prod. |

### 6. Security scanning (repo)

| Requirement | Status | Notes |
|-------------|--------|--------|
| Secret scan | **OK** | `security.yml` **Secret Scan** — blocking on PR + push + dispatch (same `if:` as govulncheck/config; not var-gated). |
| Govulncheck | **OK** | **Go Vulnerability Scan** — PR + push + dispatch, blocking. |
| Config scan | **OK** | **Deployment and Config Scan** (Trivy) — same. |
| CodeQL explicit | **OK** | Gated by **`vars.ENABLE_CODE_SCANNING == 'true'`** (string) in `codeql.yml` — when skipped, that is **not** a successful CodeQL analysis (see `github-governance.md` matrix). |
| Dependency Review explicit | **OK** | PR-only; **`vars.ENABLE_DEPENDENCY_REVIEW == 'true'`** — skipped when off; **not** a pass on review. |
| Skipped var-gated jobs ≠ “all security done” | **OK** | Docs and contract separate **blocking** `security.yml` jobs from **org-gated** CodeQL/Dependency Review; **Security Release** verdict does not treat skipped CodeQL/Dep Review as enterprise success on their own. |
| Nightly dependency snapshot | **OK** | **Informational** / visibility (`nightly-dependency-snapshot`); not a required merge check. Do not conflate with `security.yml` on PR/push. |

### 7. Supply chain

| Requirement | Status | Notes |
|-------------|--------|--------|
| Digest-pinned deploy images | **OK** | Enforced in deploy paths. |
| SBOM | **OK** | CycloneDX / `sbom-reports` from Build; promotion-manifest **sbom** in verdict; optional **`SBOM_POLICY=enforce`**. |
| Build release-candidate | **OK** | `build-push.yml` uploads **`release-candidate`**; Security Release preflights and downloads it; **`build_release_evidence`** in `security-verdict.json`. |
| Signing / provenance explicit | **OK** | Cosign in build; `SIGNING_ENFORCEMENT`, `PROVENANCE_ENFORCEMENT`, verdict fields. |
| Private repo fallback visible | **OK** | Named trust classes / fallbacks in workflows and deploy gates. |
| Action pins (governance) | **OK** | `verify_github_workflow_cicd_contract.py` lists high-impact `uses` and warns on third-party non-SHA; **`ENFORCE_ACTION_SHA_PINNING`** is opt-in (not default in CI). |

### 8. Governance

| Requirement | Status | Notes |
|-------------|--------|--------|
| Branch protection docs | **OK** | `docs/runbooks/github-governance.md` + tool recommendations. |
| Production approval docs | **OK** | Same runbook + `rollback-production.md`. |
| Governance verifier | **OK** | `scripts/ci/verify_github_governance.sh` → `tools/verify_github_governance.py`. |
| Least-privilege permissions | **OK** | `verify_github_workflow_cicd_contract.py` explicit permissions; no broad write-all. |

### 9. Hygiene

| Requirement | Status | Notes |
|-------------|--------|--------|
| No committed `__pycache__` / `*.pyc` | **OK** | Tree clean at audit; `.gitignore` includes `**/__pycache__/` and `*.pyc`. |
| Local cache / ephemeral outputs | **OK** | `.gitignore` includes `/security-reports/`, `trivy-*.txt`, `**/*.exitcode`, `.cache/`, `/artifacts/`, `/tmp-ci/`, and similar at repo root. |
| Active vs legacy workflows | **OK** | [docs/runbooks/github-governance.md#active-github-actions-workflows-in-this-repository](docs/runbooks/github-governance.md#active-github-actions-workflows-in-this-repository) lists deploy paths; `deploy-production.yml` is pointer-only. |
| No committed secrets | **OK** | Examples only; prod paths ignored (discipline). |
| No accidental duplicate deploy | **OK** | `deploy-production.yml` pointer-only + documented; no second staging workflow file. |

### 10. Coherence / non-contradiction

- **Security Release** vs **Security**: distinct names and roles; contracts require **Security Release** for deploy triggers.  
- **Skipped/no-candidate** success at Security Release does not contradict deploy fail-closed: different workflows; policy is “not deployable.”  
- **Nightly** jobs do not auto-deploy per workflow design and contract checks.

---

## Implemented controls (summary table)

| Workflow | Role |
|----------|------|
| `ci.yml` | CI gates, contracts, migrations layout checks |
| `security.yml` | Repo scans; **not** deploy verdict |
| `build-push.yml` | Images, SBOM, signing evidence, promotion artifacts |
| `security-release.yml` | **Release gate**, **security-verdict**, optional **release-manifest** (main) |
| `deploy-develop.yml` | Staging after **Security Release** on **develop**; post-deploy **staging-smoke-evidence** when real deploy is enabled |
| `deploy-prod.yml` | Production (**environment: production**) after **Security Release** on **main** or dispatch |
| `deploy-production.yml` | **Pointer only** — no deploy |
| `nightly-security.yml` | Scheduled rescan; **no** deploy |
| `enterprise-release-verify.yml` | Extra checks; **not** a deploy trigger |

---

## Commands (audit / CI parity)

From repo root:

```bash
git diff --check
bash scripts/ci/verify_workflow_contracts.sh
# Ensure VERIFY_ACTION_SHA_PINNING is unset unless you want extra informational output:
python tools/verify_github_workflow_cicd_contract.py
bash scripts/ci/verify_migrations.sh
```

**GitHub governance** (optional; requires **`GH_TOKEN`** or **`GITHUB_TOKEN`** and **`GITHUB_REPOSITORY=owner/repo`**):

```bash
bash scripts/ci/verify_github_governance.sh || true
# Non-enforcing local run:
ENFORCE_GITHUB_GOVERNANCE=false bash scripts/ci/verify_github_governance.sh
```

**Full enterprise script** (includes Go tests, compose, OpenAPI):

```bash
bash scripts/verify_enterprise_release.sh
```

**Final audit run (2026-04-26):** `git diff --check`, `verify_workflow_contracts.sh`, and `verify_github_workflow_cicd_contract.py` **passed**. `verify_github_governance.sh` **skipped API checks** without a token (**expected** locally). Repository tree contained **no** `__pycache__` or `*.pyc` files.

---

## Remaining manual GitHub UI tasks

- **Branch protection** on `develop` / `main`: required status checks aligned with `tools/verify_github_governance.py` recommendations (CI, Security, Enterprise release verification, etc.).
- **`production` environment**: deployment branches (**main** only), **required reviewers**, optional wait timer — see `docs/runbooks/github-governance.md`.
- **Repository variables** and **Environments** (GitHub **Settings**): set feature flags and integration endpoints referenced by workflow `vars` (see [docs/runbooks/github-governance.md](docs/runbooks/github-governance.md) and [docs/runbooks/supply-chain-security.md](docs/runbooks/supply-chain-security.md) for policy-related names).
- **Artifact retention:** Build / Security Release artifacts must remain available long enough for downstream resolution.
- **Org toggles:** `ENABLE_DEPENDENCY_REVIEW`, `ENABLE_CODE_SCANNING` — document intentional skips when not `true`.

---

## Expected GitHub Actions sequences

### PR opened / updated (target `develop` or `main`)

1. **CI** — full PR pipeline.  
2. **Security** — govulncheck (PR), secret/config scans; Dependency Review if enabled.  
3. **CodeQL** — if enabled via variable.  
4. **No** Build (unless a separate manual dispatch of Build exists — not the normal PR path).  
5. **No** Security Release / **no** deploy.

### Push to `develop`

1. **CI** (push)  
2. **Security** (push)  
3. **Build and Push Images** — when **CI** `workflow_run` completes successfully for an eligible push/release candidate.  
4. **Security Release** — when **Build** completes.  
5. **Staging Deployment Contract** — when **Security Release** completes successfully on **`develop`**; deploy steps gated on **`verdict=pass`**, **`source_branch=develop`**, digest policy, and org toggles.

### Push to `main`

Same as develop through step 4; then:

5. **Deploy Production** — **manual** **`workflow_dispatch` only** on **`main`**. The operator must supply the Build run id, Security Release run id, and digest-pinned image refs; gates require **`verdict=pass`**, **`source_branch=main`**, and related evidence (this workflow does not start automatically when **Security Release** finishes).

---

## Known limitations (honest)

- **Repository code does not set GitHub branch protection, rulesets, or environment protection.** Those are **UI (or org policy) only**. A malicious or mistaken admin could weaken them unless org governance backs the repo. This audit documents required behavior; the owner must still configure GitHub.  
- **GitHub configuration** (branch protection, environment reviewers) is **not** fully verifiable offline; `verify_github_governance.py` is best-effort and prints a **manual checklist** when the token is missing or the API is insufficient.  
- **CodeQL** and **Dependency Review** may be **skipped** by design when repository variables are not `true` — this is **visible** in workflow `if:` conditions, not a silent bypass.  
- **Automatic image rollback** in production **does not** reverse database migrations.  
- **Action version tags** (`@v4`, etc.) are used instead of immutable SHAs by default — optional `VERIFY_ACTION_SHA_PINNING=1` lists candidates for hardening.

---

## Rollback and triage

| Situation | Action |
|-----------|--------|
| **Security Release** failure or **verdict=fail** | Fix findings; new Build + Security Release; **no** deploy from failed gate. See `docs/runbooks/security-release-failure.md`. |
| **skipped** / **no-candidate** | Not deployable; fix upstream candidate/Build eligibility. Deploy workflows **fail closed** if invoked with non-pass verdict. |
| **Production deploy** failure | Automatic rollback when LKG digests exist; else manual **`workflow_dispatch` rollback**. See `docs/runbooks/rollback-production.md`, `docs/runbooks/deploy-failure.md`. |
| **Staging** issues | `docs/runbooks/staging-release.md`, `docs/runbooks/deploy-failure.md`. |
| **Nightly rescan** findings | Informational / triage; **no** auto-promotion. `docs/runbooks/security-rescan.md`. |

---

## Optional improvements (non-blocking)

- Run **`verify_github_governance.sh`** on a schedule with a read-scoped token for drift detection.  
- Enable **Dependency Review** and **CodeQL** where org policy allows.  
- Consider **SHA-pinned** actions after verifying tag-to-commit mapping (`VERIFY_ACTION_SHA_PINNING=1`).

---

## Acceptance checklist (final audit)

| Criterion | Status |
|-----------|--------|
| `bash scripts/ci/verify_workflow_contracts.sh` passes | **OK** (2026-04-26) |
| `python tools/verify_github_workflow_cicd_contract.py` passes | **OK** |
| `bash scripts/ci/verify_github_governance.sh \|\| true` acceptable locally | **OK** (skips without token) |
| `git diff --check` passes | **OK** |
| No deploy path bypasses **Security Release** | **OK** |
| **Security Release** is the only deploy approval artifact source | **OK** |
| No `__pycache__` / `*.pyc` in tree | **OK** (audit snapshot) |
| This document matches repository behavior | **OK** — to be re-run after major workflow changes |

---

*This file is the consolidated enterprise CI/CD audit for the AVF vending backend as of the audit date above. Re-validate after material workflow or policy changes.*
