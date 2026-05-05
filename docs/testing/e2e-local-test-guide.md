# Local E2E test guide (shell harness)

This guide describes how **multi-protocol local E2E** runs work using `tests/e2e/run-*.sh` and `tests/e2e/lib/*.sh`. Database-heavy correctness tests still live under **[`local-e2e.md`](local-e2e.md)** (`make test-e2e-local`).

## Scope

- **REST:** Web Admin + machine routes from **[`docs/swagger/swagger.json`](../swagger/swagger.json)** and **[`docs/postman/`](../postman/)**
- **gRPC:** `proto/avf/machine/v1/*.proto` services (vending app)
- **MQTT:** **[`docs/api/mqtt-contract.md`](../api/mqtt-contract.md)** topic layouts

## Prerequisites

1. **API server** running locally (default `BASE_URL`).
2. **PostgreSQL** when scenarios assert DB state (same assumptions as [`local-e2e.md`](local-e2e.md)).
3. **bash**, **curl**, **jq** (required for all runners).
4. **grpcurl** (optional until `tests/e2e/scenarios/grpc_local.sh` exists; runner skips if absent).
5. **mosquitto_pub** / **mosquitto_sub** (optional until `tests/e2e/scenarios/mqtt_local.sh` exists; runner skips if absent).

## Environment file

1. Copy **`tests/e2e/.env.example`** → **`tests/e2e/.env`** (the `.env` file is gitignored at repo root; keep machine-specific secrets only under `.e2e-runs/`).
2. Variables are documented in `.env.example`, including:

| Variable | Purpose |
|----------|---------|
| `BASE_URL` | REST base (default `http://127.0.0.1:8080`) |
| `GRPC_ADDR` | gRPC host:port |
| `MQTT_HOST` / `MQTT_PORT` | Broker for MQTT scenarios |
| `POSTMAN_COLLECTION` / `POSTMAN_ENV` | Paths to Postman artifacts (relative to repo root) |
| `E2E_TARGET` / `E2E_ALLOW_WRITES` | Target guard (`production` + writes needs confirmation — see `e2e_common.sh`) |
| `E2E_REUSE_DATA` / `E2E_DATA_FILE` | Reuse capture file (env or overridden by CLI) |
| `ADMIN_TOKEN`, `MACHINE_TOKEN`, MQTT credentials | Secrets — never commit |
| `GRPC_PROTO_ROOT`, `GRPC_USE_REFLECTION` | gRPC / grpcurl |

**Postman paths:** `.env.example` references `docs/postman/avf-vending-api-function-path.postman_collection.json`. If that file does not exist in your tree yet, point `POSTMAN_COLLECTION` at an existing export such as `docs/postman/avf-vending-api.postman_collection.json`.

## Run directory layout

Each run writes artifacts under:

```text
.e2e-runs/run-<timestampUTC>-<pid>-<random>/
  run-meta.json
  events.jsonl
  test-data.json          # public capture; tokens stored masked
  secrets.private.json    # full tokens (local only)
  rest/                   # *.request.json, *.response.json, *.meta.json
  grpc/                   # *.request.json, *.response.json, *.log
  mqtt/                   # *.publish.json, *.publish.log, *.meta.json, *.subscribe.log
  reports/
    summary.md
    remediation.md
    coverage.json
```

The directory **`.e2e-runs/`** is gitignored.

## Commands

From the **repository root**:

```bash
./tests/e2e/run-all-local.sh
./tests/e2e/run-rest-local.sh
./tests/e2e/run-grpc-local.sh
./tests/e2e/run-mqtt-local.sh
./tests/e2e/run-web-admin-flows.sh
./tests/e2e/run-vending-app-flows.sh
```

Common options:

- **`--fresh-data`** — empty `test-data.json` for the run.
- **`--reuse-data PATH`** — copy capture JSON into `test-data.json`.
- **`-h` / `--help`**

`run-all-local.sh` order: **preflight** → REST → web-admin flows → vending-app flows → gRPC → MQTT → **reports**.

When `run-all-local.sh` invokes phase scripts, it sets **`E2E_IN_PARENT=1`** and reuses the same `E2E_RUN_DIR`. Phase scripts avoid duplicate report generation; the orchestrator writes **one** `reports/` set at the end.

Optional scenario files (not required for Phase 1):

- `tests/e2e/scenarios/preflight.sh`
- `tests/e2e/scenarios/rest_local.sh`
- `tests/e2e/scenarios/web_admin_flows.sh`
- `tests/e2e/scenarios/vending_app_flows.sh`
- `tests/e2e/scenarios/grpc_local.sh`
- `tests/e2e/scenarios/mqtt_local.sh`

## Library layout

| File | Role |
|------|------|
| `lib/e2e_common.sh` | strict mode, env, logging, steps, events, safety guard, CLI parsing |
| `lib/e2e_data.sh` | `test-data.json` + `secrets.private.json` |
| `lib/e2e_http.sh` | REST helpers + curl transcripts |
| `lib/e2e_grpc.sh` | grpcurl wrappers |
| `lib/e2e_mqtt.sh` | mosquitto wrappers |
| `lib/e2e_report.sh` | `summary.md`, `remediation.md`, `coverage.json`, console summary |

## Reuse vs fresh data

- **`--reuse-data PATH`** — same org/machine IDs across runs; faster regression.
- **`--fresh-data`** — after activation collisions, idempotency conflicts, or corrupted scratch state.

See **[`e2e-test-data-guide.md`](e2e-test-data-guide.md)** for entity hierarchy and cleanup.

## Logs and reports

- **Console:** phase lines from `log_info` / `log_warn`.
- **`events.jsonl`:** one JSON object per step outcome (`passed` / `failed` / `skipped`).
- **`reports/summary.md`:** human-readable rollup.
- **`reports/remediation.md`:** failed steps only; link to **[`e2e-remediation-playbook.md`](e2e-remediation-playbook.md)**.
- **`reports/coverage.json`:** machine-readable counts + full events array.

## Related

- **[`e2e-test-data-guide.md`](e2e-test-data-guide.md)**
- **[`e2e-flow-coverage.md`](e2e-flow-coverage.md)**
- **[`e2e-troubleshooting.md`](e2e-troubleshooting.md)**
- **[`tests/e2e/README.md`](../../tests/e2e/README.md)**
