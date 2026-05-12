# Production Proof Report

## Target commit

- **Branch:** `security/goose-otel-fix`
- **Commit:** `3536e9e42536bcbf92e70052325b74c41a106ffc`

## Production read-only smoke

- **Status:** NOT_RUN
- **Reason:** `STAGING_BASE_URL`, `PRODUCTION_BASE_URL`, `PROD_BASE_URL`, and `BASE_URL_PROD` were all unset in the verification environment.
- **Next action:** Export a read-only-safe URL and run  
  `BASE_URL="$STAGING_BASE_URL" bash scripts/test/run-production-readonly-smoke.sh`  
  (or set `PRODUCTION_BASE_URL` / `PROD_BASE_URL` / `BASE_URL_PROD` per your convention.)

## CI / Security / Production Proof (exact commit)

- **Status:** **NOT_VERIFIED** for `3536e9e`
- **Evidence:** `gh run list --commit 3536e9e42536bcbf92e70052325b74c41a106ffc` returned **no runs**.
- **Why:** `ci.yml`, `security.yml`, and `production-proof.yml` run on **pull_request** or **push** to **`develop` / `main`** — not on ordinary pushes to `security/goose-otel-fix`. At verification time there was **no open PR** whose head was this branch (`gh pr list --head security/goose-otel-fix` was empty).
- **Last recorded green workflows (older SHA):** commit `3369b5129af2c8a49307272c6093f059cfd74c2e` — e.g. [CI](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25710985939), [Production Proof](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25710985923), [Security](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25710985930).
- **Commits after that SHA without CI on `3536e9e`:** `35e5278`, `3536e9e`.
- **Security Release (Trivy app/goose images):** runs after **Build and Push Images** on `develop`/`main` (`workflow_run`); **not** automatically re-run per feature-branch tip.

## Production canary / PSP / hardware

- **Production canary E2E:** BLOCKED — full canary env (`ALLOW_PROD_WRITES`, `PROD_WRITE_CONFIRMATION`, `CANARY_*`) not configured for this run.
- **PSP / provider:** BLOCKED — no sandbox/production PSP proof in scope.
- **Hardware:** BLOCKED — no canary machine / device evidence in scope.

## Local proof (reference only; not production)

- **Full local E2E:** pass (23 passed, 0 failed, 0 skipped — prior session; do not commit `.e2e-runs`).
- **Go test / govulncheck:** pass locally with correct DB env and `GOTOOLCHAIN=go1.25.10` (see earlier evidence).
- **REST OpenAPI live runner:** **partial** — 365 operations; **6** pass; **359** blocked (`reports/test/rest-full-live-coverage.json`). **Do not claim 100% REST live coverage.**

## Secret audit

- **Status:** pass — no live tokens or private keys in these committed reports.
