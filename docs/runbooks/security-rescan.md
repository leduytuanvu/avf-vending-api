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
| Release images | Trivy `image` on **digest-pinned** `app` and `goose` refs from the latest successful **Build and Push Images** run on `main` that still has a `promotion-manifest` artifact — same policy as **Security Release** |

## Artifacts

Typical artifact names (see the job summary for the exact run):

- `nightly-govulncheck-report`
- `nightly-gitleaks-report`
- `nightly-trivy-config-report`
- `nightly-dependency-snapshot`
- `nightly-image-candidate` (resolution JSON: `ok` vs `no-candidate`)
- `nightly-trivy-image-reports` (when an image candidate existed)

## Failure does **not** roll production back automatically

- A **red nightly run does not stop or revert** the currently deployed production stack by itself.
- **Deploy Production** is unchanged: it is **not** triggered by this workflow.
- Treat a failure as **signal**: triage severity, patch dependencies or images, cut a new build, and follow your normal **Security Release → deploy** path.

## No image candidate

If the workflow cannot resolve digest-pinned refs (for example **no recent main build**, **expired artifacts**, or API access issues), the **image rescan job fails** with a **no-candidate** summary. That is **not** a “clean” image scan — do not interpret it as a pass. Fix the underlying resolution problem and rerun.

## How to rerun

1. Open **Actions** → **Nightly Security Rescan** → **Run workflow** (workflow_dispatch).
2. Pick branch default (`main` is used by jobs explicitly where it matters).
3. After the run, download artifacts and use the job summary for quick orientation.

## Relation to **Security** (`security.yml`)

Push and pull request scans stay on **Security** (`security.yml`). **Nightly Security Rescan** adds **scheduled** and **manual** coverage on **main** plus **post-release image** rescans without replacing PR/push gates.
