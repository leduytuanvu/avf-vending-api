# Production Readiness

- **Tip commit:** `35a699603143b5048d01b2d8e5defc279472488d`
- **PR:** https://github.com/leduytuanvu/avf-vending-api/pull/210 (`security/goose-otel-fix` → `develop`)
- **Merge:** **NOT_MERGED** — required review / branch policy blocked `gh pr merge`.

## CI on current tip

- **CI / Security / Production Proof:** **PASS** on `35a6996` (runs [25725682062](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682062), [25725682070](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682070), [25725682100](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682100)). CodeQL skipped by configuration.
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

**BLOCKED:** PR checks **green** on `35a6996`, but **merge blocked**; production smoke URL missing; **post-merge** image Security Release unverified; **canary/PSP/hardware** unproven; **REST** evidence **partial (6/365)**.
