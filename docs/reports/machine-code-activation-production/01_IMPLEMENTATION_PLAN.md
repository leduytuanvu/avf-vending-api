# Machine-Code Activation — Implementation Plan

Date: 2026-07-06

## Scope

Minimal enterprise-safe change: admin activation by `machineCode`; runtime UUID unchanged. This plan covers gap closure + full production verification.

---

## Files to change

| File | Change |
|------|--------|
| `internal/app/activation/machineref.go` | Add `MachineIdentityRef`; return struct from resolver |
| `internal/httpserver/activation_http.go` | Update `resolveAdminMachineRef`, catalog create |
| `internal/app/activation/machineref_test.go` | Update for struct return |
| `internal/app/activation/machineref_integration_test.go` | Update + snake_case body test |
| `internal/httpserver/activation_admin_http_test.go` | DELETE-by-code, machines/AVF path, snake_case catalog |

No SQL/proto/MQTT/JWT changes required.

---

## Routes (no new routes)

Existing routes remain; internal resolver returns `MachineIdentityRef`.

---

## SQL queries

No changes — `GetMachineByCode` and list joins already present.

---

## DTOs

- New: `activation.MachineIdentityRef { MachineID, MachineCode }`
- Existing: `CreateResult.MachineCode`, `ListRow.MachineCode`, `ClaimResult.DeviceAttachmentID` unchanged

---

## Tests to add

| Test | File |
|------|------|
| `POST /machines/AVF000301/activation-codes` | `activation_admin_http_test.go` |
| `DELETE /machine-codes/{code}/activation-codes/{id}` | `activation_admin_http_test.go` |
| Catalog `machine_id`, `machine_code` snake_case | `activation_admin_http_test.go` |
| `ResolveMachineBody` by `machine_code` only | `machineref_integration_test.go` |

---

## OpenAPI / Postman / docs

No swagger regen required unless comment-only changes. Postman env already includes `machineCode`, `machineId`, `activationCode`, `activationCodeId`; verify `deviceAttachmentId`, MQTT vars in production-full template during verification.

---

## Production test data

- **Strategy:** Bootstrap isolated machine via `tools/production_full_test/bootstrap_test_data.py` (generates unique `AVF…` code)
- **Fallback:** `TEST_MACHINE_CODE` in `.env.production.e2e.local` only if confirmed isolated
- **Never** modify real customer machines/orders/inventory

---

## Production credentials

Store **only** in gitignored `tests/e2e/production/.env.production.e2e.local`:

```bash
BASE_URL=https://api.ldtv.dev
ADMIN_EMAIL=<redacted>
ADMIN_PASSWORD=<redacted>
E2E_ALLOW_WRITES=true
E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION
GRPC_ADDR=machine-api.ldtv.dev:443
MQTT_HOST=mqtt.ldtv.dev
MQTT_PORT=8883
MQTT_USE_TLS=true
RUN_PREFIX=MCODE-ACT-PROD-YYYYMMDD-HHMMSS
```

**Never commit credentials.**

---

## Production deployment plan

1. Feature branch from `develop`
2. PR → CI green
3. Merge to `develop` → promote to `main`
4. **Deploy Production** workflow
5. Verify `/version` commit SHA matches deployed commit

If no code changes after verification-only pass, skip redeploy and verify existing `22e56f0f`.

---

## Rollback plan

1. Revert gap-fix commit(s) on `main`
2. Run **Deploy Production** rollback to prior manifest digest
3. Confirm:
   - `GET /health/live`, `/health/ready` → 200
   - UUID activation routes still work
   - gRPC health OK
   - MQTT broker auth OK

---

## Risk assessment

| Risk | Mitigation |
|------|------------|
| Customer machine impact | Bootstrap isolated test machine only |
| Credential leak | Gitignored env; redact in all reports |
| Regex `{6}$` vs fleet `{6,}` | Document; activation admin rejects 7+ digit codes by design |
| OpenAPI/Chi count drift | Use production-full runner as source of truth |
| Contract-only gRPC RPCs | Count as passed when `UNIMPLEMENTED` matches accepted list |

---

## Go / no-go criteria

| Verdict | Criteria |
|---------|----------|
| **GO** | Activation-by-machineCode passes; UUID compat passes; REST/gRPC/MQTT required tests: `failed=skipped=blocked=not_run=0`; production SHA verified; no secret leaks |
| **GO_WITH_LIMITED_SCOPE** | All required API surfaces pass; only optional prod board-replacement inspection blocked (local evidence acceptable) |
| **NO_GO** | Any activation failure, UUID regression, required surface failure/skip/block/not-run, deploy mismatch, secret leak |

---

## Execution phases

1. Phase 0–1: Audit + plan (this document)
2. Phase 2: `MachineIdentityRef` + tests
3. Phase 3: Local verification → `03_LOCAL_TEST_REPORT.md`
4. Phase 4: Surface matrix → `04_FULL_SURFACE_TEST_MATRIX.md`
5. Phase 5: Production plan → `05_PRODUCTION_TEST_PLAN.md`
6. Phase 6–7: Production execution + retest loop
7. Phase 8: Final verdict JSON + markdown
