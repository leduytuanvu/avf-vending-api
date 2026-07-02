# Production MQTT Blocker Re-Audit

**UTC:** 20260702T210742Z  
**Prior bundle:** `reports/production-full-api-grpc-mqtt/20260702T195405Z/`  
**Deployed API SHA (prior):** `156fc468fa3c5fec7042e1f656f78b6ea94c2639`

## Executive summary

REST (352), gRPC (71), DB state, security (17+2), and fake-pass audit are green across 3 multi-pass runs. **MQTT is blocked** at broker authentication: no per-machine EMQX user is provisioned during activation claim on production. Local partial fix exists in repo but is **not deployed**.

---

## Audit answers (17 questions)

| # | Question | Answer |
|---|----------|--------|
| 1 | Why did MQTT fail? | No valid per-machine MQTT credentials. Bootstrap used machine JWT or missing password; EMQX built-in DB auth rejects with CONNACK rc=5 or connect timeout. |
| 2 | Which 12/14 topics failed? | All 12 enterprise **publish** tails in `MQTT_FAIL_LIST.md`: `commands/ack`, `commands/receipt`, `presence`, `state/heartbeat`, `telemetry`, `telemetry/snapshot`, `telemetry/incident`, `events`, `events/vend`, `events/cash`, `events/inventory`, `shadow/reported`. Subscribe + negative rows not consistently recorded in fail list (matrix total 12 in final coverage). |
| 3 | Same CONNACK rc=5 root cause? | Yes for auth-phase failures (documented in triage). Latest multi-pass run shows **connect timeout** when runner could not authenticate — same underlying missing/invalid creds. |
| 4 | Topic/payload/ACL failures after auth? | **Not observed** — failures occur before successful connect/publish. ACL must still be verified after auth fix. |
| 5 | Local code implements EMQX provisioning? | **Partial:** `internal/platform/emqxadmin` + `activation.provisionMachineMQTT` on claim only. |
| 6 | Wired into activation claim? | **Yes locally** when `EMQX_API_KEY`/`EMQX_API_SECRET` set on API; **not on production**. |
| 7 | Wired into reattach/rotation? | **No** — `reattach.go` does not call `provisionMachineMQTT`. |
| 8 | Claim response includes mqttUsername/mqttPassword? | **Locally yes** (REST) when EMQX client configured; gRPC proto lacks fields. Production returns broker metadata only. |
| 9 | Bootstrap includes mqtt config? | gRPC/HTTP bootstrap returns `mqtt.brokerUrl`, `topicPrefix`, `topicLayout`; no password; no credential status field yet. |
| 10 | Bootstrap returns mqttPassword? | **No** (by design). Claim may return password once on issue — must redact in logs/reports. |
| 11 | API access to EMQX management in production? | **Not configured** on app-node env today. Data-node EMQX mgmt on private IP `:18083`. |
| 12 | Required production env vars? | `EMQX_API_KEY`, `EMQX_API_SECRET`, `EMQX_MANAGEMENT_URL` (private, e.g. `http://187.127.99.153:18083`). Separate: `MQTT_BROKER_URL`, `MQTT_USERNAME`/`MQTT_PASSWORD` for API/ingest service account. |
| 13 | EMQX ACL per machine? | **File-based** `deployments/prod/emqx/acl.conf.example` with `%u` = machine UUID. Not API-provisioned; must verify `authorization.enable=true` on broker. |
| 14 | MQTT revoke/rotation lifecycle? | **Missing** — no `DeleteUser`/disable in emqxadmin; no fleet hooks. |
| 15 | Cross-machine topic denial tested? | Runner has wrong-machine **publish** negative only; missing cross-machine **subscribe** and JWT-as-password negatives. |
| 16 | Machine JWT as MQTT password tested? | **Not explicitly** in matrix; manually verified rc=5 in triage session. |
| 17 | Remaining gaps? | Deploy env, Upsert on 409, metadata persistence, fail-closed claim, reattach/lifecycle, proto fields, ACL verification, E2E flows A–I, strict bootstrap, expanded verdict. |

---

## Evidence references

- `reports/production-full-api-grpc-mqtt/20260702T195405Z/FAILURE_TRIAGE.md`
- `reports/production-full-api-grpc-mqtt/20260702T195405Z/MQTT_FAIL_LIST.md`
- `reports/production-full-api-grpc-mqtt/20260702T195405Z/MULTI_PASS_PRODUCTION_VALIDATION.md` — Pass 1–3 FAIL (MQTT)
- `internal/platform/emqxadmin/client.go`
- `internal/app/activation/service.go` — `provisionMachineMQTT` after commit (ordering risk)
- `internal/app/activation/reattach.go` — no EMQX
- `deployments/prod/emqx/acl.conf.example`
