# Production Proof Report

## Latest GitHub state (after documentation follow-up commit)

| Field | Value |
|-------|--------|
| headRefOid (2026-05-12) | `f36e39ddd2455ba02b5d7ec5f783e232ec359f9b` |
| mergeStateStatus | **BEHIND** — sync with `develop` before merge |
| Checks on head | **PASS** — CI [25727355368](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25727355368), Security [25727355333](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25727355333), Production Proof [25727355354](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25727355354), CodeQL [25727355319](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25727355319) (skipped) |

## Current proof target

| Field | Value |
|-------|--------|
| Branch | `security/goose-otel-fix` |
| Commit (tip) | `d7d28ecc5cb910b66ec2d80a069f4bf5a5f3cbc5` |
| PR | [#210 → `develop`](https://github.com/leduytuanvu/avf-vending-api/pull/210) |
| Merge status | **NOT_MERGED** — `mergeStateStatus` BLOCKED; `reviewDecision` REVIEW_REQUIRED (`gh pr merge 210 --merge` rejected by branch policy) |

## GitHub Actions (verified on head SHA)

| Workflow | Run | Conclusion |
|----------|-----|------------|
| CI | [25726935875](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935875) | success |
| Security | [25726935841](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935841) | success |
| Production Proof | [25726935862](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935862) | success |
| CodeQL | [25726935837](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935837) | skipped |

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

**BLOCKED:** Required PR checks are green (latest **`f36e39d`**), but **merge did not land** (**BEHIND** `develop` + review); **post-merge** image security gates **unverified**. Production smoke **NOT_RUN**; canary/PSP/hardware blocked; REST **partial (6/365)**.
