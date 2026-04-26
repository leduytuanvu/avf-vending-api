# Security Release failure triage

**Context:** [cicd-release.md](./cicd-release.md) (overall pipeline and failure routing).

**Security Release** (`.github/workflows/security-release.yml`) is the only workflow that produces **`security-verdict`** for staging and production. This runbook covers common failure modes when the workflow **fails** or yields a **non-pass** verdict.

## `security-verdict.json` (machine-readable)

Every **Security Release** run should write **`security-reports/security-verdict.json`** and upload the **`security-verdict`** artifact. The blocking field is **`verdict`**: **`pass`**, **`fail`**, **`skipped`**, or **`no-candidate`**. The same file must also include (for automation and the release dashboard) at least: **`release_gate_verdict`**, **`release_gate_mode`**, **`repo_security_verdict`**, **`repo_release_verdict`**, **`published_image_verdict`**, **`provenance_release_verdict`**, **`source_sha`**, **`source_branch`**, **`source_build_run_id`**, **`source_workflow_name`**, **`app_image_ref`**, **`goose_image_ref`**, **`security_workflow_run_id`**, **`generated_at_utc`**, **`failure_reasons`**, and **`job_results`**. Staging and production **deploy** workflows accept **pass** only.

## Repo `Security` vs optional checks (no silent “full pass”)

**Security Release** is separate from repo **Security** (`security.yml`). For **promotion and deploy evidence**, the release gate is **Security Release** + **`security-verdict`**, not “CodeQL skipped” or “Dependency Review skipped”.

- **Always blocking** when `security.yml` runs: **Go Vulnerability Scan**, **Secret Scan**, **Deployment and Config Scan** (PR + push to `develop`/`main` + `workflow_dispatch`). A skipped job here is **not** a policy success.
- **Intentionally off** until repo variables are set: **CodeQL** requires `ENABLE_CODE_SCANNING == 'true'`; **Dependency Review** requires `ENABLE_DEPENDENCY_REVIEW == 'true'` (PR-only). A **skipped** run of those is **not** a substitute for the three blocking jobs — see [github-governance.md](./github-governance.md).

## What “failure” means

- **Workflow failure (red X):** a required step exited non-zero (e.g. enforce step when `verdict=fail`, scan failure under enforce policy, missing artifact).
- **Verdict `fail`:** the release gate recorded a blocking outcome in **`security-reports/security-verdict.json`**; investigate **`failure_reasons`** in that file and the job summary sections **Security release signal — decision details** and **failure_reasons (blocking)**.
- **Verdict `skipped` / `no-candidate`:** not deployable; usually upstream **Build** was not a release candidate or the run was ineligible. This is **not** a successful release — deploy workflows **fail closed** if they see these. The workflow is still **green** when the verdict writer completed; the **enforce** step allows **`skipped`** / **`no-candidate`** to succeed so the artifact is uploaded and operators can see structured reasons.

## Quick checks

1. Open the failed run → **Summary** tab → **Release dashboard** (concise table: SHA, branch, image refs, verdict).
2. Download artifact **`security-verdict`** and inspect **`failure_reasons`**.
3. Confirm the triggering **Build and Push Images** run was **`push`** or **`workflow_dispatch`** on **`develop`** or **`main`** (not chain-only **`workflow_run`** as the Build’s GitHub event).
4. Confirm image refs are **digest-pinned** in the verdict and cosign/Trivy/provenance sections match org policy (**`SIGNING_ENFORCEMENT`**, **`PROVENANCE_ENFORCEMENT`**, etc.).

## Common causes

| Symptom | Likely cause | Direction |
|--------|----------------|-----------|
| Trivy / govulncheck / cosign **fail** | Policy set to enforce; CVE or signature mismatch | Fix dependency or image; or adjust policy only per org governance (do not weaken without review). |
| **No security-verdict artifact** | Early failure before write; enforce step may emit emergency verdict | Re-run after fix; check **Security Release Signal** job logs. |
| **Repo Security (push) not successful** | Signal step polls repo **Security** for the same `head_sha` / branch | Ensure a **Security** workflow run completed **success** for that commit on **develop**/**main**. |
| **Provenance / attestations** | Private repo fallback or attestations missing | See verdict `provenance_release_checks`; may be expected with **`ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK`**. |
| **Skipped / no-candidate** | Build did not produce a releasable candidate | Fix **upstream-ci-release-gate** / **Build** inputs; not a deploy path. |

## What not to do

- Do **not** deploy from repo **Security** (`security.yml`) or **Build** alone — only **Security Release** + **`verdict=pass`** authorizes deploy gates.
- Do **not** re-tag images to “pass” scans without fixing the underlying issue.
- Do **not** expect **nightly-security** or **nightly-ops** to replace **Security Release** for promotion.

## Related

- [security-release.md](./security-release.md) — workflow intent and artifacts.  
- [release-process.md](./release-process.md) — manifest and promotion chain.  
- [deploy-failure.md](./deploy-failure.md) — when Security Release passed but deploy failed.  
- [supply-chain-security.md](./supply-chain-security.md) — SBOM and signing context.
