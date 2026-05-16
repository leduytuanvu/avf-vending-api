# Test matrix: independent endpoints vs business flows

This document splits coverage into **independent** REST-style checks (single endpoints or small reads) and **business flows** (multi-step narratives). No matrix row requires an **organization** tenant key: runners use `tests/e2e/.env` (`ADMIN_EMAIL` / `ADMIN_PASSWORD`) and scripts explicitly avoid `organization_id` / `organizationId` query parameters (see `reports/test/rest-independent-api-report.md` and `reports/test/rest-admin-crud-flow-report.md`).

**Local run session (2026-05-16 UTC)**

| Scope | Command | Exit | Primary evidence |
|--------|---------|------|------------------|
| Independent smoke | `python scripts/test/rest_independent_api_smoke.py` | **1** | `reports/test/rest-independent-api-report.md`, `reports/test/rest-independent/*.json` |
| Admin CRUD | `python scripts/test/rest_admin_crud_flow.py` | **1** | `reports/test/rest-admin-crud-flow-report.md`, `reports/test/rest-admin-crud/*.json` |
| Auth refresh probe | One-shot login + refresh (see row below) | **0** | `reports/test/matrix-auth-refresh-evidence.json` |
| Full flows | `bash tests/e2e/run-all-local.sh --fresh-data` | **0** | `.e2e-runs/run-20260516T095941Z-842-20119/` |

**Infra blockers:** None for the rows above — API answered on `BASE_URL` (`http://127.0.0.1:18080`), Postgres reachable by the API process, gRPC `:9090`, MQTT `:1883`, and optional Newman/grpcurl present per preflight. Failures below are **application-level** (HTTP 500, OpenAPI probe mismatch), not missing dependencies.

---

## Final pass/fail table

| # | Category | Type | Result | Notes |
|---|----------|------|--------|--------|
| I1 | Health / version / OpenAPI | Independent | **FAIL** | Health + version + metrics pass; `GET /swagger/doc.json` fails probe (`openapi_body_not_json`). |
| I2 | Auth login / me / refresh | Independent | **PASS** | Login + me via smoke; refresh via dedicated probe (both HTTP 200). |
| I3 | Site CRUD | Independent | **PASS** | Create, list, patch pass; `GET /v1/admin/sites/{id}` returned **400** but step marked pass by harness — confirm contract vs runner. |
| I4 | Product CRUD | Independent | **PASS** | All product steps pass in CRUD script. |
| I5 | Machine CRUD | Independent | **FAIL** | `GET /v1/admin/machines` and `GET /v1/admin/machines/{id}` → **500** (list/get); create/patch/slots pass. |
| I6 | Inventory read | Independent | **PASS** | `GET /v1/admin/inventory/low-stock` and `.../refill-suggestions` pass. Machine-scoped inventory GETs in smoke **skipped** (no `REST_INDEPENDENT_MACHINE_ID`; list endpoint 500). |
| I7 | Payment provider read | Independent | **FAIL** | `GET /v1/admin/payments/webhook-events`, `.../settlements`, `.../disputes` → **500**. |
| I8 | Reporting read | Independent | **PASS** | Sales / payments / fleet-health / inventory-exceptions reports return 200 in smoke. |
| I9 | Audit read | Independent | **PASS** | `GET /v1/admin/audit/events` → 200. |
| F1 | Admin setup flow | Flow | **PASS** | `run-web-admin-flows.sh --full` inside full run (`events.jsonl`: `web-admin-flows` passed). |
| F2 | Machine activation flow | Flow | **PASS** | Web-admin machine + activation code creation + vending claim + phase **E2E-40** (`tests/e2e/scenarios/40_e2e_first_boot.sh`). |
| F3 | Product / planogram / media flow | Flow | **PASS** | Web-admin product + planogram draft/publish + operator session; gRPC catalog/media manifest calls in `run-grpc-local.sh` (`reports/coverage.json` / grpc evidence). |
| F4 | Cash sale flow | Flow | **PASS** | Phase **E2E-41** (`41_e2e_cash_sale_success.sh`). |
| F5 | QR payment flow | Flow | **PASS** | Phase **E2E-42** (`42_e2e_qr_payment_success_mock.sh`). |
| F6 | Vend success flow | Flow | **PASS** | Covered inside **E2E-41** and **E2E-42** (`vend/start` + `vend/success`); REST captures e.g. `rest/vm-vend-ok.*`, `rest/p8-qr-vok.*`. |
| F7 | Vend failure refund flow | Flow | **PASS** | Phase **E2E-43** (`43_e2e_vend_failure_refund.sh`). |
| F8 | Offline replay flow | Flow | **PASS** | Phase **E2E-44** (`44_e2e_offline_replay.sh`); gRPC offline sync evidence under run dir. |
| F9 | Inventory restock flow | Flow | **PASS** | Phase **E2E-46** (`46_e2e_inventory_restock_adjustment.sh`). |
| F10 | Remote command ACK flow | Flow | **PASS** | Phase **E2E-45** (`45_e2e_remote_command_ack.sh`); MQTT `commands/ack` + related REST. |
| F11 | Reporting / audit flow | Flow | **PASS** | Phase **E2E-47** (`47_e2e_reporting_audit.sh`). |

**Counts:** Independent **6 PASS / 3 FAIL** · Flows **11 PASS / 0 FAIL**.

---

## Independent rows (detail)

### I1 — Health / version / OpenAPI

| Field | Value |
|--------|--------|
| **Preconditions** | API listening on `BASE_URL`; no auth for `/health/*`, `/version`, `/swagger/doc.json`, `/metrics`. |
| **Command** | `BASE_URL=http://127.0.0.1:18080 LOGIN_EMAIL="$ADMIN_EMAIL" LOGIN_PASSWORD="$ADMIN_PASSWORD" python scripts/test/rest_independent_api_smoke.py` |
| **Expected** | Script exit **0** when all probes pass; health live **200** body `ok`; ready **200** or **503** per rules; version **200** JSON with `version`; OpenAPI JSON with top-level `openapi` key (runner-specific). |
| **Evidence** | `reports/test/rest-independent-api-report.md`; `reports/test/rest-independent/get_health_live.json`, `get_health_ready.json`, `get_version.json`, `get_swagger_doc_json.json`, `get_metrics.json` |
| **Result** | **FAIL** — `/swagger/doc.json` probe: `openapi_body_not_json` (HTTP 200 but schema check failed). |

### I2 — Auth login / me / refresh

| Field | Value |
|--------|--------|
| **Preconditions** | Valid admin credentials in `tests/e2e/.env` (`ADMIN_EMAIL`, `ADMIN_PASSWORD`); maps to `LOGIN_EMAIL` / `LOGIN_PASSWORD` for Python runners. |
| **Command (login/me)** | Same as I1 (`rest_independent_api_smoke.py` issues `POST /v1/auth/login` and `GET /v1/auth/me`). |
| **Command (refresh)** | After login, `POST /v1/auth/refresh` with refresh token from login response (see saved probe file for exact capture). |
| **Expected** | Login **200**, me **200**; refresh **200** with new tokens. |
| **Evidence** | `reports/test/rest-independent/post_v1_auth_login.json`, `get_v1_auth_me.json`; `reports/test/matrix-auth-refresh-evidence.json` |
| **Result** | **PASS** |

### I3 — Site CRUD

| Field | Value |
|--------|--------|
| **Preconditions** | Admin auth; `REST_ADMIN_CRUD_CLEANUP=true` (default) archives/deletes created resources after run. |
| **Command** | `BASE_URL=... LOGIN_EMAIL=... LOGIN_PASSWORD=... python scripts/test/rest_admin_crud_flow.py` |
| **Expected** | Script exit **0** when every step passes; typically **201** create site, **200** list/patch, **200** get by id. |
| **Evidence** | `reports/test/rest-admin-crud-flow-report.md`; `reports/test/rest-admin-crud/02_site_create.json` … `05_site_patch.json` |
| **Result** | **PASS** (harness); **GET site by id returned 400** — review API vs id format / scope. |

### I4 — Product CRUD

| Field | Value |
|--------|--------|
| **Preconditions** | Same as I3. |
| **Command** | `python scripts/test/rest_admin_crud_flow.py` |
| **Expected** | Exit **0**; create/list/get/patch **200** (or **201** where applicable per OpenAPI). |
| **Evidence** | `reports/test/rest-admin-crud/06_product_create.json` … `09_product_patch.json` |
| **Result** | **PASS** |

### I5 — Machine CRUD

| Field | Value |
|--------|--------|
| **Preconditions** | Same as I3; site must exist (script creates one). |
| **Command** | `python scripts/test/rest_admin_crud_flow.py` |
| **Expected** | Exit **0**; list/get machines **200** with JSON body. |
| **Evidence** | `reports/test/rest-admin-crud/10_machine_create.json`, `11_machine_list.json`, `12_machine_get.json`, `13_machine_patch.json`, `14_machine_slots_get.json` |
| **Result** | **FAIL** — steps **11** and **12** HTTP **500**. |

### I6 — Inventory read

| Field | Value |
|--------|--------|
| **Preconditions** | Admin bearer token (script logs in). |
| **Command** | `python scripts/test/rest_independent_api_smoke.py` |
| **Expected** | **200** on fleet inventory helper endpoints; optional machine-scoped reads if `REST_INDEPENDENT_MACHINE_ID` set or parseable from machine list. |
| **Evidence** | `reports/test/rest-independent/get_v1_admin_inventory_low_stock.json`, `get_v1_admin_inventory_refill_suggestions.json` |
| **Result** | **PASS** for fleet reads; machine-scoped inventory lines **skipped** this run (see report note). |

### I7 — Payment provider read

| Field | Value |
|--------|--------|
| **Preconditions** | Admin auth. |
| **Command** | `python scripts/test/rest_independent_api_smoke.py` |
| **Expected** | **200** on `GET /v1/admin/payments/webhook-events`, `.../settlements`, `.../disputes`. |
| **Evidence** | `reports/test/rest-independent/get_v1_admin_payments_webhook_events.json`, `get_v1_admin_payments_settlements.json`, `get_v1_admin_payments_disputes.json` |
| **Result** | **FAIL** — all three **500**. |

### I8 — Reporting read

| Field | Value |
|--------|--------|
| **Preconditions** | Admin auth. |
| **Command** | `python scripts/test/rest_independent_api_smoke.py` |
| **Expected** | **200** on `GET /v1/reports/sales-summary`, `.../payments-summary`, `.../fleet-health`, `.../inventory-exceptions`. |
| **Evidence** | `reports/test/rest-independent/get_v1_reports_*.json` |
| **Result** | **PASS** |

### I9 — Audit read

| Field | Value |
|--------|--------|
| **Preconditions** | Admin auth. |
| **Command** | `python scripts/test/rest_independent_api_smoke.py` |
| **Expected** | **200** on `GET /v1/admin/audit/events`. |
| **Evidence** | `reports/test/rest-independent/get_v1_admin_audit_events.json` |
| **Result** | **PASS** |

---

## Flow rows (detail)

**Shared preconditions for all flows:** Load `tests/e2e/.env` (or equivalent) so `BASE_URL`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `GRPC_ADDR`, `MQTT_HOST`/`MQTT_PORT`, Postman paths, and local webhook HMAC defaults match the API. Optional: Mosquitto client on PATH (Windows Git Bash prepends Mosquitto dir in harness).

**Shared command (full matrix coverage in one orchestrated run):**

```bash
bash tests/e2e/run-all-local.sh --fresh-data
```

**Shared evidence root:** `.e2e-runs/run-20260516T095941Z-842-20119/`

| Field | Typical paths under run dir |
|--------|-----------------------------|
| Step timeline | `events.jsonl` |
| REST captures | `rest/*.meta.json`, `rest/*.response.json` |
| gRPC | `grpc/*.meta.json`, `grpc/*.response.json` |
| MQTT | `mqtt/*.publish.json`, `reports/mqtt-contract-results.jsonl` |
| Human summary | `reports/summary.md` |
| Phase 8 rollup | `reports/phase8-scenario-results.jsonl` |

### F1 — Admin setup flow

| Field | Value |
|--------|--------|
| **Maps to harness** | Child script `tests/e2e/run-web-admin-flows.sh --full` |
| **Expected** | Parent exit **0**; `events.jsonl` row `web-admin-flows` **passed**. |
| **Evidence** | `.e2e-runs/run-20260516T095941Z-842-20119/events.jsonl` (line with `"step":"web-admin-flows"`); `reports/wa-module-results.jsonl` |
| **Result** | **PASS** |

### F2 — Machine activation flow

| Field | Value |
|--------|--------|
| **Maps to harness** | Web-admin machine lifecycle + `POST .../activation-codes` + vending `POST /v1/setup/activation-codes/claim`; phase **`tests/e2e/scenarios/40_e2e_first_boot.sh`** |
| **Expected** | Phase 8 step `phase8-E2E-40-first-boot` **passed**. |
| **Evidence** | `.e2e-runs/run-20260516T095941Z-842-20119/events.jsonl`; `rest/wa-activation-create.*`, `rest/vm-claim.*`, `rest/vm-bootstrap.*`, `rest/vm-sale-catalog.*` |
| **Result** | **PASS** |

### F3 — Product / planogram / media flow

| Field | Value |
|--------|--------|
| **Maps to harness** | `run-web-admin-flows.sh` (product, planogram draft/publish, slots); `run-grpc-local.sh` (catalog + media manifest/delta/ack) |
| **Expected** | Web-admin module passes; gRPC rows **pass** for media/catalog as exercised. |
| **Evidence** | `rest/wa-product-create.*`, `rest/wa-planogram-draft.*`, `rest/wa-planogram-publish.*`; `grpc/*Media*` / catalog artifacts |
| **Result** | **PASS** |

### F4 — Cash sale flow

| Field | Value |
|--------|--------|
| **Maps to harness** | `tests/e2e/scenarios/41_e2e_cash_sale_success.sh` |
| **Expected** | `phase8-E2E-41-cash-sale-success` **passed**. |
| **Evidence** | `.e2e-runs/run-20260516T095941Z-842-20119/events.jsonl`; `rest/vm-cash-co.*`, `rest/vm-vend-start.*`, `rest/vm-vend-ok.*` |
| **Result** | **PASS** |

### F5 — QR payment flow

| Field | Value |
|--------|--------|
| **Maps to harness** | `tests/e2e/scenarios/42_e2e_qr_payment_success_mock.sh` |
| **Expected** | `phase8-E2E-42-qr-payment-mock` **passed**. |
| **Evidence** | `events.jsonl`; `rest/p8-qr-order.*`, `rest/p8-qr-ps.*`, `rest/p8-qr-vstart.*`, `rest/p8-qr-vok.*` |
| **Result** | **PASS** |

### F6 — Vend success flow

| Field | Value |
|--------|--------|
| **Maps to harness** | Narrative inside **E2E-41** and **E2E-42** (cash checkout / QR order → vend start → vend success). |
| **Expected** | Same phase steps **passed**; order reaches successful vend. |
| **Evidence** | `rest/vm-vend-ok.*`, `rest/p8-qr-vok.*`; `reports/summary.md` Phase 8 table |
| **Result** | **PASS** |

### F7 — Vend failure refund flow

| Field | Value |
|--------|--------|
| **Maps to harness** | `tests/e2e/scenarios/43_e2e_vend_failure_refund.sh` |
| **Expected** | `phase8-E2E-43-vend-failure-refund` **passed**. |
| **Evidence** | `events.jsonl`; `rest/vm-fail-*`, `rest/vm-fail-refund.*` |
| **Result** | **PASS** |

### F8 — Offline replay flow

| Field | Value |
|--------|--------|
| **Maps to harness** | `tests/e2e/scenarios/44_e2e_offline_replay.sh` |
| **Expected** | `phase8-E2E-44-offline-replay` **passed**. |
| **Evidence** | `events.jsonl`; `grpc/p8-off-*` (offline sync); see `reports/summary.md` gRPC summary |
| **Result** | **PASS** |

### F9 — Inventory restock flow

| Field | Value |
|--------|--------|
| **Maps to harness** | `tests/e2e/scenarios/46_e2e_inventory_restock_adjustment.sh` |
| **Expected** | `phase8-E2E-46-inventory-restock` **passed**. |
| **Evidence** | `events.jsonl`; `rest/inv-stock-*`, `rest/wa-stock.*`, `rest/inv-slots*.response.json` |
| **Result** | **PASS** |

### F10 — Remote command ACK flow

| Field | Value |
|--------|--------|
| **Maps to harness** | `tests/e2e/scenarios/45_e2e_remote_command_ack.sh` |
| **Expected** | `phase8-E2E-45-remote-command-ack` **passed**. |
| **Evidence** | `events.jsonl`; `mqtt/command.ack.*`; REST command traces under `rest/` as emitted |
| **Result** | **PASS** |

### F11 — Reporting / audit flow

| Field | Value |
|--------|--------|
| **Maps to harness** | `tests/e2e/scenarios/47_e2e_reporting_audit.sh` |
| **Expected** | `phase8-E2E-47-reporting-audit` **passed**. |
| **Evidence** | `events.jsonl`; `rest/rpt-audit-events.*`, `rest/rpt-commerce-recon.*`, `rest/rpt-finance-close-list.*` |
| **Result** | **PASS** |

---

## How to reproduce

From repository root (Git Bash on Windows is supported by the harness):

```bash
set -a && source tests/e2e/.env && set +a
export LOGIN_EMAIL="$ADMIN_EMAIL" LOGIN_PASSWORD="$ADMIN_PASSWORD"

python scripts/test/rest_independent_api_smoke.py
python scripts/test/rest_admin_crud_flow.py

bash tests/e2e/run-all-local.sh --fresh-data
```

Ensure the API and dependencies match `docs/testing/e2e-local-test-guide.md` (or your local standard operating procedure).
