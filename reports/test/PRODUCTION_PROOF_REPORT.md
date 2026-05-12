# Production Proof Report

## Current proof target

| Field | Value |
|-------|--------|
| Branch | `security/goose-otel-fix` |
| Commit (tip) | `3fb3f281360354b498b3e160e10519c4aed8ff72` |
| PR | [#210 → `develop`](https://github.com/leduytuanvu/avf-vending-api/pull/210) |

## GitHub Actions (verified on head SHA)

| Workflow | Run | Conclusion |
|----------|-----|------------|
| CI | [25723652856](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25723652856) | success |
| Security | [25723652689](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25723652689) | success |
| Production Proof | [25723652868](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25723652868) | success |
| CodeQL | [25723652672](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25723652672) | skipped |

**PR required checks (representative):** Go CI Gates — pass; Linux race and contract gates — pass; Go Vulnerability Scan (govulncheck) — pass; Deployment and Config Scan (Trivy config) — pass; Secret Scan — pass. Dependency Review — skipped (repo toggle).

**Historical commit `3369b51` is not used as proof for this tip** — evidence above is for `3fb3f28` only.

## Security Release / image Trivy (app + goose)

**NOT_TRIGGERED for this PR event.** `build-push.yml` listens for successful **CI** on **push** to `develop`/`main` (not `pull_request` on a feature head). **Security Release** follows **Build and Push Images**. After merge to `develop`, confirm Build + Security Release for **published image** Trivy evidence.

## Production read-only smoke

**NOT_RUN** — `STAGING_BASE_URL`, `PRODUCTION_BASE_URL`, `PROD_BASE_URL`, `BASE_URL_PROD` were unset; `run-production-readonly-smoke.sh` was not executed.

## Canary / PSP / hardware

- **Production canary E2E:** BLOCKED — full canary env not configured.
- **PSP:** BLOCKED — no sandbox PSP proof in scope.
- **Hardware:** BLOCKED — no canary device proof in scope.

## REST OpenAPI live coverage

**PARTIAL** — 365 operations in spec; **6** pass / **359** blocked in `reports/test/rest-full-live-coverage.json`. **Do not claim 100% live REST verification.**

## Local reference (non-production)

Full local E2E 23 passed / 0 failed / 0 skipped (prior session). .e2e-runs not committed.

## Secret audit

Pass — no live tokens in these report files.
