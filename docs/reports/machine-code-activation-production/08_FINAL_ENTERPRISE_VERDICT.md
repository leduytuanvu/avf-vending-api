# Final Enterprise Verdict — Machine-Code Activation

Date: 2026-07-06

## Verdict: **GO_WITH_LIMITED_SCOPE**

---

## Security verdict

**Pass.** List responses exclude plaintext activation codes and `codeHash`. Production security auth tests: 0 failures. No secrets in committed reports.

## Data integrity verdict

**Pass.** Activation rows store `machine_id` UUID FK. Board replacement preserves `machineId` and `machineCode`. No FK or identity migration.

## Field usability verdict

**Pass.** Admin can create/list/revoke activation codes using `machineCode` (`AVF######` — exactly 6 digits). Catalog body accepts `machineId`, `machine_id`, `machineCode`, `machine_code` with conflict detection.

## Board replacement verdict

**Pass (limited).** Local integration tests verify fingerprint reuse/replacement. Production claim returns `deviceAttachmentId`. Dedicated production A/B board swap test not run standalone.

## Backward compatibility verdict

**Pass.** UUID paths on `/v1/admin/machines/{machineId}/activation-codes` work with UUID or code in path segment. Runtime JWT/MQTT/gRPC remain UUID-based.

## REST API verdict

**Pass.** 363/363 operations tested, 0 failed, 0 skipped, 0 blocked, 0 not_run.  
Evidence: `reports/production-full-api-grpc-mqtt/20260706T034900Z/REST_FINAL_COVERAGE.json`

## gRPC verdict

**Pass.** 75/75 RPCs tested, 0 failed.  
Evidence: `reports/production-full-api-grpc-mqtt/20260706T034900Z/GRPC_FINAL_COVERAGE.json`

## MQTT verdict

**Pass.** 17/17 tests, 0 failed (includes ACL negatives).  
Evidence: `reports/production-full-api-grpc-mqtt/20260706T034900Z/MQTT_FINAL_COVERAGE.json`

## Production deployment verdict

**Limited.** Deploy run `28755628234` for commit `e57c9486` failed at pre-deploy SLO collection; automatic rollback succeeded. Production serves **`22e56f0f`** which **already includes** machine-code activation (PR #423). `MachineIdentityRef` struct refactor is in `e57c9486` but not yet live.

| Check | Result |
|-------|--------|
| `/health/live` | 200 |
| `/health/ready` | 200 |
| `/version` git_sha | `22e56f0f972cc94031d95371ce79007f57cf6fb8` |

## OpenAPI / Postman / docs verdict

**Pass.** Swagger documents machine-codes routes. Postman e2e manifest includes machineCode path. Reports 00–07 complete.

## Rollback readiness

Production rollback to prior digest succeeded automatically when deploy failed. UUID activation routes verified on current production SHA.

## Remaining risks

1. **Regex divergence:** Fleet bootstrap uses `AVF` + variable digits; activation admin accepts only `^AVF[0-9]{6}$`.
2. **Deploy gap:** `e57c9486` not deployed — struct refactor is internal-only; no API contract change.
3. **E2E flow I:** Offline replay idempotency failure — unrelated to machine-code activation.

## Recommended next steps

1. Resolve pre-deploy SLO gate and redeploy `e57c9486` (optional — internal refactor only).
2. Align bootstrap test machine codes with 6-digit activation format or document operator workflow.
3. Fix E2E flow I offline replay separately.

---

## PR / commit references

| Item | Reference |
|------|-----------|
| Machine-code activation (live on prod) | PR #422/#423, SHA `22e56f0f` |
| MachineIdentityRef + reports | PR #424/#425/#426, SHA `e57c9486` |
| Activation smoke tooling | PR #425 |
