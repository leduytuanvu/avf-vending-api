# Pre-Deploy Re-Audit — EMQX MQTT Provisioning

**UTC:** 20260702T212040Z  
**Branch:** `develop` (local HEAD `3f021486155c827d48b7dfb181216ae9dec62a5f`, behind `origin/develop` by 1)  
**Production deployed SHA (prior):** `156fc468fa3c5fec7042e1f656f78b6ea94c2639`  
**Prior MQTT verdict:** `BLOCKED_BY_VERIFICATION_GAPS` (12 MQTT fail)

---

## Audit answers (16 questions)

| # | Question | Answer |
|---|----------|--------|
| 1 | Are all EMQX provisioning code changes present? | **Yes** — `internal/platform/emqxadmin` (`UpsertUser`, `DeleteUser`); activation `provisionMachineMQTT` before commit + `ErrMQTTProvisioning`; reattach rotation; fleet revoke/rotate/compromised hooks; gRPC/REST mqtt fields |
| 2 | Are all local tests still green? | **Pending Phase 2** — targeted packages passed in prior session; re-run before deploy |
| 3 | Are proto/sqlc generated and in working tree? | **Yes** — `machine_activation.proto` fields 14/15; `machine_mqtt_credentials.sql` + `internal/gen/db/machine_mqtt_credentials.sql.go` |
| 4 | Uncommitted changes? | **Yes** — 82 modified + untracked (`emqxadmin/`, `tools/production_full_test/`, reports, sqlc, activation/fleet) |
| 5 | Secrets in git diff? | **No real secrets** — only `CHANGE_ME_*` placeholders in `.env.app-node.example` and field names in code/docs |
| 6 | Current branch? | `develop` |
| 7 | Files needing commit? | EMQX client, activation/reattach/fleet, config/bootstrap, proto, sqlc, docs, `tools/production_full_test/*`, `db/queries/machine_mqtt_credentials.sql`; exclude unrelated enterprise-flow report drift if possible |
| 8 | Required production env vars? | `EMQX_MANAGEMENT_URL`, `EMQX_API_KEY`, `EMQX_API_SECRET` on both app nodes |
| 9 | Which app nodes? | Both app-node VPSes (app-node-a and app-node-b per topology) |
| 10 | Data-node config? | EMQX ACL file + `authorization.enable=true` on data-node `187.127.99.153` |
| 11 | Is EMQX_MANAGEMENT_URL safe/private? | **Intended:** `http://187.127.99.153:18083` on private IP — must verify not publicly reachable (Phase 4) |
| 12 | EMQX management publicly exposed? | **Unknown until operator check** — hard stop if public without firewall |
| 13 | Firewall/VPN protection? | Per runbook: `:18083` internal-only; SSH/VPN for operator access |
| 14 | Exact deploy method? | Manual `workflow_dispatch` on `.github/workflows/deploy-prod.yml` from `main` with build/security run IDs + digest-pinned images |
| 15 | Rollback plan? | Same workflow `action_mode=rollback` with prior digest refs (`docs/runbooks/production-2-vps.md`) |
| 16 | Retest command after deploy? | See `01_DEPLOY_AND_RETEST_PLAN.md` — `$env:PRODUCTION_FULL_TEST_STRICT=1`; `run_production_full_suite.py --passes 3` |

---

## Code verification checklist

| Component | Path | Status |
|-----------|------|--------|
| EMQX Upsert/Delete | `internal/platform/emqxadmin/client.go` | Present |
| Metadata sqlc | `db/queries/machine_mqtt_credentials.sql` | Present |
| Claim fail-closed | `internal/app/activation/service.go` | Present |
| Reattach MQTT | `internal/app/activation/reattach.go` | Present |
| Fleet lifecycle | `internal/app/fleet/mqtt_lifecycle.go`, `admin_crud.go` | Present |
| Proto fields | `proto/avf/machine/v1/machine_activation.proto` | Present |
| Harness strict + negatives | `tools/production_full_test/*` | Present |
| Env example | `deployments/prod/app-node/.env.app-node.example` | Present |

---

## Gate decision

**Proceed to local validation and commit** — do **not** deploy until Phase 2 passes and develop/main parity verified.
