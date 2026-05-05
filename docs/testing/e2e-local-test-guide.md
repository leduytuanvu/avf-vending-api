# Local E2E test guide (shell-driven, planned)

This guide describes how **future** multi-protocol local E2E runs **will** work once `tests/e2e/run-*.sh` scripts exist. Today, database-heavy correctness tests live under **[`local-e2e.md`](local-e2e.md)** (`make test-e2e-local`).

## Scope

- **REST:** Web Admin + machine-helper routes from **[`docs/swagger/swagger.json`](../swagger/swagger.json)** and **[`docs/postman/`](../postman/)**
- **gRPC:** `proto/avf/machine/v1/*.proto` services (vending app)
- **MQTT:** **[`docs/api/mqtt-contract.md`](../api/mqtt-contract.md)** topic layouts

## Prerequisites

1. **API server** built and running locally (or pointed at **staging** with explicit flags — never default to production mutation without unlock env vars from Postman policy).
2. **PostgreSQL** (if tests assert DB state — align with [`local-e2e.md`](local-e2e.md)).
3. **MQTT broker** (Mosquitto or enterprise equivalent) when running `run-mqtt-local.sh`.
4. **Tools:** `bash`, `curl`, `jq`, `grpcurl` (or lang-specific gRPC client), `mosquitto_pub/sub` (optional), `python3` for JSON helpers.
5. **Env file:** copy `tests/e2e/data/seed.local.example.json` principles into a **local** `.env` **outside** git (see [`e2e-test-data-guide.md`](e2e-test-data-guide.md)); never commit tokens.

## Environment conventions (planned)

| Variable | Purpose |
|----------|---------|
| `E2E_API_BASE` | e.g. `http://localhost:8080` |
| `E2E_GRPC_HOST` | e.g. `localhost:9090` |
| `E2E_GRPC_TLS` | `0` / `1` |
| `E2E_MQTT_URL` | `tls://` or `tcp://localhost:1883` for dev |
| `E2E_MQTT_PREFIX` | Topic prefix; **must not** be `avf/devices` on staging per Postman guard |
| `E2E_ADMIN_TOKEN` | Bearer for admin REST |
| `E2E_MACHINE_TOKEN` | Bearer for machine REST if needed |
| `E2E_RUN_ID` | UUID; defaults per-run under `.e2e-runs/run-*` |

## Run directory layout

Each run writes artifacts under:

```text
.e2e-runs/run-<timestamp>-<short-id>/
  env.export              # redacted snapshot of non-secret config
  rest/                   # curl transcripts or HAR fragments
  grpc/                   # grpcurl logs
  mqtt/                   # pub/sub captures
  reports/summary.json    # pass/fail per flow_id
  data-capture.json       # optional link to reusable data (see reusable-test-data.example.json)
```

The directory **`.e2e-runs/`** is gitignored.

## Reuse vs fresh data

- **`--reuse-data`:** load `data-capture.json` (or path from prior run) — same org/machine/product IDs; faster regression.
- **`--fresh-data`:** generate new IDs from seed template; use after **activation already claimed**, **idempotency conflicts**, or corrupted scratch state.

## Logs and reports

- **Console:** scripts echo phase + flow_id.
- **`reports/summary.json`:** machine-readable overall status (planned schema TBD).
- **Release alignment:** field evidence remains **[`field-test-cases.md`](field-test-cases.md)**; this harness is **dev/staging** acceleration.

## Intended commands (see `tests/e2e/README.md`)

```bash
./tests/e2e/run-all-local.sh
./tests/e2e/run-rest-local.sh
./tests/e2e/run-grpc-local.sh
./tests/e2e/run-mqtt-local.sh
./tests/e2e/run-web-admin-flows.sh
./tests/e2e/run-vending-app-flows.sh
```

## Related

- **[`e2e-test-data-guide.md`](e2e-test-data-guide.md)**
- **[`e2e-troubleshooting.md`](e2e-troubleshooting.md)**
- **[`e2e-remediation-playbook.md`](e2e-remediation-playbook.md)**
