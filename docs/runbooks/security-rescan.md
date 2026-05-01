# Security rescan (nightly)

The **Nightly Security Rescan** workflow (`.github/workflows/nightly-security.yml`) runs on a **daily schedule** and via **workflow_dispatch**. It re-runs the same classes of checks used in the release path so that **new CVEs** (or secrets/config regressions) are detected **after** an image or dependency set was previously approved.

## What runs

On branch **`main`** (checkout):

| Area | Tool / behavior |
|------|------------------|
| Go dependencies | `govulncheck ./...` (blocking — fails the job on findings) |
| Secrets | `gitleaks` with `.gitleaks.toml` (blocking) |
| Config / IaC | Trivy `config` with `trivy.yaml` (**HIGH** / **CRITICAL**, `ignore-unfixed: true`) |
| Modules inventory | `go list -u -m -json all` (report artifact) |
| Release images | Trivy `image` on digest-pinned `app` and `goose` refs from **`scripts/security/resolve_nightly_main_image_candidate.py`**, which lists recent successful **Build and Push Images** runs on **`main`** (**`gh api -X GET`** …/ **`build-push.yml`**`/runs`), downloads **`promotion-manifest`**, and requires **`source_branch`** **`main`**, semantic **`source_event`** **`push`** or **`workflow_dispatch`**, and digest **`app_ref`/`goose_ref`** or **`app_image_ref`/`goose_image_ref`** (CI **`workflow_run`‑wrapped** Builds qualify when the manifest says so). Same HIGH/CRITICAL policy as **Security Release**. |

## Artifacts

Typical artifact names (see the job summary for the exact run):

- `nightly-govulncheck-report`
- `nightly-gitleaks-report`
- `nightly-trivy-config-report`
- `nightly-dependency-snapshot`
- `nightly-image-candidate` (resolution JSON: `ok` vs `no-candidate`)
- `nightly-trivy-image-reports` (when an image candidate existed)

## No image candidate

If the workflow cannot resolve digest-pinned refs, the **image rescan job fails** with a **no-candidate** summary — **not** a clean scan pass.

Resolution walks recent **successful** **Build and Push Images** runs on **`main`** (via explicit **`gh api -X GET`** on `build-push.yml/runs`), inspects **`promotion-manifest`** per run, and records **`inspected_runs`** / **`inspected_run_count`** / **`reason`** in artifact **`nightly-image-candidate`** (`status: no-candidate`). Skips use structured reasons such as **missing promotion-manifest artifact**, **download failed**, **missing promotion-manifest.json**, **source_branch mismatch**, **unsupported source_event**, **missing digest-pinned refs**, or **`api_diagnostic`** text when the runs list API fails.

**Triage:** confirm **main** builds succeed with **`promotion-manifest`**, artifact retention, **`actions: read`** / **`packages: read`**, and promotion-manifest **semantic** fields (`source_event`, digest refs). Fix and rerun (scheduled or **workflow_dispatch**).

## production-deploy-candidate vs Deploy Production

The **`production-deploy-candidate`** artifact from **Security Release** may ship **`TODO_STAGING_EVIDENCE_RUN_ID`** for **`staging_evidence_id`**. **Deploy Production** requires a real **`staging_evidence_id`** from a successful **Staging Deployment Contract** run unless an operator intentionally sets **`allow_missing_staging_evidence=true`** **and** a non-empty **`missing_staging_evidence_reason`** (temporary bypass — not normal). The **`deploy-production-gh-command.sh`** helper refuses TODO/empty staging unless those bypass fields are present.

## Failure does **not** roll production back automatically

- A **red nightly run does not stop or revert** the currently deployed production stack by itself.
- **Deploy Production** is unchanged: it is **not** triggered by this workflow.
- Treat a failure as **signal**: triage severity, patch dependencies or images, cut a new build, and follow your normal **Security Release → deploy** path.

## How to rerun

1. Open **Actions** → **Nightly Security Rescan** → **Run workflow** (workflow_dispatch).
2. Pick branch default (`main` is used by jobs explicitly where it matters).
3. After the run, download artifacts and use the job summary for quick orientation.

## Relation to **Security** (`security.yml`)

Push and pull request scans stay on **Security** (`security.yml`). **Nightly Security Rescan** adds **scheduled** and **manual** coverage on **main** plus **post-release image** rescans without replacing PR/push gates.
