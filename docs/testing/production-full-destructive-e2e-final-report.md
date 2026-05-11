# Production full destructive E2E — final report

**Generated:** 2026-05-05 (UTC, agent pass)

**Branch:** `test/production-full-destructive-e2e`

**Production base URL:** `https://api.ldtv.dev`
**Commit (this documentation):** see `git rev-parse HEAD` on `test/production-full-destructive-e2e` (deliverable message: `test(e2e): run full destructive production validation`).

## Destructive mode confirmation (harness)

The harness enforces, when `E2E_TARGET=production` and `E2E_ALLOW_WRITES=true`:

- `E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`

When `E2E_ALLOW_DESTRUCTIVE=true` (same target + writes):

- `E2E_PRODUCTION_DESTRUCTIVE_CONFIRMATION=I_UNDERSTAND_DB_WILL_BE_RESET_AFTER_TEST`

**Irreversible external gates** (default **false** in `load_env`): `E2E_ALLOW_REAL_PAYMENT`, `E2E_ALLOW_REAL_DISPENSE`, `E2E_ALLOW_REAL_MACHINE_COMMANDS`, `E2E_ALLOW_EXTERNAL_NOTIFICATIONS`.

## Operator env (never commit)

- **Tracked template:** `tests/e2e/.env.production.destructive.example`
- **Local secrets file (gitignored):** `tests/e2e/.env.production.destructive.local` — copy from example; quote values with spaces.
- Fill **ADMIN_TOKEN** or **ADMIN_EMAIL** + **ADMIN_PASSWORD**, plus **MACHINE_***, **MQTT_***, **GRPC_*** as needed before Steps 5–12.

## Commands executed (automated agent)

| Step | Command | Result |
|------|---------|--------|
| 0 | Template + `.gitignore` + harness | **Done** |
| 1 | `./scripts/ci/verify_e2e_assets.sh` | **PASS** |
| 1 | `git diff --check` | **PASS** |
| 2 | Production GET probes `https://api.ldtv.dev/health/live|ready|version` | **200 / 200 / 200** |
| 2 | `E2E_ENV_FILE=… E2E_TARGET=production E2E_ALLOW_WRITES=false ./tests/e2e/run-rest-local.sh --readonly` | **PASS** |
| 3 | `run-flow-review.sh --static-only` (writes forced false via CLI over env file) | **PASS** |
| 4–16 | Web Admin / Vending / gRPC / MQTT / full `run-all-local` / reuse / cleanup / P0–P1 audit / Newman | **Not run** — **no production credentials** in agent environment |

## Run directories (examples from this pass; under `.e2e-runs/`, gitignored)

- Read-only REST: `run-20260505T162749Z-9943-26082`
- Static flow review: `run-20260505T162813Z-10133-14627`

## Blockers for completing Steps 5–16

1. **Secrets:** Set `ADMIN_TOKEN` (or email/password) and machine/MQTT/gRPC values in `tests/e2e/.env.production.destructive.local`.
2. **gRPC TLS:** `GRPC_ADDR=api.ldtv.dev:443` may require additional `grpcurl` TLS flags or corporate trust store — validate with `grpcurl` from an operator workstation.
3. **Newman / coverage:** Run when collection + env paths point at production-safe folders (or excluded requests); install `newman` if missing.

## Reports / artifacts (when a full run completes)

Expect under `.e2e-runs/run-*/` and `reports/`: `summary.md`, `remediation.md`, `improvement-summary.md`, `optimization-backlog.md`, `flow-review-scorecard.json`, `coverage.json`, `test-data.json`, `test-data.redacted.json`, `test-events.jsonl`, `improvement-findings.jsonl`.

## Cleanup runner

- **Script:** `tests/e2e/run-cleanup-production-e2e.sh`
- **Current behavior:** logs a **P2** cleanup-gap finding (no automated delete/archive flow yet). Safe for CI; operators still rely on **DB reset** or manual teardown.

## Harness changes in this pass (summary)

- `load_env`: preserve CLI **`E2E_ALLOW_WRITES`** over file (read-only smoke + destructive template).
- Defaults + export for destructive / real-payment / notification flags.
- `e2e_target_safety_guard`: production **destructive** confirmation.
- `e2e_append_test_event`: validate `--argjson` payload (avoids Windows/subshell empty JSON).
- **Quoted** namespace strings in `.env.production.destructive.example`.
- `verify_e2e_assets.sh`: requires `run-cleanup-production-e2e.sh` + grep for destructive confirmation string.

## Exact operator rerun commands (after filling `.local`)

```bash
./scripts/ci/verify_e2e_assets.sh

E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  E2E_TARGET=production E2E_ALLOW_WRITES=false E2E_ENABLE_FLOW_REVIEW=true \
  ./tests/e2e/run-rest-local.sh --readonly

E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  E2E_TARGET=production E2E_ALLOW_WRITES=false E2E_ENABLE_FLOW_REVIEW=true \
  ./tests/e2e/run-flow-review.sh --static-only

# Destructive (requires confirmations + creds in .local):
E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  E2E_ENABLE_FLOW_REVIEW=true E2E_FAIL_ON_P0_FINDINGS=true E2E_FAIL_ON_P1_FINDINGS=false \
  ./tests/e2e/run-web-admin-flows.sh --fresh-data --full

latest_run="$(ls -dt .e2e-runs/run-* | head -n1)"

E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  ./tests/e2e/run-vending-app-flows.sh --rest-equivalent --reuse-data "$latest_run/test-data.json"

E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  ./tests/e2e/run-grpc-local.sh --reuse-data "$latest_run/test-data.json"

E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  ./tests/e2e/run-mqtt-local.sh --reuse-data "$latest_run/test-data.json"

E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  ./tests/e2e/run-all-local.sh --reuse-data "$latest_run/test-data.json"

E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  ./tests/e2e/run-all-local.sh --fresh-data

# …then reuse + cleanup as in your playbook
./tests/e2e/run-cleanup-production-e2e.sh --reuse-data "$latest_run/test-data.json"
```

## P0 / P1 / P2 / P3 (this agent pass)

- **Static flow review artifact sample:** 18 lines in one run (includes markers/debt — not a full-suite audit).
- **Full-suite counts:** **N/A** until Steps 5–14 run.

## DB reset

**Recommended** after operator completes destructive validation on production, per org policy — external PSP/hardware/SMS effects are **not** undone by DB reset alone.

## Backend redeploy

**Not required** for this harness/docs-only pass. Redeploy only if a failing E2E run proves a bug and a fix is merged.
