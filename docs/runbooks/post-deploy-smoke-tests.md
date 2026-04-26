# Post-deploy smoke tests

After a staging or production deploy, GitHub Actions runs **read-only HTTP checks** from the workflow runner against the public API base URL. This verifies that the app is actually serving traffic (not only that containers restarted).

## What is exercised

| Check | Method | Expectation |
| --- | --- | --- |
| DNS (HTTPS only) | Resolver | Hostname resolves for port 443 before TLS requests. |
| Liveness | `GET /health/live` | HTTP 2xx; body contains `ok`. |
| Readiness | `GET /health/ready` | HTTP 2xx; body contains `ok`. |
| Version / build | `GET /version` | HTTP 2xx; body contains JSON fragment `"version"`. |
| Optional authenticated read | `GET {SMOKE_AUTH_READ_PATH}` | Only when token and path are both set; HTTP 2xx (no body assertion). |

TLS: for `https` URLs, Python uses the default verification context (certificate validation). Retries with exponential backoff apply to transient HTTP/network errors.

**Latency:** responses slower than `SMOKE_WARN_LATENCY_MS` (default 2000) produce warnings in the report and stderr summary; they do **not** fail the run unless combined with real failures.

## Configuration

### Required

- `BASE_URL` — origin only, e.g. `https://api.example.com` (no trailing slash required).
- `ENVIRONMENT_NAME` — label in the JSON report (e.g. `staging`, `production`).

### Optional

- `SMOKE_AUTH_TOKEN` — Bearer token for the optional read-only GET. **Never logged** by `tools/smoke_test.py`.
- `SMOKE_AUTH_READ_PATH` — path such as `/api/v1/example` (must be safe, idempotent GET).
- `SMOKE_MAX_ATTEMPTS` (default `5`), `SMOKE_BACKOFF_BASE_SEC` (default `2`), `SMOKE_REQUEST_TIMEOUT_SEC` (default `15`), `SMOKE_WARN_LATENCY_MS` (default `2000`).
- `SMOKE_REPORT_PATH` — override report path (wrapper passes `--report`).

### Local / manual run

From the repository root:

```bash
export BASE_URL="https://your-api.example.com"
export ENVIRONMENT_NAME="local"
bash scripts/deploy/smoke_test.sh
```

Report: `smoke-reports/smoke-test.json` (fields `overall` and `overall_status`).

## CI wiring

- **Staging** (`deploy-develop.yml`): after remote `scripts/smoke_staging.sh`, the runner invokes `scripts/deploy/smoke_test.sh`. Base URL comes from `STAGING_SMOKE_BASE_URL`, `STAGING_PUBLIC_BASE_URL`, or the origin of `STAGING_API_READY_URL`. Artifact: `staging-post-deploy-smoke`. If the smoke step fails, the job fails.
- **Production** (`deploy-prod.yml`): after final cluster smoke, the runner runs the same script against `PRODUCTION_PUBLIC_BASE_URL`. Artifact: `post-deploy-smoke-report`. On job failure, the existing **automatic rollback** path also treats a failed post-deploy smoke step like other rollout failures (see workflow `Attempt automatic rollback`).

## Adding authenticated smoke checks later

1. Choose a **read-only** route (GET only, no side effects, no PII in logs).
2. Store a token in the environment’s deployment secrets (e.g. `STAGING_SMOKE_AUTH_TOKEN` / `PRODUCTION_SMOKE_AUTH_TOKEN` — names already wired in workflows).
3. Set the repo variable for the path (e.g. `STAGING_SMOKE_AUTH_READ_PATH` / `vars.PRODUCTION_SMOKE_AUTH_READ_PATH`).
4. Workflows already mask the token with `::add-mask::` where the secret is injected.

If the token is set but `SMOKE_AUTH_READ_PATH` is empty, the tool skips the authenticated check and records a warning in the report.

## What failure means

- **Any required check returns non-2xx**, wrong body, or DNS does not resolve (for HTTPS): the smoke process exits non-zero, the workflow fails, and the JSON report lists failing checks with details.
- **Staging:** treat as a bad deploy; fix forward or redeploy; inspect the uploaded `staging-post-deploy-smoke` artifact.
- **Production:** the deploy job should fail; automatic rollback may run if previous image refs are available. If rollback cannot run, the workflow emits explicit errors — follow production runbooks and use digest-pinned rollback when needed.

The smoke suite is intentionally **non-destructive**: no POST/PUT/PATCH/DELETE, no admin-only mutations, and tokens are not printed.
