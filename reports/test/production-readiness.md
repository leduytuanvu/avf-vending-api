# Production Readiness

- **Tip commit:** `3fb3f281360354b498b3e160e10519c4aed8ff72`
- **PR:** https://github.com/leduytuanvu/avf-vending-api/pull/210 (`security/goose-otel-fix` → `develop`)

## CI on current tip

- **CI / Security / Production Proof:** **PASS** on `3fb3f28` (runs 25723652856, 25723652689, 25723652868). CodeQL skipped by configuration.
- **Security Release + Trivy on built app/goose images:** **NOT_RUN** for this PR — triggers after merge via **Build and Push Images** on `develop`/`main`.

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

**BLOCKED:** CI proof on tip is **green**, but production smoke URL is missing, **image Security Release** was not part of this PR event, **canary/PSP/hardware** are unproven, and **REST** evidence remains **partial**.
