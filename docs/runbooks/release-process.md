# Release process (main → Security Release → production)

**For the full branch flow (develop, main) and triage, start at** [cicd-release.md](./cicd-release.md).

This runbook describes **release artifacts** for the AVF vending API backend: what is produced, where it lives, and how it chains into production deploy.

**Which workflow is which (deploy vs legacy pointer):** see [github-governance — Active GitHub Actions workflows](github-governance.md#active-github-actions-workflows-in-this-repository) — use **`deploy-prod.yml`** for production, **`deploy-develop.yml`** for staging; do not use **`deploy-production.yml`** to deploy (pointer only).

## Chain overview (deterministic order)

**Pull request (to `develop` / `main`):** **CI** and repo **Security** only — no images, no deploy.

**Push to `develop` or `main`:**

```text
CI (success) → Security → Build and Push Images → Security Release → Deploy
```

1. **CI** and **Security** run on the push (in parallel; both must pass policy you enforce on the branch).
2. **Build and Push Images** runs only after **CI** completes successfully (`workflow_run` from **CI**), and only for **push** to `develop`/`main` (or manual `workflow_dispatch` to those branches). It publishes digest-pinned image metadata, **promotion-manifest**, **image-metadata**, **release-candidate** (audit `release-candidate.json`), **sbom-reports**, signing evidence, and related artifacts. **Releasable coordinates** (commit SHA, branch, image digests) come from these Build artifacts, not from ad-hoc `gh` “latest by head” searches on the happy path.
3. **Security Release** runs when **Build and Push Images** **completes** (`types: [completed]`). For automatic runs it uses **`github.event.workflow_run.id`** = that Build run id, downloads **promotion-manifest**, **image-metadata**, **release-candidate**, and related artifacts from it, and writes **security-verdict** JSON (with **`build_release_evidence`** and the same digest-pinned image refs the scans used). It does **not** trigger on repo **Security** completion.
4. For **`source_branch == main`**, Security Release also generates a **release manifest** (machine-readable) and Markdown summary, uploaded as the **release-manifest** artifact **before** any production deploy consumes that run.
5. **Deploy** (**Staging** / **Develop** or **Deploy Production**) runs only from **Security Release** `workflow_run` **completed** (and production still needs environment approval, manual deploy inputs, etc.). It consumes **security-verdict** (and thus the Build run id and image refs embedded there), **not** a guessed Build from `head_sha` on the automatic path. After a real staging deploy, **staging-smoke-evidence** records HTTP liveness/readiness and optional probes (`tools/smoke_test.py`); production promotion still uses **main** + **Security Release** only — there is no repo automation that auto-promotes from develop smoke to production.
6. **Deploy Production** downloads the **release-manifest** artifact from the authorizing Security Release run when present, runs the rollout, then **appends** deployment outcomes and uploads **release-manifest-post-deploy**.

## Artifacts

| Artifact | Workflow | Contents |
|----------|----------|----------|
| `security-verdict` | Security Release | `security-reports/security-verdict.json` |
| `release-manifest` | Security Release | `release-reports/release-manifest.json`, `release-reports/release-summary.md` |
| `sbom-reports` | Build and Push Images | CycloneDX JSON (app + goose); referenced from the manifest |
| `release-candidate` | Build and Push Images | `release-candidate.json` (commit, branch, build run id, digest-pinned app/goose refs, repo) |
| `promotion-manifest` | Build and Push Images | Coordinates + embedded SBOM metadata |
| `release-manifest-post-deploy` | Deploy Production | Updated manifest + summary after rollout |
| `staging-smoke-evidence` | Staging Deployment Contract (`deploy-develop.yml`) | `smoke-reports/smoke-test.json` (schema `smoke-test-v1`), `staging-smoke-report.json`, `staging-smoke-summary.md` — post-deploy HTTP/optional dependency probes; **fails the staging job** if required checks fail. Use as human evidence before production; **not** an automatic production gate. |

### `security-reports/security-verdict.json` (from `security-verdict` artifact)

**Security Release** is the only workflow that publishes **`security-verdict`**. The file is the machine-readable contract for the gate. **`verdict`** is **`pass`**, **`fail`**, **`skipped`**, or **`no-candidate`**. It must also include **`release_gate_verdict`**, **`release_gate_mode`**, **`repo_security_verdict`**, **`repo_release_verdict`**, **`published_image_verdict`**, **`provenance_release_verdict`**, probe fields (**`source_sha`**, **`source_branch`**, **`source_build_run_id`**, **`source_workflow_name`**), image refs (**`app_image_ref`**, **`goose_image_ref`**, with detail under **`published_images`**), **`build_release_evidence`** (from Build **`release-candidate.json`** when the signal downloaded it), optional **`sbom`**, **`security_workflow_run_id`**, **`generated_at_utc`**, **`failure_reasons`**, and **`job_results`**. **Deploy** workflows treat **`verdict: pass`** as the only deployable outcome; other verdict values may still result in a **successful** Security Release run (artifact + summary) so operators get a full record.

## Release manifest fields (v1)

The JSON schema is versioned as `schema_version: release-manifest-v1`. Important fields:

- **release_id** — Stable id derived from branch, commit short SHA, Build run id, and Security Release run id.
- **source_branch**, **source_sha**, **source_commit_message** (first line / subject when available).
- **build_run_id**, **security_release_run_id**.
- **app_image_ref**, **goose_image_ref** — Digest-pinned refs when the release candidate produced images.
- **sbom_artifact** — Pointer metadata for the `sbom-reports` artifact (from verdict or promotion-manifest).
- **security_verdict**, **release_gate_verdict** — From `security-verdict.json`.
- **migration_safety_verdict** — Resolved from the latest successful **CI** run for the same `source_sha` and the **Migration Safety Check** job when possible; otherwise `unknown`.
- **smoke_test_verdict** — `null` / not run until post-deploy; updated after production deploy.
- **environment_target** — `production-candidate` at Security Release; `production` after deploy append.
- **rollback_candidate** — Previous digest-pinned refs when known (filled from production deployment manifest after deploy).
- **deployment** — Populated only in **release-manifest-post-deploy** (deployed_at, hosts, health, smoke, rollback).

## Policy

- **No secrets** are written into the manifest (no tokens, keys, or workflow secrets).
- **GitHub org/repo governance** (who may push to `main`, required reviews, the **`production`** environment’s required approvers) is **not** enforceable from this repository’s code. Configure **Settings → Branches** and **Settings → Environments** as described in [github-governance.md](./github-governance.md). Run `scripts/ci/verify_github_governance.sh` with a read token in CI to detect drift; `ENFORCE_GITHUB_GOVERNANCE=true` may fail the job if policy is clearly missing in the API.
- **GitHub Releases** are not created by this automation unless separately adopted.
- **Enterprise chain (enforced offline by `tools/verify_github_workflow_cicd_contract.py` and `scripts/ci/verify_workflow_contracts.sh`):** on PRs, **CI** and repo **Security** only—workflows with `on.pull_request` must not set `environment: production|staging` or `uses: ./.github/workflows/deploy-…`. On push to `develop`/`main`, **CI** → **Security** → **Build and Push Images** (after successful push CI) → **Security Release** → **Deploy Develop** / **Deploy Production** (automatic paths only after **Security Release**). Failed checks print `CONTRACT VIOLATION:` with the workflow path, the violated rule, and expected remediation.
- **Deploy gates** consume the `security-verdict` artifact only from **Security Release** (`.github/workflows/security-release.yml`, workflow display name `Security Release`). The repo **Security** workflow (`security.yml`) does not authorize deploys and must not upload `security-verdict`.
- Staging and production deploy workflows **fail closed** when the Security Release verdict is not **pass** (including `skipped`, `no-candidate`, and `fail`).
- Production deploy still requires existing gates (verdict **pass**, `main`, digest-pinned refs, confirmations).
- Verdict semantics and artifact contract: [security-release.md](./security-release.md).
- Production **environment** protection, **LKG** selection, and **manual rollback** (image-only; no automatic DB down): [rollback-production.md](./rollback-production.md).

## Operator tips

- Use the **Security Release** job summary: it links **release-manifest**, **security-verdict**, and related artifact names.
- For a given production deploy, the **Deploy Production** job summary links **release-manifest-post-deploy** when the append step ran.
- **Optional webhooks:** when **`vars.NOTIFY_WEBHOOK_URL`** is set, **Security Release** (on signal job failure), **staging deploy** (`deploy-develop`), and **Deploy Production** call **`scripts/deploy/notify_deployment_status.sh`**. Omissions and webhook errors are non-fatal; see [deploy-failure.md](./deploy-failure.md#optional-notifications).
