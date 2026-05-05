# E2E shell harness

Multi-protocol local runs under **`tests/e2e/`** complement:

- **Go/DB correctness:** [`docs/testing/local-e2e.md`](../../docs/testing/local-e2e.md) (`make test-e2e-local`)
- **Field pilot matrix:** [`docs/testing/field-test-cases.md`](../../docs/testing/field-test-cases.md)

## Safety (local vs production)

| Rule | Behavior |
|------|----------|
| Default target | **`E2E_TARGET=local`** (`load_env` / `.env.example`). |
| Local writes | **`E2E_ALLOW_WRITES=true`** by default so full flows work against a dev API; use **`E2E_ALLOW_WRITES=false`** for read-only smoke (e.g. `run-rest-local.sh --readonly`). |
| Production writes | Blocked unless **`E2E_TARGET=production`**, **`E2E_ALLOW_WRITES=true`**, and **`E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`** (see `e2e_target_safety_guard` in **`lib/e2e_common.sh`**). |
| Secrets | Real tokens live in **`.env`** (gitignored) and **`.e2e-runs/run-*/secrets.private.json`** (gitignored). **`test-data.json`** stores **masked** tokens via `e2e_save_token`; **`test-data.redacted.json`** is for triage. Never commit live secrets. |
| Artifacts dir | **`.e2e-runs/`** is gitignored. |
| Destructive / MQTT vs prod | Prefer local/staging for mutations; matrix **safety level** in **[`e2e-flow-coverage.md`](../../docs/testing/e2e-flow-coverage.md)** marks **prod-safe-test-machine-only** where relevant. |

## Coverage snapshot

Authoritative matrix: **[`docs/testing/e2e-flow-coverage.md`](../../docs/testing/e2e-flow-coverage.md)** (REST / gRPC / MQTT / Postman exclusions).

| Area | Harness |
|------|---------|
| **Web admin** | **WA-SETUP-01** (`01_web_admin_setup.sh`); **WA-CAT / WA-INV / WA-SUP / WA-RPT** (`10`–`13`). Maps to matrix **WA-002–016** (some rows **partial** / **planned**: deep RBAC user admin, full media upload pipeline, some command REST paths). |
| **Vending app** | gRPC **20–24**; machine REST **02–08**; Phase 8 **40–47**. |
| **Protocols** | REST, gRPC, MQTT. |
| **Postman** | **`coverage-from-postman.py`** maps requests to the matrix or the **exclusions** table. |

## Artifacts (orchestrated run)

Under **`.e2e-runs/run-*`**: **`run-meta.json`**, **`events.jsonl`**, **`test-data.json`**, **`test-events.jsonl`** (created at data init, then filled by modules that call `e2e_append_test_event`), **`improvement-findings.jsonl`**, **`secrets.private.json`**, **`rest/`**, **`grpc/`**, **`mqtt/`**, **`reports/summary.md`**, **`reports/remediation.md`**, **`reports/coverage.json`**, **`reports/improvement-summary.md`**, **`reports/optimization-backlog.md`**, **`reports/flow-review-scorecard.json`** (these three are always written at finalize, even when empty), **`test-data.redacted.json`**, optional **`reports/e2e-junit.xml`**, and mirrors **`improvement-summary.md`**, **`optimization-backlog.md`**, **`flow-review-scorecard.json`** at the run root. On failure, stderr prints **`E2E_RUN_DIR`**; start with **`reports/remediation.md`**. For API/contract debt, see **`reports/improvement-summary.md`** even when the run exits **0**.

## Quality (offline, no API)

From the repo root — validates every `tests/e2e/**/*.sh` with `bash -n`, runs `py_compile` on `tests/e2e/tools/*.py`, checks JSON under `tests/e2e/data/`, scenario contracts, `.gitignore` for `.e2e-runs/`, and heuristics for committed secrets. **shellcheck** and **lychee** / **markdown-link-check** run when installed (otherwise skipped with a warning).

```bash
./scripts/ci/verify_e2e_assets.sh
# or
make verify-e2e-assets
```

## Documentation

| Doc | Purpose |
|-----|---------|
| [`docs/testing/e2e-flow-coverage.md`](../../docs/testing/e2e-flow-coverage.md) | Flow ↔ protocol matrix + Postman exclusion table |
| [`docs/testing/e2e-local-test-guide.md`](../../docs/testing/e2e-local-test-guide.md) | Prerequisites, `.e2e-runs/` |
| [`docs/testing/e2e-test-data-guide.md`](../../docs/testing/e2e-test-data-guide.md) | Seeds, idempotency |
| [`docs/testing/e2e-troubleshooting.md`](../../docs/testing/e2e-troubleshooting.md) | Common failures |
| [`docs/testing/e2e-remediation-playbook.md`](../../docs/testing/e2e-remediation-playbook.md) | Structured fixes |

## Run orchestration

From the repository root:

```bash
./tests/e2e/run-all-local.sh --fresh-data
E2E_ENABLE_FLOW_REVIEW=true ./tests/e2e/run-all-local.sh --fresh-data
E2E_FAIL_ON_P1_FINDINGS=true ./tests/e2e/run-all-local.sh --reuse-data path/to/.e2e-runs/run-*/test-data.json
./tests/e2e/run-rest-local.sh --readonly
./tests/e2e/run-grpc-local.sh
./tests/e2e/run-mqtt-local.sh
./tests/e2e/run-web-admin-flows.sh --full
./tests/e2e/run-vending-app-flows.sh --rest-equivalent
```

With **flow review** enabled (default), each finished run directory should contain **`improvement-findings.jsonl`** (may be empty; JSON Schema **`tests/e2e/data/improvement-finding.schema.json`**), and after **`e2e_finalize_reports`**: **`improvement-summary.md`**, **`optimization-backlog.md`**, **`flow-review-scorecard.json`** under **`reports/`** and at the run root.

Common flags: `--reuse-data path/to/test-data.json`, `--fresh-data`, `-h`.

### Flow improvement logger env (optional)

Set in **`tests/e2e/.env`** or the shell: **`E2E_ENABLE_FLOW_REVIEW`**, **`E2E_WARN_SLOW_MS`**, **`E2E_FAIL_ON_P0_FINDINGS`**, **`E2E_FAIL_ON_P1_FINDINGS`** (see **`tests/e2e/.env.example`**). **`improvement-summary.md`**, **`optimization-backlog.md`**, and **`flow-review-scorecard.json`** are always written at finalize (may be empty).

## Postman / Newman (Phase 9)

### Prerequisites

- **Newman:** `npm install -g newman` (or use `npx newman`).
- **Collection / env:** set `POSTMAN_COLLECTION` and `POSTMAN_ENV` in `tests/e2e/.env` (see `.env.example`). The default collection filename is `docs/postman/avf-vending-api-function-path.postman_collection.json` (same content as the **Public** requests in `avf-vending-api.postman_collection.json` until you replace it with a fuller OpenAPI import).

### Run Newman

Writes **`rest/newman-cli.log`**, **`rest/newman-report.json`**, **`rest/newman-junit.xml`** under the active **`E2E_RUN_DIR`** (or a standalone `.e2e-runs/newman-*` dir).

```bash
# After a normal E2E run dir exists (or export E2E_RUN_DIR):
export E2E_RUN_DIR=.e2e-runs/run-…
export POSTMAN_COLLECTION=docs/postman/avf-vending-api-function-path.postman_collection.json
export POSTMAN_ENV=docs/postman/avf-local.postman_environment.json
export E2E_ALLOW_WRITES=false   # only run folder "Public" when present
./tests/e2e/postman/run-newman.sh
```

- **`E2E_ALLOW_WRITES!=true`:** Newman is invoked with **`--folder Public`** if that folder exists (safe smoke).
- **`E2E_TARGET=production`** and **`E2E_ALLOW_WRITES=true`:** requires **`E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`** before running (matches other E2E writers). The collection’s prerequest script also blocks production mutations unless Postman env unlock vars are set.

If **newman** is not installed, **`run-newman.sh` exits 0** and appends remediation lines to **`rest/newman-cli.log`** (and an **`events.jsonl`** skip when **`E2E_RUN_DIR`** is set).

### Generate Postman environment from `test-data.json`

Maps capture file + `secrets.private.json` into Postman variables (`base_url`, `admin_token`, `machine_token`, `organization_id`, `site_id`, `machine_id`, `product_id`, `order_id`, `slot_id`, `allow_production_writes`, plus `allow_mutation` / `allow_production_mutation` / `confirm_production_run` for the collection scripts).

```bash
export E2E_RUN_DIR=.e2e-runs/run-…
export BASE_URL=http://127.0.0.1:8080
./tests/e2e/postman/generate-local-env.sh --out /tmp/avf-generated.postman_environment.json "$E2E_RUN_DIR"
newman run "$POSTMAN_COLLECTION" -e /tmp/avf-generated.postman_environment.json
```

### Coverage gate (matrix mapping)

Lists every request in the collection, compares normalized paths to **[`docs/testing/e2e-flow-coverage.md`](../../docs/testing/e2e-flow-coverage.md)**, and writes JSON with **`total_requests`**, **`covered_requests`**, **`uncovered_requests`**, **`excluded_requests`**.

```bash
python3 tests/e2e/postman/coverage-from-postman.py \
  --collection docs/postman/avf-vending-api-function-path.postman_collection.json \
  --matrix docs/testing/e2e-flow-coverage.md \
  --out reports/coverage-postman.json
```

- **Excluded** requests match the **Postman / Newman coverage exclusions** table in the matrix doc (`path`, `prefix`, `name`, or `regex`).
- **Critical gap:** mutating **`POST`/`PUT`/`PATCH`/`DELETE`** under **`/v1/admin`**, **`/v1/commerce`**, **`/v1/setup`**, **`/v1/device`**, or **`/v1/machines`** must be either **covered** (path appears in the matrix) or **excluded** with a reason; otherwise the script exits **`1`**.

`./tests/e2e/run-rest-local.sh` (without **`--readonly`**) invokes **`postman/run-newman.sh`** (honours **`E2E_ALLOW_WRITES`**) and then runs the coverage script into **`reports/coverage-postman.json`** when **`python3`** is available.

### Adding a new API to the test matrix

1. Add or import the Postman request (or document the canonical path in OpenAPI).
2. Add or extend a **Matrix** row in **`e2e-flow-coverage.md`** (`endpoint_or_rpc_or_topic` column) so the normalized path can be matched.
3. If the request must **not** be treated as a business flow (duplicate, optional scrape, template URL), add a row to **Postman / Newman coverage exclusions** with **`kind`** and **`reason`**.
4. Re-run **`coverage-from-postman.py`** and fix any **`uncovered_critical`** list.

## Data templates (examples)

- `data/seed.local.example.json` — initial fictional seed
- `data/reusable-test-data.example.json` — captured IDs after success
- `data/test-data.schema.json` — JSON Schema for capture file
