# Final Pre-Merge and Production Deploy Report

**Generated:** 2026-05-20  
**Branch:** `chore/final-full-system-verification-uuidv7-postman-tests`  
**Commit SHA (before commit):** `6527d502437f5137fb05c56d4851043b258afbc1`  
**Commit SHA (after commit):** `891c5e2` — `chore: verify release readiness and production deploy gates`  
**Remote:** `origin` → `https://github.com/leduytuanvu/avf-vending-api.git`

**Actions:** pre-merge verification + single commit. **No push, merge, or deploy.**

---

## 1. Current branch

`chore/final-full-system-verification-uuidv7-postman-tests`

---

## 2. Commit SHA before commit

```
6527d502437f5137fb05c56d4851043b258afbc1
```

(Merge PR #226 on `main` — branch had no prior commits on this feature name.)

---

## 3. Files changed (working tree vs HEAD)

~404 paths in `git status --short`; unstaged diff summary:

```
139 files changed, 3553 insertions(+), 3260 deletions(-)
```

**Major areas:**

| Area | Changes |
|------|---------|
| UUID v7 | `internal/platform/id/`, `migrations/00005_uuid_v7_defaults.sql`, tests, `scripts/checks/check-uuid-v7.sh` |
| Production migration | `cmd/migrate/`, `deployments/prod/Dockerfile`, `scripts/deploy/production-migrate.sh`, `validate_migration_image.sh` |
| CI / workflows | `.github/workflows/*`, deploy-prod migration gate |
| Postman | Collections, environments, UUID v7 prerequest |
| Repo structure | `ops/` → `deployments/docker/observability/`, docs/scripts reorg |
| Verification | `scripts/local/verify-full-system.sh`, `docs/testing/*` reports |
| Deletions | `secret-vars-scan.txt`, `vending_schema.sql` |

---

## 4. Go test result

```bash
go test ./...
```

**PASS** — all packages green (integration tests skip without `TEST_DATABASE_URL` in default unit run).

With DB (via `verify-full-system.sh`): postgres UUID v7 integration **PASS**.

---

## 5. Go vet result

```bash
go vet ./...
```

**PASS**

---

## 6. Go list result

```bash
go list ./...
```

**PASS** — all modules/packages resolve.

---

## 7. UUID v7 verification result

```bash
bash scripts/audit/verify-uuid-v7.sh   # PASS
bash scripts/checks/check-uuid-v7.sh   # PASS
```

- Production resource IDs: `id.NewUUIDV7()` / `public.uuid_generate_v7()`
- Allowlisted: JWT jti, request IDs, idempotency strings, historical migrations 00002/00004
- Migration `00005`: goose `StatementBegin/End` fix for plpgsql blocks

---

## 8. Migration verification result

| Check | Result |
|-------|--------|
| `scripts/ci/verify_migrations.sh` | PASS (5 files, 0 findings) |
| Fresh DB `go run ./cmd/migrate up` | PASS → version **5** |
| `TestUUIDV7DefaultOnInsert` | PASS |
| No DROP DATABASE / TRUNCATE in migrations | PASS |
| Docker image `/app/migrations` count | **5** SQL files |
| `/app/migrate validate` | PASS |

---

## 9. REST / OpenAPI status

| Check | Result |
|-------|--------|
| `tools/build_openapi.py` | PASS |
| `tools/openapi_verify_release.py` | PASS (327 operations) |
| `git diff --exit-code docs/swagger/` | PASS |
| Bearer auth on protected `/v1` routes | Verified |

---

## 10. gRPC status

| Check | Result |
|-------|--------|
| `buf generate` + `buf lint` | PASS |
| Proto/generated drift | PASS (none) |
| `check_machine_grpc_docs.py` | PASS (13 services) |

---

## 11. MQTT status

| Check | Result |
|-------|--------|
| `go test ./internal/platform/mqtt/...` | PASS |
| `run-mqtt-full-coverage.sh` | PASS (local broker `avf-emqx`) |
| Topic contract | Documented in `internal/platform/mqtt/topics.go` |

---

## 12. Postman collection / environment status

| Check | Result |
|-------|--------|
| `tools/build_postman_collection.py` | PASS |
| `tools/check_postman_artifacts.py` | PASS |
| All repo `*.json` parse validation | PASS |
| `postman-drift` (index vs generator) | PASS |
| UUID v7 `uuid7()` + `{{resource_uuid}}` in prerequest | Present |

---

## 13. Production auto-migration deploy gate status

| Control | Status |
|---------|--------|
| `deploy-prod.yml` manual `workflow_dispatch` only | Verified |
| Digest-pinned `app_image_ref` / `goose_image_ref` inputs | Present |
| `run_migration` default `true` | Present |
| `RUN_MIGRATION_ON_FIRST_NODE` on app-node A | Wired |
| `scripts/deploy/production-migrate.sh` (backup + up) | Present |
| `scripts/deploy/validate_migration_image.sh` | PASS (Windows MSYS path fix applied) |
| Migrations embedded in prod Dockerfile | PASS (5 files) |
| Smoke after ready on deploy | Wired in workflow |

**Not run:** live production workflow dispatch or Supabase migration.

---

## 14. Skipped checks and reason

| Check | Reason |
|-------|--------|
| `check-production-placeholders` | `rg` not on Windows PATH (CI covers) |
| `grpc-full-coverage-script` | Requires `VERIFY_WITH_GRPC=1` + live gRPC server |
| `newman-smoke` | Requires `VERIFY_WITH_NEWMAN=1` + Newman CLI |
| `verify-workflow-contracts` | Requires `actionlint` on PATH |
| `test-e2e-local` (45m) | Requires `VERIFY_DESTRUCTIVE=1` (skipped by design) |
| Production deploy / push | Explicitly out of scope this phase |

**Full wrapper (Phase 2):** `verify-full-system.sh` — **23 passed, 5 skipped, exit 0**

---

## 15. git diff --check

**WARN** (non-blocking): CRLF normalization warnings; trailing whitespace in `.env.example:179`, `.gitignore:1-3`. No conflict markers.

---

## 16. Final verdict

### **READY_TO_COMMIT** → **COMMITTED**

Commit `891c5e2` created on branch `chore/final-full-system-verification-uuidv7-postman-tests` (411 files, +14198 / −7463). **Not pushed.**

**After commit (not this phase):** push branch → PR to `develop` → CI → staging → manual production deploy per runbooks.

---

*Report generated immediately before `git commit` on this branch.*
