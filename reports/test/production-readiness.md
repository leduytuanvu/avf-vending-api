# Production Readiness

- **Tip commit:** `e204934d3975aa4a6afcab3a54d1c359c3fa8487`
- **PR:** https://github.com/leduytuanvu/avf-vending-api/pull/210 (`security/goose-otel-fix` → `develop`)
- **Merge:** **NOT_MERGED** — required review / branch policy blocked `gh pr merge`.

## CI on current tip

- **CI / Security / Production Proof:** **PASS** on `e204934` (runs [25726479840](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726479840), [25726479824](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726479824), [25726479817](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726479817)). CodeQL skipped by configuration.
- **Security Release + Trivy on built app/goose images:** **NOT_RUN** — merge to `develop` did not complete; **Build and Push Images** was not triggered from this PR outcome.

## Production read-only smoke

**NOT_RUN** — set `STAGING_BASE_URL` or `PRODUCTION_BASE_URL` / `PROD_BASE_URL` / `BASE_URL_PROD` and run `scripts/test/run-production-readonly-smoke.sh`.

## Canary / PSP / hardware

**BLOCKED** — environment not provided.

## REST live coverage

**PARTIAL (6/365)** — see `rest-full-live-coverage.json`; no full-coverage claim.

## Workflow triggers (reference)

| Workflow | `on` |
|----------|------|
| `ci.yml` | `pull_request` → `develop`/`main`; `push` → `develop`/`main`; `workflow_dispatch` |
| `security.yml` | same |
| `production-proof.yml` | `pull_request`; `workflow_dispatch` |
| `build-push.yml` | `workflow_run` after CI **success**; **branches** `develop`/`main` (feature PR CI does not enqueue this) |
| `security-release.yml` | `workflow_run` after **Build and Push Images** on `develop`/`main`; `workflow_dispatch` |

## Final claim

**BLOCKED:** PR checks **green** on `e204934`, but **merge blocked**; production smoke URL missing; **post-merge** image Security Release unverified; **canary/PSP/hardware** unproven; **REST** evidence **partial (6/365)**.
