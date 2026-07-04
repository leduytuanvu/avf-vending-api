# Market Readiness Final — Post-Pass Re-Audit

**UTC:** 20260704T001000Z  
**Auditor:** readonly git / live API / bundle / cleanup scan  
**Gate:** Phase 1 harness work authorized after this document.

---

## 20-Question Re-Audit

| # | Question | Answer |
|---|----------|--------|
| 1 | What is live production `/version` git SHA? | `51485f5583a4f550cfe6fdb6e529e7339daad9ca` (`app_env=production`, build `2026-07-03T23:26:39Z`) |
| 2 | Do `/health/live` and `/health/ready` return 200? | Yes — both return `ok` (verified 2026-07-04) |
| 3 | Are `origin/develop` and `origin/main` identical? | **No** — `develop=8991f526`, `main=51485f55`; diff is PR #411 timeline SQL hotfix (2 files) |
| 4 | What is the prior runtime-fleet verdict? | `PRODUCTION_REST_GRPC_MQTT_RUNTIME_FLEET_100_PERCENT_PASS` ([`05_FINAL_VERDICT.json`](../machine-runtime-fleet-next/05_FINAL_VERDICT.json)) |
| 5 | Is prior bundle `reports/production-full-api-grpc-mqtt/20260703T234500Z/` on disk? | **No** — directory missing locally (0 files); verdict docs reference it |
| 6 | What harness gaps were documented after runtime-fleet pass? | Full fingerprint reattach, direct Postgres assertions, technician RBAC negatives, fleet filter + timeline exhaustive checks, strict security (no skip-as-pass) |
| 7 | Is `PROD_DATABASE_URL` available for direct SQL? | Session-only operator env (not in repo); required for `verify_db_direct.py` |
| 8 | Will market phase use strict mode? | Yes — `MARKET_READINESS_STRICT=1`, `PRODUCTION_FULL_TEST_STRICT=1`, no skipped-as-pass |
| 9 | What migrations are on production? | `00017` / `00018` (runtime fleet deploy run `28686916171`) |
| 10 | What is the recorded pre-destructive backup? | `backup-20260703T230257Z.dump` (inline deploy backup) |
| 11 | What REST Chi-only fleet routes were missing from harness? | `ops-overview`, `device-attachments/*`, `timeline/unified`, `reattach-device`, runtime/app session reads |
| 12 | Does `security_auth_tests.py` skip roles as pass today? | Yes in non-strict mode; fixed to **fail** under market strict |
| 13 | What timeline event types does SQL emit? | e.g. `device.attachment.attached`, `device.attachment.replaced`, `runtime.app_session.started`, … (see `machine_ops_timeline.sql`) |
| 14 | What fingerprint fields must the matrix cover? | All keys in `DeviceIdentityFromFingerprint` (`overview.go`): androidId, boardSerial, simIccid, appBuildSha, camelCase + snake_case |
| 15 | Current git branch? | `docs/runtime-fleet-prod-verify` (local) |
| 16 | Working tree anomalies? | Many deleted `reports/enterprise-flow-verification/20260703T013119Z/*` (local, not committed) |
| 17 | Safe cleanup candidates (gitignored)? | `.env`, `.pytest_cache/`, `ci-reports/`, `deployments/prod/backups/`, old `docs/testing/production-e2e/RESULTS_*` per `git clean -ndX` |
| 18 | Is PR #412 (runtime-fleet reports) open? | Yes — reports + harness toward `develop` |
| 19 | Can market readiness reuse runtime-fleet harness base? | Yes — `tools/production_full_test/` with new `tools/market_readiness/` orchestrator |
| 20 | Proceed to Phase 1 gap closure? | **Yes** — live SHA matches hotfix; gaps are harness-only unless prod failures prove bugs |

---

## Readonly command capture

```
origin/main  = 51485f5583a4f550cfe6fdb6e529e7339daad9ca
origin/develop = 8991f526d883fd6cfbc996ba5a4affbd558ae02d
git diff origin/develop..origin/main --stat → machine_ops_timeline.sql (2 files)
/version → git_sha 51485f55, app_env production
/health/live → ok
/health/ready → ok
reports/production-full-api-grpc-mqtt/ → MISSING locally
tmp/ → exists (gitignored)
```

---

## Risk flags for market phase

1. **Strict REST** will fail operations that previously passed via permissive `expected_ok_status()` — expect triage volume.
2. **Missing local evidence bundle** — market run must produce fresh `reports/production-market-readiness-final/{UTC}/`.
3. **develop/main divergence** — merge hotfix to `develop` before claiming parity gate.
4. **No retroactive upgrade** of runtime-fleet verdict — market readiness is a separate verdict series.
