# Production Readiness

- **Tip commit:** `d7d28ecc5cb910b66ec2d80a069f4bf5a5f3cbc5`
- **PR:** https://github.com/leduytuanvu/avf-vending-api/pull/210 (`security/goose-otel-fix` → `develop`)
- **Merge:** **NOT_MERGED** — required review / branch policy blocked `gh pr merge`.

- **Latest head (post-report commit):** `f36e39ddd2455ba02b5d7ec5f783e232ec359f9b` — checks **PASS** (CI 25727355368, Security 25727355333, Production Proof 25727355354); **BEHIND** `develop` — rebase/merge **develop** before shipping.

## CI on current tip

- **CI / Security / Production Proof:** **PASS** on `d7d28ec` (runs [25726935875](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935875), [25726935841](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935841), [25726935862](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935862)). CodeQL skipped by configuration.
- **Security Release + Trivy on built app/goose images:** **NOT_RUN** — merge to `develop` did not complete; **Build and Push Images** was not triggered from this PR outcome.

## OpenAPI JSON and public metrics (expectations)

- **Public `GET /metrics` → 404** in production with default settings is **expected and safe** (metrics on **`HTTP_OPS_ADDR`**).
- **`GET /swagger/doc.json` → 404** means OpenAPI JSON is **not** enabled or the container was **not** restarted with the right env; when intentionally enabled, set **`HTTP_OPENAPI_JSON_ENABLED=true`** and **`PRODUCTION_OPENAPI_JSON_ALLOWED=true`** (and keep Swagger UI off unless explicitly allowed).
- **Production smoke** must **not** fail solely because public `/metrics` is **404** when **`SMOKE_EXPECT_PUBLIC_METRICS`** is unset/false — use `scripts/test/run-readonly-smoke.sh` / `run-production-readonly-smoke.sh` with documented flags (`docs/operations/production-openapi-and-metrics.md`).

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

**BLOCKED:** PR checks **green** on **`f36e39d`**, but branch is **BEHIND** `develop`; production smoke URL missing; **post-merge** image Security Release unverified; **canary/PSP/hardware** unproven; **REST** **partial (6/365)**.
