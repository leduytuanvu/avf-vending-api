# Phase 10 — Final repository-wide consistency gate

**Report:** `reports/product-media-offline-cache/final-consistency-gate.md`  
**Environment:** Windows PowerShell, repo root `avf-vending-api`  
**Date:** 2026-05-19  

---

## Final verdict: **READY_TO_PUSH**

All required static checks, generators, validators, contract scans, legacy-term grep, and `go test ./...` (with **`DATABASE_URL`** / **`TEST_DATABASE_URL`** unset) completed successfully.

No git push was performed (gate only).

---

## 1. Git snapshot (pre-gate working tree)

Commands:

```powershell
git status --short
git diff --stat
```

**Observation:** The branch contains a large feature set (many modified paths plus untracked paths such as `.cursor-*.log`, report folders, and optional Postman bundle artifacts). Phase 10 does **not** require a clean tree; it requires **generators + tests + scans** to pass. Before pushing, manually exclude IDE/logs/zips from commits unless they are intentional deliverables.

---

## 2. Static checks — commands and results

| Step | Command | Result |
|------|---------|--------|
| SQL codegen | `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate` | **PASS** (exit 0) |
| OpenAPI | `python tools/build_openapi.py` | **PASS** — wrote `docs/swagger/swagger.json`, `docs/swagger/docs.go` |
| Postman (docs) | `python tools/build_postman_collection.py` | **PASS** — wrote `docs/postman/*.postman_collection.json`, `docs/postman/*.postman_environment.json` |
| Full Postman suite | `python postman/full-production-suite/generate_full_postman_suite.py` | **PASS** — `VALIDATION_PASS`, `PASS_IMPORT_ASSETS_COMPLETE` |
| Postman asset validator | `python postman/full-production-suite/validate_generated_assets.py` | **PASS** — `VALIDATION_PASS` |
| Format | `go fmt ./...` | **PASS** (exit 0) |
| Vet | `go vet ./...` | **PASS** (exit 0) |
| Tests | `Remove-Item Env:TEST_DATABASE_URL, Env:DATABASE_URL -ErrorAction SilentlyContinue; go test ./... -count=1` | **PASS** (exit 0) |

**Note:** Clearing **`DATABASE_URL`** avoids accidental execution of heavy `internal/modules/postgres` integration tests against a non-fixture database (see Phase 9 migration report).

---

## 3. Generated JSON validation

Parsed with Python `json.load` (UTF-8):

| Artifact | Path | Result |
|----------|------|--------|
| OpenAPI | `docs/swagger/swagger.json` | **OK** |
| Postman collection | `docs/postman/avf-vending-api.postman_collection.json` | **OK** |
| Postman environment | `docs/postman/avf-local.postman_environment.json` | **OK** |
| gRPC request templates | `postman/full-production-suite/grpc/grpc_request_templates.json` | **OK** |
| MQTT request templates | `postman/full-production-suite/mqtt/mqtt_request_templates.json` | **OK** |
| MQTT payloads bundle | `postman/full-production-suite/mqtt/AVF_MQTT_100_PAYLOADS.json` | **OK** |

Embedded OpenAPI is additionally covered by **`internal/httpserver` tests** (`TestOpenAPI_*`), which passed.

---

## 4. Product media contract scan

| Check | Method | Result |
|-------|--------|--------|
| **`imageBase64`** | Ripgrep `*.json`, `*.md`, `*.yaml`, `*.yml`, `*.proto`, `*.go` | **No hits** |
| **`base64` (product/MQTT transport)** | Ripgrep in docs/swagger + MQTT paths | **No product-image base64 payloads**; remaining hits are legitimate (e.g. MFA key text in swagger description, MQTT contract explicitly **forbidding** base64 on MQTT, ClickHouse column names in ops docs) |
| **Image-only product mutation** | Review OpenAPI `V1AdminProductMutationRequest` examples | **Aligned** — active product examples include **`primaryMediaId`** (see §6 fix) |
| **gRPC media manifest** | `grpc_request_templates.json` + `docs/api/machine-grpc.md` | **Present** — e.g. `GetMediaManifest` entries; docs describe manifest/delta behavior |
| **MQTT binary/base64 images** | Ripgrep under `postman/full-production-suite/mqtt/` | **No hits** for `base64`, `imageBase64`, `data:image` |

---

## 5. Forbidden legacy terms (`git grep`)

Command (exact pathspec exclusions per Phase 10):

```text
git grep -nE 'scope_id|scopeId|ScopeID|organization_id|organizationId|OrganizationID|tenant_id|tenantId|TenantID|org_admin|tenant-scoped|org-scoped' -- \
  '*.go' '*.sql' '*.json' '*.yaml' '*.yml' \
  ':(exclude)migrations/**' \
  ':(exclude)reports/**' \
  ':(exclude)docs/runbooks/**'
```

**Result:** **No matches** (`git grep` exit code **1** = zero lines — expected on Windows/Git for “no match”).

---

## 6. Contract fix applied during this gate

**Issue:** OpenAPI request examples for **`POST /v1/admin/products`** used **`active: true`** without **`primaryMediaId`**, conflicting with documented rules (“required when activating a product that does not yet have primary media”).

**Change:** `tools/build_openapi.py` — `product_mut_req` now sets **`primaryMediaId`** to the same canonical UUID used in `product_row`.

Regenerated:

- `docs/swagger/swagger.json`, `docs/swagger/docs.go`
- `docs/postman/*.json`
- Full-suite outputs under `postman/full-production-suite/` (via generator script)

---

## 7. Files touched by Phase 10 automation / fix

At minimum (committed-ready subset):

- `tools/build_openapi.py` — **`primaryMediaId` on `product_mut_req`**
- `docs/swagger/swagger.json`, `docs/swagger/docs.go` — regenerated
- `docs/postman/avf-vending-api.postman_collection.json`, `docs/postman/avf-vending-api-function-path.postman_collection.json`, `docs/postman/avf-*.postman_environment.json` — regenerated
- `postman/full-production-suite/**` — refreshed by `generate_full_postman_suite.py` / validator (as produced by the scripts)

Other paths may still show edits from the broader feature branch (`git status`).

---

## 8. Optional checks not gated here

- **E2E shell harness** (`bash tests/e2e/run-all-local.sh`) — not run in this Windows session; does not affect **READY_TO_PUSH** for the commands listed in §2.

---

## 9. Deliverables summary

| Deliverable | Status |
|-------------|--------|
| **Final verdict** | **READY_TO_PUSH** |
| **Files changed** | See §7 + full `git status` |
| **Commands / results** | See §2–§5 |
