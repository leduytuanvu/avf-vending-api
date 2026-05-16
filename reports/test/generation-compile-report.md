# Generation & compile consistency — report

**Repo:** `avf-vending-api`  
**Purpose:** Regenerate primary artifacts after company-scope cleanup, then compile-check the tree and scan outputs for policy tokens from task §3.

## Summary

| Step | Result |
|------|--------|
| **sqlc** | **PASS** — exit `0` |
| **Swagger / OpenAPI** | **PASS** — `docs/swagger/swagger.json` + `docs/swagger/docs.go` written |
| **Postman (docs/postman)** | **PASS** — four JSON artifacts written |
| **Postman full production suite** | **PASS** — `VALIDATION_PASS`, REST **333** requests |
| **Protobuf (`buf generate`)** | **PASS** — both passes exit `0` |
| **`go fmt ./...`** | **PASS** — exit `0`; `gofmt -l .` empty after run |
| **`go vet ./...`** | **PASS** — exit `0`, no diagnostics |
| **`go test ./... -run TestNonExistent -count=0`** | **PASS** — full compile of all packages; every listing shows `[no tests to run]` / `[no test files]` as expected |
| **Policy grep (§3 alternation)** | **PASS** — **zero** hits on generated hotspots and **zero** on full tracked tree (`git grep` exit **1**) |

---

## 1) Exact generation commands

Commands mirror `Makefile` targets (`sqlc`, `swagger`, `postman-generate`, `proto-generate`) plus the repository’s extended Postman bundle script.

```powershell
Set-Location <repo-root>

# sqlc (pinned generator — Makefile SQLC_VERSION)
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate

# OpenAPI / Swagger
python tools/build_openapi.py

# Postman JSON under docs/postman (Makefile postman-generate dependency)
python tools/build_postman_collection.py

# Full production suite (ZIP + matrices + sidecars); not invoked by Makefile alone
python postman/full-production-suite/generate_full_postman_suite.py

# Protobuf — two-phase buf invocation (Makefile proto-generate)
Set-Location proto
go run github.com/bufbuild/buf/cmd/buf@v1.47.0 generate --exclude-path avf/internal
go run github.com/bufbuild/buf/cmd/buf@v1.47.0 generate --template buf.gen.avfinternal.yaml --path avf/internal/v1
Set-Location ..
```

---

## 2) Exact compile / format commands

```powershell
go fmt ./...
go vet ./...
go test ./... -run TestNonExistent -count=0
gofmt -l .
```

---

## 3) Compile / vet / test results

| Command | Exit code | Notes |
|---------|-----------|-------|
| `go fmt ./...` | **0** | No lingering non-gofmt files (`gofmt -l .` produced no paths). |
| `go vet ./...` | **0** | Silent success (no issues printed). |
| `go test ./... -run TestNonExistent -count=0` | **0** | All packages built; filter ensured **no test bodies executed**. |

---

## 4) Policy grep — task §3 literals

The verification used **`git grep -n -I -E '<task-§3-alternation>'`** where the alternation is exactly the seven literals from the operator checklist joined with `|` (substring semantics apply). The concrete `-E` string was pasted **only at the shell** so this Markdown stays searchable under stricter doc policies.

### 4a) Generated hotspots only

Scopes: `internal/gen/db`, `docs/swagger`, `docs/postman`, `postman/full-production-suite`, `internal/gen/avfinternalv1`, with exclusions `:!vendor`, `:!node_modules`, `:!.git`.

**Result:** **no output**, exit code **1** (no matches).

### 4b) Full tracked tree

Same pattern over `.` with the same exclusions.

**Result:** **no output**, exit code **1** (no matches).

---

## 5) Files changed (`git diff --name-only`)

- **Entire working tree vs `HEAD`:** **639** paths differ (large pre-existing branch drift; CRLF warnings observed on some shell scripts).
- **Scoped to typical generator outputs** (`internal/gen/db`, `docs/swagger`, `docs/postman`, `postman/full-production-suite`, `proto/avf/**`, `internal/gen/avfinternalv1`): **89** paths.

Representative scoped paths (first batch):

- `docs/postman/*.postman_collection.json`, `docs/postman/*.postman_environment.json`
- `docs/swagger/swagger.json`
- `internal/gen/db/*.sql.go`, `internal/gen/db/models.go`, …
- `internal/gen/avfinternalv1/*.pb.go`
- `proto/avf/**/**.pb.go` (+ occasional `.proto` noise — review CRLF / manual edits before commit)

Reproduce the full listing with:

```powershell
git diff --name-only -- internal/gen/db docs/swagger docs/postman postman/full-production-suite proto/avf internal/gen/avfinternalv1
```

1. If committing: inspect unexpected `proto/**/*.proto` diffs — regeneration normally touches `*.pb.go`; stray `.proto` churn suggests CRLF or manual edits.  
2. Run `make api-contract-check` on CI/Linux for buf **lint** / breaking-change gates (not executed here).  
3. Optional: `python tools/check_postman_artifacts.py` after committing Postman JSON.

---

## 7) Deliverables checklist

- This file: **`reports/test/generation-compile-report.md`**
