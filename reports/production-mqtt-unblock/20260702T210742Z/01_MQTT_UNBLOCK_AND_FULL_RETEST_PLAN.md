# MQTT Unblock and Full Retest Plan

**UTC:** 20260702T210742Z  
**Prior blocker bundle:** `reports/production-full-api-grpc-mqtt/20260702T195405Z/`

## Goal

Achieve `PRODUCTION_REST_GRPC_MQTT_100_PERCENT_PASS` by provisioning per-machine EMQX users on claim/reattach, wiring lifecycle revoke/rotate, verifying broker ACL, and extending the production harness with auth/ACL negatives plus E2E flows A–I.

---

## Phase P0 — Code (this repo)

| Step | Deliverable | Pass criteria |
|------|-------------|---------------|
| 2.1 | `emqxadmin.UpsertUser`, `DeleteUser`, tests | `go test ./internal/platform/emqxadmin/...` green |
| 2.2 | `machine_mqtt_credentials` sqlc queries | `make sqlc` clean; no plaintext password in DB |
| 2.3 | Claim fail-closed + provision before commit + proto mqtt fields | Unit tests; claim returns creds when EMQX configured |
| 2.4 | Reattach rotates MQTT + JSON fields | Reattach response includes mqttUsername/mqttPassword |
| 2.5 | Fleet compromised/revoke/rotate hooks | EMQX user deleted/rotated on lifecycle |
| 2.6 | `.env.app-node.example` + runbook topology | Documented vars only (no secrets) |
| 4.x | MQTT harness negatives, strict bootstrap, fake-pass, E2E flows | Matrix ≥14 rows; strict bootstrap fails without claim creds |

**Local verification:**

```powershell
cd D:\admin\development\avf\avf-vending-system\avf-vending-api
go test ./internal/platform/emqxadmin/... ./internal/app/activation/... ./internal/app/fleet/... -count=1
go test ./internal/httpserver/... -run TestEnterpriseFlowSecurityRules -count=1
python tools/enterprise_flow/validate_mqtt_surface.py
python tools/enterprise_flow/validate_grpc_surface.py
python tools/enterprise_flow/validate_rest_surface.py
```

---

## Phase P0 — Deploy

**Workflow:** `.github/workflows/deploy-prod.yml` (manual `workflow_dispatch` on `main`)

**App-node `.env.app-node` (both VPSes):**

```env
EMQX_MANAGEMENT_URL=http://187.127.99.153:18083
EMQX_API_KEY=<from secrets manager>
EMQX_API_SECRET=<from secrets manager>
```

**Reachability smoke (from app container):**

```bash
curl -sf -u "$EMQX_API_KEY:$EMQX_API_SECRET" "$EMQX_MANAGEMENT_URL/api/v5/status"
```

**Data-node ACL:**

1. Copy `deployments/prod/emqx/acl.conf.example` → `/opt/emqx/etc/acl.conf` (`TOPIC_PREFIX=avf/devices`)
2. Merge `deployments/prod/emqx/authorization.snippet.hocon`; `authorization.enable = true`
3. Restart EMQX

**Post-deploy:**

```powershell
curl https://api.ldtv.dev/health/live
curl https://api.ldtv.dev/version
```

**Output:** `reports/production-mqtt-unblock/20260702T210742Z/DEPLOY_RESULT.md`

**Rollback:** deploy-prod workflow rollback mode + prior manifest (`docs/runbooks/production-2-vps.md`)

---

## Phase P0 — Production retest

Use **new UTC** (do not reuse `20260702T195405Z`):

```powershell
$env:PRODUCTION_FULL_TEST_UTC = (Get-Date -AsUTC -Format "yyyyMMddTHHmmssZ")
$env:PRODUCTION_FULL_TEST_STRICT = "1"
$env:PROD_TEST_ADMIN_EMAIL = "admin@avf.com"
$env:PROD_TEST_ADMIN_PASSWORD = "<session-only>"
python tools/production_full_test/run_production_e2e_flows.py
python tools/production_full_test/run_production_full_suite.py --base-url https://api.ldtv.dev --passes 3
python tools/production_full_test/write_final_verdict.py
```

| Gate | Criteria |
|------|----------|
| REST | fail=0, untested=0 |
| gRPC | fail=0, untested=0 |
| MQTT | fail=0 on publish tails + subscribe + auth/ACL negatives |
| DB / Security / Fake-pass | fail=0; fakePassRisk=false |
| Multi-pass | 3/3 OK |
| E2E | pass or explicit `BLOCKED_BY_HARDWARE` |

**Outputs:**

- `reports/production-full-api-grpc-mqtt/<NEW_UTC>/FINAL_PRODUCTION_REST_GRPC_MQTT_VERDICT.*`
- `reports/production-mqtt-unblock/<UTC>/FINAL_MQTT_UNBLOCK_AND_FULL_FLOW_VERDICT.*`

---

## Phase P1 — Fix loop

On failure → append `FAILURE_TRIAGE.md/json` with surface, evidence, root cause, fix, retest command. Loop until 100% pass or honest `BLOCKED_BY_*`.

---

## Risk notes

- Provision-before-commit avoids orphaned activations without MQTT user.
- Upsert on HTTP 409 rotates password (required for reattach).
- Public broker URL for devices: `tls://mqtt.ldtv.dev:8883`; internal app publish may use `tcp://187.127.99.153:1883`.
- Redact mqtt passwords in all reports and registry exports.
