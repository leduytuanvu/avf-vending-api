# Final Release Readiness Checklist

Generated: 2026-05-20 | Branch: `chore/final-full-system-verification-uuidv7-postman-tests`

---

## 1. What was verified

- [x] Repository inventory (REST, gRPC, MQTT, migrations, workers, Postman)
- [x] UUID v7 static audit + DB default integration test
- [x] Fresh database migration 00001 → 00005
- [x] Production auto-migration gate (workflow + scripts static audit)
- [x] OpenAPI generation + 327-operation contract verify
- [x] Postman JSON validation + UUID v7 prerequest
- [x] Proto/sqlc codegen drift checks
- [x] `go test -count=1 ./...` — all packages pass
- [x] Manual testing guide for Postman flows
- [ ] `verify-full-system.sh` exit 0 — blocked on postman git drift (see §13)

---

## 2. Commands run

See `docs/reports/verification/FULL_SYSTEM_FINAL_VERIFICATION_REPORT.md` for full command list and outputs.

Key:

```bash
go test -count=1 ./...                         # PASS
bash scripts/audit/verify-uuid-v7.sh           # PASS
bash scripts/local/verify-full-system.sh       # FAIL (postman-drift)
VERIFY_WITH_DB=1 TEST_DATABASE_URL=... \
  bash scripts/local/verify-full-system.sh     # 20/21 pass
```

---

## 3. Test results

| Suite | Result |
|-------|--------|
| Go unit + integration (default) | **PASS** |
| UUID v7 audit | **PASS** |
| Migration offline safety | **PASS** |
| Fresh DB migrate | **PASS** |
| OpenAPI / Postman artifact check | **PASS** |
| Proto / sqlc drift | **PASS** |
| verify-full-system.sh | **FAIL** (postman-drift only) |

---

## 4. Files changed (high level)

| Area | Files |
|------|-------|
| UUID v7 | `internal/platform/id/`, `migrations/00005_*`, `db/schema/01_platform.sql`, tests |
| Migration fix | `migrations/00005_uuid_v7_defaults.sql` (StatementBegin) |
| Audit scripts | `scripts/audit/verify-uuid-v7.sh`, `scripts/local/verify-full-system.sh` |
| Postman | `postman/collections/*.json`, `tools/build_postman_collection.py` |
| Integration test | `internal/modules/postgres/integration_test.go` |
| Docs | `docs/testing/*.md` (12 reports + manual guide) |
| Prior uncommitted work | Production migration, deploy workflows, repo layout (see `git status`) |

---

## 5. UUID v7 status

**PASS** — static audit clean; DB defaults v7; Go production paths use `id.NewUUIDV7()`.

---

## 6. REST status

**PASS** — 266 paths / 327 operations; OpenAPI verify pass; handler tests green.

---

## 7. gRPC status

**PASS** — proto generate/lint/drift clean; `internal/grpcserver` tests pass.

---

## 8. MQTT status

**PASS (contract + unit)** — live broker E2E optional (`VERIFY_WITH_BROKER=1`).

---

## 9. Postman status

**PASS (validation)** — JSON valid, artifact checks pass.  
**PENDING (git drift)** — regenerated collection not yet committed → `postman-drift` fails.

---

## 10. Migration status

**PASS** — fresh DB to version 5; UUID v7 function + 91 column defaults applied.

---

## 11. Production deployment gate status

**PASS (static)** — `deploy-prod.yml` manual dispatch, digest-pinned images, `run_migration` default true, migrate-before-rollout on node A, smoke gates, no secret echo in scripts reviewed.

---

## 12. Remaining risks

| Risk | Mitigation |
|------|------------|
| Postman uncommitted drift | Commit collection/env from this branch |
| `rg` missing on Windows dev hosts | CI runs placeholder check |
| Live MQTT/gRPC/Newman not run locally | Optional verify flags documented |
| 45m E2E suite not re-run this pass | `VERIFY_DESTRUCTIVE=1` when needed |
| Real PSP / hardware vend | Manual/external — documented in E2E report |

---

## 13. Manual / external tests still required

- Production workflow dispatch with real GHCR digests + Supabase backup/migrate
- Newman full collection against staging with operator credentials
- Physical machine vend + motor ACK
- Real payment provider webhook in sandbox/production

---

## 14. Final verdict

### NOT READY FOR MERGE (one blocker)

**Blocker:** `bash scripts/local/verify-full-system.sh` exits **1** because `postman-drift` detects uncommitted Postman UUID v7 collection changes. Stage and commit Postman artifacts (and migration 00005 fix) with this branch, then re-run:

```bash
bash scripts/local/verify-full-system.sh
# Expected: exit 0 after Postman files committed
```

**Otherwise ready:** All Go tests pass, UUID v7 enforced, migrations work on fresh DB, API contracts verified, documentation complete.

### After committing Postman + migration fix

Re-run full checklist → expected verdict: **READY FOR MERGE**
