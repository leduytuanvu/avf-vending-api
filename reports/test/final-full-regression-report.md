# Final full regression report (release-candidate gate)

**Generated (UTC):** `2026-05-16T10:21:00Z` (approx., session end)  
**Repository:** `avf-vending-api`  
**Host:** Windows + Git Bash for POSIX steps  

## 1. Summary

| Gate area | Result |
|-----------|--------|
| Working tree snapshot (`git status --short`) | **Recorded** — branch had large pre-existing diff; see §4 |
| Forbidden tenant/org grep (tracked files only) | **PASS** — `git grep` returned **no matches** |
| `sqlc generate` | **PASS** (exit 0) |
| OpenAPI / Swagger (`python tools/build_openapi.py`) | **PASS** |
| Postman (`python tools/build_postman_collection.py`) | **PASS** |
| `gofmt` (tracked `*.go` files that exist on disk) | **PASS** |
| `go vet ./...` | **PASS** |
| `go test ./...` | **PASS** |
| Shell syntax (`bash -n` on `tests/`, `scripts/`, `deployments/**/*.sh`) | **PASS** |
| Python syntax (`python -m py_compile` on `tools/**/*.py`) | **PASS** |
| Postman JSON (`python -m json.tool` on `*.postman_collection.json` / `*.postman_environment.json` under `docs/postman/` and `postman/`) | **PASS** |
| MQTT (`bash tests/e2e/run-mqtt-local.sh`) | **PASS** (after harness bootstrap for standalone machine scope) |
| gRPC (`bash tests/e2e/run-grpc-local.sh`) | **PASS** (after harness bootstrap + scenario 20 refresh ordering / activation secret fix) |
| Full local E2E (`bash tests/e2e/run-all-local.sh --fresh-data`) | **PASS** — 23/23 steps, **0 failed**, **0 skipped** at harness summary |
| Final API smoke scripts (step 12 runners) | **FAIL** — see §7 (database/view drift vs scripts + provisioning assumptions) |

**Bottom line:** Compile-time gates, unit/integration Go tests, MQTT/gRPC/E2E orchestration, and strict **no organization / tenant** grep on **tracked** paths are green. Auxiliary Python “final smoke” runners failed against the live API with an explicit Postgres error (`v_machine_current_operator` missing) and stricter HTTP expectations than the harness narratives.

---

## 2. Exact commands run (ordered)

```bash
git status --short

git grep -n -I -i -E 'organization|organization_id|organizationid|org_admin|tenant|tenant_id|tenantid|canary_organization|e2e_organization|devorganization' \
  -- . ':!vendor' ':!node_modules' ':!.git' || true

sqlc generate
python tools/build_openapi.py
python tools/build_postman_collection.py

git ls-files "*.go" | while read -r f; do test -f "$f" && gofmt -w "$f"; done
go vet ./...
go test ./...

find tests scripts deployments -name "*.sh" -print0 | xargs -0 -n1 bash -n

shopt -s globstar nullglob
for f in tools/*.py tools/**/*.py; do python -m py_compile "$f"; done

# JSON validate Postman artifacts
while IFS= read -r -d "" f; do python -m json.tool "$f" >/dev/null; done < <(find docs/postman postman -name "*.postman_collection.json" -print0 2>/dev/null)
while IFS= read -r -d "" f; do python -m json.tool "$f" >/dev/null; done < <(find docs/postman postman -name "*.postman_environment.json" -print0 2>/dev/null)

bash tests/e2e/run-mqtt-local.sh
bash tests/e2e/run-grpc-local.sh
bash tests/e2e/run-all-local.sh --fresh-data

set -a && source tests/e2e/.env && set +a
export LOGIN_EMAIL="$ADMIN_EMAIL" LOGIN_PASSWORD="$ADMIN_PASSWORD"
python scripts/test/rest_admin_crud_flow.py
python scripts/test/commerce_http_flow.py
python scripts/test/inventory_telemetry_http_flow.py
```

---

## 3. Exact results (high signal)

| Command | Exit |
|---------|------|
| `sqlc generate` | 0 |
| `python tools/build_openapi.py` | 0 |
| `python tools/build_postman_collection.py` | 0 |
| `go vet ./...` | 0 |
| `go test ./...` (final) | 0 |
| `bash -n` over shell scripts | 0 |
| `python -m py_compile` (tools) | 0 |
| `python -m json.tool` (Postman JSON files) | 0 |
| `bash tests/e2e/run-mqtt-local.sh` | 0 |
| `bash tests/e2e/run-grpc-local.sh` | 0 |
| `bash tests/e2e/run-all-local.sh --fresh-data` | 0 (**23 passed / 0 failed / 0 skipped** at top-level harness summary) |
| `python scripts/test/rest_admin_crud_flow.py` | **1** |
| `python scripts/test/commerce_http_flow.py` | **1** |
| `python scripts/test/inventory_telemetry_http_flow.py` | **1** |

---

## 4. Files changed (this regression / fixes applied here)

These edits were made to **unblock gates** and **align single-company semantics** with `/v1` handlers:

- `internal/config/config_test.go` — clear inherited `REDIS_*` shell leakage in `setMinimalValidLoadEnv`.
- `internal/config/deployment_env_test.go` — same for staging fixtures + replace `os.Unsetenv` on Redis with `t.Setenv` in production Redis tests.
- `internal/app/fleetadmin/service.go` — `GetMachine`: accept `companyID == uuid.Nil` (matches HTTP `parseAdminFleetCompanyScope` single-company placeholder).
- `internal/app/payments/admin_service.go` — drop stale `payments: company required` guards for read/list/export paths where SQL is already org-global (`ForOrg*` queries).
- `tests/e2e/run-mqtt-local.sh` — standalone bootstrap: run `scenarios/01_web_admin_setup.sh` when `machineId` / `MQTT_MACHINE_ID` missing.
- `tests/e2e/run-grpc-local.sh` — standalone bootstrap: run `01_web_admin_setup.sh` when neither machine token nor activation secret is present; gate on `get_secret activationCodePlain`.
- `tests/e2e/scenarios/20_grpc_machine_auth.sh` — read activation plaintext via **`get_secret`** first; call **`MachineAuthService/RefreshMachineToken` before `MachineTokenService/RefreshMachineToken`** so refresh rotation order matches server expectations.

**Note:** The branch already contained a very large modified/untracked set unrelated to this session (company-scope removal work). Only the paths above were edited specifically for this regression gate pass/fail behavior.

**Working tree hygiene:** `internal/httpserver/machine_tenant_middleware.go` is **deleted on disk** but still tracked in git in this snapshot — resolve with `git rm` or restore before merge.

---

## 5. Generated artifacts regenerated

Commands:

- `sqlc generate`
- `python tools/build_openapi.py` → `docs/swagger/swagger.json`, `docs/swagger/docs.go`
- `python tools/build_postman_collection.py` → `docs/postman/*.postman_collection.json`, `docs/postman/*.postman_environment.json`

Generated outputs matched the tree already in most cases; rerun logged successful writes.

---

## 6. E2E run directory (canonical full gate)

**`.e2e-runs/run-20260516T101500Z-1573-6996/`**

Artifacts: `reports/summary.md`, `events.jsonl`, `rest/`, `grpc/`, `mqtt/`, phase 8 scenarios **40–47**, merged `reports/coverage.json`.

---

## 7. Remaining blockers / explicit reasons

### A. Final API smoke scripts (step 12)

**Evidence:** `reports/test/rest-admin-crud/11_machine_list.json`

**Exact SQL error from API:**

```text
ERROR: relation "v_machine_current_operator" does not exist (SQLSTATE 42P01)
```

**Meaning:** The database backing `BASE_URL` (`http://127.0.0.1:18080`) is **missing the view** defined in migration `migrations/00008_machine_operator_sessions.sql` / `db/schema/01_platform.sql`. Apply migrations (`make migrate-up` with correct `DATABASE_URL`, or recreate local Postgres from compose + goose) until `v_machine_current_operator` exists.

Until that view exists, **`GET /v1/admin/machines`** and **`GET /v1/admin/machines/{id}`** paths that hit fleet-admin enrichment queries will continue to 500 in those scripts.

### B. `commerce_http_flow.py` / `inventory_telemetry_http_flow.py`

Even with login/admin provisioning, these scripts enforce **stricter success criteria** (e.g. `POST /v1/commerce/orders` **201** + inventory delta) than “single POST succeeded”. Latest commerce evidence shows `400` with **`product is not in the machine's published assortment`** — expected when the script’s lightweight provisioning skips full publish/planogram parity.

**Not hidden:** `inventory_telemetry_http_flow` printed optional Docker-backed integration tests as **SKIPPED** when `TEST_DATABASE_URL` / Docker engine were unavailable — see `reports/test/inventory-telemetry-offline-report.md`.

### C. Forbidden grep scope

`git grep` searches **tracked files only** (by design). Untracked paths are **not** scanned until added to the index.

---

## 8. API / MQTT / gRPC / E2E confirmation

| Plane | Passed locally? | Notes |
|-------|-------------------|-------|
| HTTP API (via E2E harness against `BASE_URL`) | **Yes** — full `run-all-local.sh --fresh-data` | Includes REST/Newman phase, web-admin, vending REST equivalents |
| MQTT | **Yes** — `run-mqtt-local.sh` | Broker + mosquitto clients reachable |
| gRPC | **Yes** — `run-grpc-local.sh` | grpcurl + reflection/proto path OK |
| Full E2E | **Yes** | `.e2e-runs/run-20260516T101500Z-1573-6996/` |

**Infra blocker for step 12 only:** Postgres schema missing **`v_machine_current_operator`** for the API process’s `DATABASE_URL` (exact error above).

---

## Appendix — Step 12 coverage mapping (honest)

The repository’s **authoritative** commerce + inventory + activation narratives for this gate are the **Phase 8 scenarios** inside **`run-all-local.sh`** (passed):

- Activation / provisioning: web-admin setup + vending bootstrap paths inside the orchestrated run.
- Commerce: `41_e2e_cash_sale_success.sh`, `42_e2e_qr_payment_success_mock.sh`, etc.
- Inventory: `46_e2e_inventory_restock_adjustment.sh`.

The Python scripts are additional probes; they currently **fail closed** on schema drift + unpublished assortment unless the DB and catalog state match their assumptions.
