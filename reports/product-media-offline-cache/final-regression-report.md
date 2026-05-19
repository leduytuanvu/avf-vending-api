# Phase 8 — Final regression & E2E proof

**Report path:** `reports/product-media-offline-cache/final-regression-report.md`  
**Generated:** 2026-05-19 (Phase 8 automation run)  
**Policy:** Nothing marked READY unless the commands listed below actually completed successfully in this phase.

---

## 1. Commands run (toolchain)

| Step | Command | Result |
|------|---------|--------|
| sqlc generate | `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate` | **PASS** |
| sqlc (Makefile) | `make sqlc` | **N/A** — `make` not available in this Windows runner; equivalent `go run … sqlc@v1.31.1 generate` used (matches `Makefile` pin). |
| OpenAPI / Swagger | `python tools/build_openapi.py` | **PASS** |
| Postman | `python tools/build_postman_collection.py` | **PASS** |
| Postman checks | `python tools/check_postman_artifacts.py` | **PASS** |
| gofmt | `go fmt ./...` | **PASS** (at least one file reformatted: `internal/platform/mqtt/catalog_refresh.go`) |
| go vet | `go vet ./...` | **PASS** |
| Unit/integration tests | `go test ./... -count=1` | **PASS** |
| FULL suite validator | `python postman/full-production-suite/validate_generated_assets.py` | **PASS** (`VALIDATION_PASS`, REST 327 / gRPC 86 / MQTT 28) |

---

## 2. Migration proof (fresh local database)

**Environment:** Docker container `avf-postgres` (repo compose `deployments/docker/docker-compose.yml`), host port **15432**.

| Step | Command | Result |
|------|---------|--------|
| Fresh DB | `docker exec avf-postgres psql -U postgres -c "DROP DATABASE IF EXISTS avf_phase8_regression;"` then `CREATE DATABASE avf_phase8_regression;` | **PASS** |
| Migrations | `go run github.com/pressly/goose/v3/cmd/goose@v3.27.0 -dir migrations postgres "postgres://postgres:postgres@127.0.0.1:15432/avf_phase8_regression?sslmode=disable" up` | **PASS** (versions **00001–00004** applied) |

### 2.1 `media_assets` / `product_tags` verification

- **`product_tags`:** composite PK `(product_id, tag_id)`, FKs to `products` / `tags` — **present**.
- **`media_assets`:** includes **`kind`**, **`object_version`**, **`sha256`**, variant object keys, **`status`**, timestamps — **present** (see `\d media_assets` on `avf_phase8_regression`).
- **Indexes (spot check):** `product_tags_pkey`, `media_assets_pkey` — **present**.

### 2.2 Legacy org / scope / tenant identifiers (database)

Queries on `information_schema` / `pg_indexes` for table names / column names / index names matching legacy partition wording:

- Tables matching `%organization%`, `%tenant%`, `%scope%`, etc.: **0 rows**.
- Columns matching `scope_id`, `organization_id`, `tenant_id`, `org_admin`: **0 rows**.
- Indexes matching those patterns: **0 rows**.

**Migration proof verdict:** **PASS** on fresh DB after full current migration set.

---

## 3. REST / OpenAPI / Postman proof

| Artifact | Path |
|----------|------|
| OpenAPI JSON | `docs/swagger/swagger.json` |
| Embedded swagger Go | `docs/swagger/docs.go` |
| Postman collection | `docs/postman/avf-vending-api.postman_collection.json` |
| Postman env (example) | `docs/postman/avf-local.postman_environment.json` |
| FULL parity collection | `postman/full-production-suite/AVF_REST_365_FULL.postman_collection.json` |
| FULL env | `postman/full-production-suite/AVF_FULL_100.postman_environment.json` |
| Manifest | `postman/full-production-suite/manifest.json` |

**Evidence:** `python tools/check_postman_artifacts.py` → **OK**; `validate_generated_assets.py` → **VALIDATION_PASS** (327 REST ops).

---

## 4. gRPC proof

| Layer | Result | Notes |
|-------|--------|--------|
| **Go tests** | **PASS** | `go test ./... -count=1` includes `internal/grpcserver` (e.g. `machine_catalog_grpc_test.go`). |
| **Bash E2E** | **NOT RUN** | See §6 — orchestration script failed to start in this environment. |

**Proto / docs:** `proto/avf/machine/v1/catalog.proto`, `docs/api/machine-grpc.md` updated as part of the branch; FULL suite gRPC matrix: `postman/full-production-suite/grpc/AVF_GRPC_86_METHOD_MATRIX.csv`.

---

## 5. MQTT proof

| Layer | Result | Notes |
|-------|--------|--------|
| **Go tests** | **PASS** | `internal/platform/mqtt`, `internal/e2e/correctness` (e.g. MQTT command integration), `mqttprom` packages exercised under `go test ./... -count=1`. |
| **Bash E2E** | **NOT RUN** | See §6. |

**Contract / suite:** `docs/api/mqtt-contract.md`; FULL suite MQTT assets under `postman/full-production-suite/mqtt/`.

---

## 6. E2E orchestration (bash scripts)

**Status:** **NOT EXECUTED** — **do not claim E2E PASS.**

Attempt:

```text
bash tests/e2e/run-grpc-local.sh
```

Failed with:

```text
execvpe(/bin/bash) failed: No such file or directory
```

(WSL/bash not available on this Phase 8 runner.)

### 6.1 Exact commands for the operator (run on Linux/macOS/Git Bash with stack up)

Prerequisites: API, Postgres, NATS, MQTT broker, etc. per `tests/e2e/lib/e2e_common.sh` / local compose profiles.

```bash
cd /path/to/avf-vending-api

bash tests/e2e/run-grpc-local.sh
bash tests/e2e/run-mqtt-local.sh
bash tests/e2e/run-all-local.sh --fresh-data
```

Optional migration replay (matches Phase 8 proof):

```bash
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:15432/avf_phase8_regression?sslmode=disable'
go run github.com/pressly/goose/v3/cmd/goose@v3.27.0 -dir migrations postgres "$DATABASE_URL" up
```

---

## 7. Grep gate (retired partition literals)

**Command:**

```bash
git grep -nE 'scope_id|scopeId|ScopeID|organization_id|organizationId|OrganizationID|tenant_id|tenantId|TenantID|org_admin|tenant-scoped|org-scoped' -- \
  '*.go' '*.sql' '*.json' '*.yaml' '*.yml' \
  ':(exclude)migrations/**' \
  ':(exclude)reports/**' \
  ':(exclude)docs/runbooks/**'
```

**Result:** **PASS — no matches** (exit status **1**, empty output = `git grep` found nothing).

---

## 8. Blockers & final verdict

### Blockers

1. **Full bash E2E** (`run-grpc-local.sh`, `run-mqtt-local.sh`, `run-all-local.sh --fresh-data`) was **not run** in Phase 8 automation because **`bash` was not available** in the execution environment.  
2. **`make sqlc`** was not invoked (no `make` on PATH); **`go run … sqlc@v1.31.1 generate`** was used instead — functionally equivalent to `Makefile` pin.

### Final verdict

**`NOT_READY_WITH_BLOCKERS`**

- Toolchain (**sqlc**, OpenAPI, Postman, **go fmt**, **go vet**, **go test -count=1**), **migration proof** on a **fresh** DB, **grep gate**, and **offline** Postman/swagger validators **passed** in this run.
- **End-to-end orchestration scripts** are **outstanding** until an operator runs the commands in §6.1 successfully.

**Do not treat as merge-ready for full production confidence until §6.1 passes on a representative local/staging stack.**

---

## 9. Artifacts summary

| Category | Location |
|----------|----------|
| Regression report | `reports/product-media-offline-cache/final-regression-report.md` |
| Migrations | `migrations/*.sql` |
| Swagger | `docs/swagger/swagger.json`, `docs/swagger/docs.go` |
| Postman (docs) | `docs/postman/*.json` |
| FULL Postman suite | `postman/full-production-suite/*` |
| E2E scripts | `tests/e2e/*.sh`, `tests/e2e/scenarios/` |

**Git push:** not performed in this phase (per instructions).
