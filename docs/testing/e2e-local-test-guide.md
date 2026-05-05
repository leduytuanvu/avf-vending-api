# Local E2E test guide (shell harness)

This guide describes how **multi-protocol local E2E** runs work using `tests/e2e/run-*.sh` and `tests/e2e/lib/*.sh`. Database-heavy correctness tests still live under **[`local-e2e.md`](local-e2e.md)** (`make test-e2e-local`).

## Scope

- **REST:** Web Admin (`/v1/admin/*`) + **machine-scoped** routes from **[`docs/swagger/swagger.json`](../swagger/swagger.json)** — used by the **vending REST-equivalent QA harness**; the **field vending app** in production uses **gRPC + MQTT** (see **`e2e-flow-coverage.md`**).
- **gRPC:** `proto/avf/machine/v1/*.proto` services (vending app)
- **MQTT:** **[`docs/api/mqtt-contract.md`](../api/mqtt-contract.md)** topic layouts

## Prerequisites

1. **API server** running locally (default `BASE_URL`).
2. **PostgreSQL** when scenarios assert DB state (same assumptions as [`local-e2e.md`](local-e2e.md)).
3. **bash**, **curl**, **jq**, **python3** (required for all runners and HTTP timing in `e2e_http.sh`).
4. **grpcurl** (required for **`run-grpc-local.sh`** Phase 6 machine contracts; optional for other phases).
5. **mosquitto_pub** / **mosquitto_sub** (required for a real Phase 7 run: **`run-mqtt-local.sh`** skips the MQTT suite with a logged reason if they are missing; **`00_preflight.sh`** only notes them as optional on PATH).

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
| `GRPC_PROTO_ROOT`, `GRPC_USE_REFLECTION` | gRPC: import root (defaults to repo **`proto/`**) vs server reflection |
| `MACHINE_ID`, `GRPC_SEND_MACHINE_ID_HEADER` | Optional `x-machine-id` on grpcurl calls |
| `E2E_ACTIVATION_CODE` | gRPC claim (**`20_grpc_machine_auth.sh`**) when no `machineToken` in secrets |

**Postman paths:** `.env.example` references `docs/postman/avf-vending-api-function-path.postman_collection.json`. If that file does not exist in your tree yet, point `POSTMAN_COLLECTION` at an existing export such as `docs/postman/avf-vending-api.postman_collection.json`.

## Run directory layout

Each run writes artifacts under:

```text
.e2e-runs/run-<timestampUTC>-<pid>-<random>/
  run-meta.json
  events.jsonl
  test-data.json          # capture used for the run (may include secrets — gitignored)
  test-data.redacted.json # same structure with token-like fields masked (safe to share in triage)
  secrets.private.json    # full tokens (local only)
  rest/                   # per-call: *.request.json, *.response.body, *.response.headers.txt, *.meta.json
                          # (older JSON mutations may also use *.response.json)
  grpc/                   # *.request.json, *.response.json, *.log, *.meta.json
  mqtt/                   # connect.log, telemetry.publish.json, command.subscribe.log, command.ack.json, …
  reports/
    summary.md            # human-readable rollup (REST/gRPC/MQTT/WA/VA/Phase 8 + coverage)
    remediation.md        # structured per-failure hints (failure_id, evidence path, rerun)
    e2e-report-context.json   # BASE_URL / GRPC_ADDR / MQTT / flags at finalize (no secrets)
    coverage.json         # merged Postman + gRPC + MQTT + Phase 8 + scenarioCoverage
    e2e-junit.xml         # JUnit from events.jsonl (optional CI ingest)
    grpc-contract-summary.md   # Phase 6 rollup (also appended to summary.md when run standalone)
    grpc-contract-results.jsonl
    mqtt-contract-summary.md   # Phase 7 rollup
    mqtt-contract-results.jsonl
  summary.md              # copy of reports/summary.md (orchestrator finalize)
  remediation.md          # copy of reports/remediation.md
  coverage.json           # copy of reports/coverage.json
  junit.xml               # copy of reports/e2e-junit.xml when present
```

**Note:** The canonical report paths are **`reports/*`**; root-level **`summary.md`**, **`remediation.md`**, **`coverage.json`**, and **`junit.xml`** are mirrors for simple globbing.

The directory **`.e2e-runs/`** is gitignored.

## Commands

```bash
BASE_URL=http://127.0.0.1:8080 E2E_TARGET=local E2E_ALLOW_WRITES=false \
  ./tests/e2e/run-rest-local.sh --readonly
```

This runs **`tests/e2e/scenarios/00_rest_readonly_smoke.sh`**: public **GET** checks only (`/health/live`, `/health/ready`, `/version`, optional `/swagger/doc.json` and `/metrics` with **404 → skipped**). No writes. Logs land under **`.e2e-runs/run-*/rest/`**; **`reports/summary.md`** lists every captured endpoint in **REST endpoints exercised**.

`run-all-local.sh` first sources **`tests/e2e/scenarios/00_preflight.sh`** (tooling, required env, same three health/version GETs as gate).

### Full orchestration

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

- **`--readonly`** (REST runner only) — read-only public GET smoke (`00_rest_readonly_smoke.sh`).
- **`--fresh-data`** — empty `test-data.json` for the run.
- **`--reuse-data PATH`** — copy capture JSON into `test-data.json`.
- **`--rest-equivalent`** — **`run-vending-app-flows.sh` only**: run machine REST mirror flows (VM-REST-02…08). Extra args from `run-all-local.sh` are forwarded to each phase; use e.g. `./tests/e2e/run-all-local.sh --reuse-data .e2e-runs/run-…/test-data.json --rest-equivalent` so the vending phase receives the flag **after** common options. **Field production apps use gRPC + MQTT**, not these REST paths.
- **`-h` / `--help`**

`run-all-local.sh` order: **preflight** → REST → web-admin flows → vending-app flows → gRPC → MQTT → **Phase 8 (scenarios 40–47)** → **reports** (`merge-events.py`, `generate-summary.py`, `generate-remediation.py`).

On non-zero exit or failed steps, the console prints the **`E2E_RUN_DIR`** and reminds you to open **`reports/summary.md`** and **`reports/remediation.md`**.

When `run-all-local.sh` invokes phase scripts, it sets **`E2E_IN_PARENT=1`** and reuses the same `E2E_RUN_DIR`. Phase scripts avoid duplicate report generation; the orchestrator writes **one** `reports/` set at the end (including **`test-data.redacted.json`** in the run root).

Implemented / built-in scenarios:

- **`tests/e2e/scenarios/00_preflight.sh`** — tooling (bash, curl, jq, python3), optional tools (newman, grpcurl, mosquitto), required env, GET `/health/live`, `/health/ready`, `/version`.
- **`tests/e2e/scenarios/00_rest_readonly_smoke.sh`** — read-only GET smoke (see **Commands** above).

Optional scenario stubs (still placeholders until you add them):

- `tests/e2e/scenarios/rest_local.sh`
- `tests/e2e/scenarios/web_admin_flows.sh`
- `tests/e2e/scenarios/vending_app_flows.sh`

Phase **6** machine gRPC contracts are **`20_grpc_*.sh` … `24_grpc_*.sh`**. Phase **7** MQTT contracts are **`30_mqtt_*.sh` … `32_mqtt_*.sh`** (invoked by **`run-mqtt-local.sh`**).

### gRPC machine contracts (Phase 6)

From repo root, with API + gRPC listener up (`GRPC_ADDR`, default `127.0.0.1:9090`):

```bash
# Proto files (default GRPC_PROTO_ROOT=$REPO_ROOT/proto when folder exists)
GRPC_USE_REFLECTION=false \
  ./tests/e2e/run-grpc-local.sh --reuse-data .e2e-runs/run-<…>/test-data.json

# Or server reflection
GRPC_USE_REFLECTION=true \
  ./tests/e2e/run-grpc-local.sh --reuse-data .e2e-runs/run-<…>/test-data.json
```

- **`--reuse-data`** should provide **`organizationId`**, **`machineId`**, **`productId`** (for commerce); copy **`secrets.private.json`** with **`machineToken`** or rely on **`20_grpc_machine_auth.sh`** with **`E2E_ACTIVATION_CODE`** / `activationCodePlain`.
- Mutating commerce steps need **`E2E_ALLOW_WRITES=true`** (see **`22_grpc_commerce_cash_sale.sh`**).
- **`reports/grpc-contract-summary.md`**: pass / fail / skip table per RPC; **`reports/grpc-contract-results.jsonl`**: machine-readable log. Missing RPCs in repo protos are logged **`skip`** / **`method_not_in_repo`** (never silent).
- **Metadata:** authenticated calls use **`Authorization: Bearer $MACHINE_TOKEN`** and optionally **`x-machine-id`**. Writes send **`idempotency-key`** where the harness sets one.

### MQTT contracts (Phase 7)

From repo root, with a reachable broker (`MQTT_HOST`, default port **1883** unless `MQTT_PORT` is set):

```bash
E2E_TARGET=local \
  ./tests/e2e/run-mqtt-local.sh --reuse-data .e2e-runs/run-<…>/test-data.json
```

- **Topics:** default layout follows **`docs/api/mqtt-contract.md`** and **`internal/platform/mqtt/topics.go`**. Set **`MQTT_TOPIC_LAYOUT`** to `legacy` or `enterprise`, **`MQTT_TOPIC_PREFIX`** (default **`avf/devices`**), and **`MQTT_MACHINE_ID`** (or reuse **`machineId`** from **`test-data.json`**). Override individual topics with **`MQTT_TOPIC_TELEMETRY`**, **`MQTT_TOPIC_COMMANDS`**, **`MQTT_TOPIC_COMMAND_ACK`**, **`MQTT_TOPIC_EVENTS`** when needed.
- **Auth / TLS:** **`MQTT_USERNAME`**, **`MQTT_PASSWORD`**, **`MQTT_CLIENT_ID`** (runner suffixes a unique id per mosquitto invocation); **`MQTT_USE_TLS=true`** with **`MQTT_CA_CERT`** when the broker requires TLS.
- **Command / ACK path:** prefers admin **`POST …/commands`** with **`commandType` `noop`** when **`ADMIN_TOKEN`** and org/machine ids are available; otherwise exercises a **synthetic** command on the wire (broker-only). On **`E2E_TARGET=production`**, the command scenario is a no-op unless **`test-data.json`** marks **`e2eTestMachine`** and you set **`E2E_MQTT_COMMAND_TEST_ACK=I_UNDERSTAND_MQTT_COMMAND_TEST_ACK`** (never targets destructive command types).
- **Artifacts:** **`mqtt/`** logs and JSON, **`reports/mqtt-contract-results.jsonl`**, **`reports/mqtt-contract-summary.md`** (and **`summary.md`** when not nested under **`run-all-local.sh`**).

### Vending app REST-equivalent (Phase 5)

Run from repo root (writes commerce paths when `E2E_ALLOW_WRITES=true`):

```bash
E2E_TARGET=local E2E_ALLOW_WRITES=true \
  ./tests/e2e/run-vending-app-flows.sh --rest-equivalent --reuse-data .e2e-runs/run-<…>/test-data.json
```

- **`test-data.json`** should include `machineId`, `productId`, `organizationId` / site fields as produced by web-admin setup (or your lab seed). Optionally set **`e2eTestMachine`** to `true` in JSON for production-target guard alignment.
- Copy **`secrets.private.json`** from the same prior run directory if you reuse data so **`machineToken`** is available; or set **`E2E_ACTIVATION_CODE`** / `activationCodePlain` in `test-data.json` and omit skip so **`02_machine_activation_bootstrap_rest.sh`** can claim. Use **`E2E_SKIP_ACTIVATION_CLAIM=1`** when the token is already in secrets.
- Artifacts: **`reports/va-rest-results.jsonl`**, **`reports/summary.md`** (appended section), REST captures under **`rest/`**, and **`test-data.json`** fields such as **`vmCashSuccessOrderId`** / **`vmCashSuccessPaymentId`** after cash success.
- **Production payment/refund:** requires `E2E_TARGET=production`, `E2E_ALLOW_WRITES=true`, `E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`, and **`e2eTestMachine`** `true` or `1` in `test-data.json`.
- **Offline out-of-order:** scenario **VM-REST-08** documents skipping unless the API exposes an explicit test hook; `E2E_OFFLINE_OUT_OF_ORDER=1` is reserved for future wiring.

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
- **`reports/summary.md`:** human-readable rollup plus **REST endpoints exercised** (table built from `rest/*.meta.json` when present).

### When the local server is not ready

See **[`e2e-troubleshooting.md`](e2e-troubleshooting.md)** (“Local API not ready”, “E2E harness prerequisites”). Typical fixes: start `cmd/api` (or your compose stack), confirm `BASE_URL` port, run DB migrations so `/health/ready` returns **200** (not **503** `not ready`).
- **`reports/remediation.md`:** failed steps only; link to **[`e2e-remediation-playbook.md`](e2e-remediation-playbook.md)**.
- **`reports/coverage.json`:** machine-readable counts + full events array.

## Related

- **[`e2e-test-data-guide.md`](e2e-test-data-guide.md)**
- **[`e2e-flow-coverage.md`](e2e-flow-coverage.md)**
- **[`e2e-troubleshooting.md`](e2e-troubleshooting.md)**
- **[`tests/e2e/README.md`](../../tests/e2e/README.md)**
