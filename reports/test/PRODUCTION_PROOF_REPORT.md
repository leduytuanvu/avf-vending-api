# Production Proof Report

## Current proof target

| Field | Value |
|-------|--------|
| Branch | `security/goose-otel-fix` |
| Commit (tip) | `35a699603143b5048d01b2d8e5defc279472488d` |
| PR | [#210 → `develop`](https://github.com/leduytuanvu/avf-vending-api/pull/210) |
| Merge status | **NOT_MERGED** — `mergeStateStatus` BLOCKED; `reviewDecision` REVIEW_REQUIRED (`gh pr merge 210 --merge` rejected by branch policy) |

## GitHub Actions (verified on head SHA)

| Workflow | Run | Conclusion |
|----------|-----|------------|
| CI | [25725682062](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682062) | success |
| Security | [25725682070](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682070) | success |
| Production Proof | [25725682100](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682100) | success |
| CodeQL | [25725682090](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682090) | skipped |

**PR required checks (representative):** Go CI Gates — pass; Linux race and contract gates — pass; Go Vulnerability Scan (govulncheck) — pass; Deployment and Config Scan (Trivy config) — pass; Secret Scan — pass. Dependency Review — skipped (repo toggle).

## Post-merge (develop)

**NOT_RUN** — merge to `develop` did not complete in this session; **Build and Push Images**, **Security Release**, **Published Image Vulnerability Scan**, and **Security Release Signal** were not observed.

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

## Final claim

**BLOCKED:** Required PR checks are green on `35a6996`, but **merge did not land**; **post-merge** image security gates are **unverified**. Production smoke **NOT_RUN**; canary/PSP/hardware blocked; REST live coverage **partial (6/365)** — no full REST claim.
