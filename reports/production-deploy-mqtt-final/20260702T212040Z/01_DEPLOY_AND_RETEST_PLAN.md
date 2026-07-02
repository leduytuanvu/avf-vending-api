# Deploy and Retest Plan — MQTT 100% Pass

**UTC:** 20260702T212040Z  
**Test prefix:** `ENTERPRISE_PROD_MQTT_DEPLOY_TEST_20260702T212040Z_`

---

## P0 — Re-audit and local validation

| Step | Command | Pass | Evidence |
|------|---------|------|----------|
| Re-audit | Read source + prior reports | 16 answers documented | `00_PRE_DEPLOY_REAUDIT.md` |
| Local tests | `go test ./internal/platform/emqxadmin/... ./internal/app/activation/... ./internal/app/fleet/... -count=1` | exit 0 | `02_LOCAL_PRE_DEPLOY_VALIDATION.*` |
| Validators | `python tools/enterprise_flow/validate_*_surface.py` | all OK | `02_*` |
| Secret scan | `git diff` — no real secrets | clean | `02_*` |

**Fail:** stop — no commit/deploy.

---

## P0 — Commit and merge

| Step | Command | Pass | Evidence |
|------|---------|------|----------|
| Stage | Intended files only (no `.env*`) | no secrets staged | `03_COMMIT_PUSH_MERGE_RESULT.md` |
| Commit | `feat(api): provision machine-scoped EMQX MQTT credentials` | hook pass | `03_*` |
| Merge | develop → main, push both | CI green | `03_*` |
| Parity | `git diff origin/develop..origin/main` | empty tree | `03_BRANCH_PARITY_RESULT.md` |

**Rollback:** revert commit on branch before push.

---

## P0 — EMQX production config

| Step | Action | Pass | Evidence |
|------|--------|------|----------|
| App nodes | Set `EMQX_*` in `.env.app-node` | curl status OK from container | `04_EMQX_PRODUCTION_CONFIG_CHECKLIST.md` |
| Data-node | ACL + `authorization.enable=true` | ACL negatives pass in smoke | `04_*` |
| Security | `:18083` not public | firewall verified | `04_*` |

**Fail:** `BLOCKED_BY_EMQX_MANAGEMENT_PUBLIC_RISK` or `BLOCKED_BY_EMQX_ENV_MISSING`.

---

## P0 — Deploy production

```powershell
gh workflow run deploy-prod.yml --ref main -f action_mode=deploy -f release_tag=mqtt-provision-20260702 ...
gh run watch <run-id>
```

| Check | Pass | Evidence |
|-------|------|----------|
| `/health/live` | 200 | `05_PRODUCTION_DEPLOY_RESULT.*` |
| `/health/ready` | 200 | `05_*` |
| `/version` SHA | matches main | `05_*` |

**Rollback:** deploy-prod rollback mode with prior digests.

---

## P0 — MQTT unblock smoke

```powershell
$env:PRODUCTION_FULL_TEST_UTC = "20260702T212040Z"
$env:PRODUCTION_FULL_TEST_STRICT = "1"
python tools/production_full_test/run_mqtt_unblock_smoke.py --base-url https://api.ldtv.dev
```

Pass: claim returns mqtt creds; connect OK; ACL negatives pass.  
Evidence: `06_MQTT_UNBLOCK_SMOKE_RESULT.*`

---

## P0 — Full 3-pass suite

```powershell
$env:PRODUCTION_FULL_TEST_UTC = (Get-Date -AsUTC -Format "yyyyMMddTHHmmssZ")
$env:PRODUCTION_FULL_TEST_STRICT = "1"
$env:PROD_TEST_ADMIN_EMAIL = "admin@avf.com"
$env:PROD_TEST_ADMIN_PASSWORD = "<session-only>"
python tools/production_full_test/run_production_full_suite.py --base-url https://api.ldtv.dev --passes 3
```

Output: `reports/production-full-api-grpc-mqtt/<NEW_UTC>/FINAL_PRODUCTION_REST_GRPC_MQTT_VERDICT.*`

---

## P0 — E2E flows A–I

```powershell
python tools/production_full_test/run_production_e2e_flows.py --base-url https://api.ldtv.dev
```

Evidence: `07_REAL_FLOW_E2E_RESULT.*`

---

## P1 — Fix loop

On failure → `FAILURE_TRIAGE.*` → fix → merge → deploy → smoke → full suite.

---

## P2 — Final verdict

`FINAL_DEPLOY_AND_PRODUCTION_100_PERCENT_VERDICT.*` — 34 audit answers.

**PASS only if:** REST/gRPC/MQTT/DB/security fail=0; fakePassRisk=false; 3/3 multi-pass; deployed SHA proven; ACL negatives pass.
