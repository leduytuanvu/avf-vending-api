# E2E remediation playbook

This playbook is aimed at **hard failures**: steps recorded as **failed** in `events.jsonl`, non-zero scenario exits, or HTTP/gRPC errors that block the scenario’s assertions.

## Failures vs flow improvement findings

- **Failure:** the harness could not complete the scenario as written (wrong status code, missing resource, broker down, etc.). Fix the environment or the product behavior, then rerun. **`reports/remediation.md`** lists failure rows with evidence paths.
- **Improvement finding:** the flow **passed or was skipped with a documented workaround**, but something about the API, contract, docs, performance, or safety still deserves a ticket. These are appended to **`improvement-findings.jsonl`** (JSON Schema: **`tests/e2e/data/improvement-finding.schema.json`**; each line includes **`created_at_utc`**, **`suggested_owner`**, **`status`**, etc.) and summarized in **`improvement-summary.md`**, **`optimization-backlog.md`**, and **`flow-review-scorecard.json`**. They do **not** replace failure diagnosis — they complement it.

Optional skips and brittle workarounds should be logged as **P2/P3** (or **P1** if they block reliable automation), not omitted.

## Severity (P0–P3) for improvement findings

| Severity | Meaning |
|----------|--------|
| **P0** | Blocks a reliable **production** vending flow or risks **money / inventory** corruption. **Default: fails the run** when **`E2E_FAIL_ON_P0_FINDINGS=true`**. |
| **P1** | Blocks **automated** coverage or can leave **incorrect business state**; should be fixed before trusting CI. Optional run failure: **`E2E_FAIL_ON_P1_FINDINGS=true`**. |
| **P2** | Slows delivery: unclear contracts, extra manual steps, perf hotspots, inconsistent protocols. |
| **P3** | Cleanup, naming, ergonomics, minor doc gaps. |

## When a finding should become a backend (or client) ticket

- **P0 / P1** API contract, idempotency, response shape, inventory/payment correctness → **backend** (or **android** for offline client behavior), with **`finding_id`** and evidence path from the run directory.
- **Docs / Postman only** → **docs** ticket, unless the doc error hides a real API bug (then split).
- Use **`optimization-backlog.md`** as the checklist for backlog grooming; link the **`improvement-summary.md`** section in the ticket.

## Rerun after fixing

1. Fix code/docs as needed; keep **`improvement-findings.jsonl`** from the failing run for comparison.
2. Rerun the same scope, e.g. **`./tests/e2e/run-all-local.sh --reuse-data .e2e-runs/run-…/test-data.json`** when IDs are still valid, or **`--fresh-data`** after collisions.
3. Confirm **failures** are gone in **`events.jsonl`** / **`reports/remediation.md`**, and that new **P0** rows are not added (unless you temporarily set **`E2E_FAIL_ON_P0_FINDINGS=false`** for exploratory runs only).

---

For each failure category: **symptom**, **likely cause**, **where to look**, **log paths** (when harness exists), **safe fix**, **reuse vs fresh data**.

## API not ready

| Field | Detail |
|-------|--------|
| Symptom | Health checks fail; connection refused |
| Likely cause | Service down; port mismatch |
| Where to look | Terminal running API; `GET /health/live` |
| Log file | `.e2e-runs/run-*/rest/health.log` (planned) |
| Safe fix | Start stack per `docs/runbooks/local-dev.md`; verify `8080`/configured port |
| --reuse-data | Yes (infra only) |
| --fresh-data | No |

## Missing admin token

| Field | Detail |
|-------|--------|
| Symptom | 401 on admin routes |
| Likely cause | Login not run; wrong env var |
| Where to look | Postman `admin_token`; `E2E_ADMIN_TOKEN` |
| Log file | `.e2e-runs/run-*/rest/auth.txt` (planned) |
| Safe fix | Re-login; ensure Bearer header set |
| --reuse-data | Yes after refresh |
| --fresh-data | If account locked — new user |

## Machine token invalid

| Field | Detail |
|-------|--------|
| Symptom | gRPC/REST machine auth errors |
| Likely cause | Expiry; rotation |
| Where to look | Admin machine credential version; device logs |
| Log file | `.e2e-runs/run-*/grpc/auth.log` |
| Safe fix | `RefreshMachineToken`; re-activate only if policy allows |
| --reuse-data | Yes after refresh |
| --fresh-data | If machine revoked |

## Activation code already claimed

| Field | Detail |
|-------|--------|
| Symptom | Claim conflict |
| Likely cause | Reused code |
| Where to look | Admin activation list |
| Log file | `.e2e-runs/run-*/grpc/activate.log` |
| Safe fix | Issue new code |
| --reuse-data | No |
| --fresh-data | **Yes** |

## gRPC reflection / proto path

| Field | Detail |
|-------|--------|
| Symptom | grpcurl cannot list services |
| Likely cause | Reflection disabled; wrong imports |
| Where to look | Server config; `proto/` tree |
| Log file | CLI stderr |
| Safe fix | Use explicit proto files from repo |
| --reuse-data | Yes |

## MQTT broker auth

| Field | Detail |
|-------|--------|
| Symptom | Connect or publish denied |
| Likely cause | TLS, ACL, stale creds |
| Where to look | Broker logs; [`mqtt-contract.md`](../api/mqtt-contract.md) |
| Log file | `.e2e-runs/run-*/mqtt/broker-stderr.log` |
| Safe fix | Fix `MQTT_*` env; rotate machine MQTT password version with backend |
| --reuse-data | After credentials fixed |
| --fresh-data | If machine must be re-bound |

## Phase 8 full stack (`run-all-local.sh`, scenarios `40`–`47`)

| Field | Detail |
|-------|--------|
| Symptom | Non-zero exit from **`40_e2e_*.sh`–`47_e2e_*.sh`** after REST/admin/vending/gRPC/MQTT phases; **`reports/phase8-scenario-results.jsonl`** row shows **`result: fail`** |
| Likely cause | Stale **`test-data.json`**; missing **`machineToken`** / **`ADMIN_TOKEN`**; commerce outbox **503** on payment-session; webhook **401/403** (HMAC); MQTT broker down; gRPC not listening; WA-INV / WA-RPT **5xx** |
| Where to look | **`reports/summary.md`** Phase 8 table; per-scenario **`remediation`** string in **`phase8-scenario-results.jsonl`**; **`rest/p8-*.response.json`**, **`grpc/p8-off-*.log`**, **`mqtt/phase8-*.publish.json`** |
| Log file | **`.e2e-runs/run-*/reports/phase8-scenario-results.jsonl`**, **`events.jsonl`** (step **`phase8-E2E-…`**) |
| Safe fix | Fix infra per failure row: payment outbox + **`COMMERCE_PAYMENT_WEBHOOK_SECRET`** (or API **`COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED`** in dev only); **`MQTT_HOST`/`MQTT_PORT`**; **`GRPC_ADDR`**; re-run **`01`** + **`--fresh-data`** when IDs collide |
| --reuse-data | Yes — when org/machine still valid |
| --fresh-data | Yes — after activation collision or corrupted scratch |

## Payment mock

| Field | Detail |
|-------|--------|
| Symptom | Session create fails; no webhook |
| Likely cause | Sandbox keys; tunnel |
| Where to look | API commerce logs; PSP dashboard (test) |
| Log file | `.e2e-runs/run-*/rest/payment.log` |
| Safe fix | Configure PSP test keys; expose webhook URL |
| --reuse-data | Yes for order retry with **new** idempotency key if order abandoned |
| --fresh-data | If PSP customer reference collides |

## Idempotency conflict (409)

| Field | Detail |
|-------|--------|
| Symptom | 409 / gRPC aborted |
| Likely cause | Key reuse with different body |
| Where to look | Response `requestId`; DB ledger |
| Log file | rest/grpc transcripts |
| Safe fix | New key **only** for new logical operation |
| --reuse-data | Yes |
| --fresh-data | Rarely — if DB stuck |

## Inventory insufficient

| Field | Detail |
|-------|--------|
| Symptom | Cannot create order / vend |
| Likely cause | Zero stock |
| Where to look | Admin inventory |
| Log file | rest inventory calls |
| Safe fix | Stock adjustment or refill |
| --reuse-data | Yes |

## Command timeout (MQTT)

| Field | Detail |
|-------|--------|
| Symptom | Pending command; no ACK |
| Likely cause | Device offline; topic typo |
| Where to look | `command_ledger`; [`mqtt-command-debug.md`](../runbooks/mqtt-command-debug.md) |
| Log file | `.e2e-runs/run-*/mqtt/*.log` |
| Safe fix | Bring device online; fix ACL; cancel/retry per admin |
| --reuse-data | Yes |

## Phase 4 web admin business (`--full`, scenarios `10`–`13`)

| Field | Detail |
|-------|--------|
| Symptom | Non-zero exit from catalog / inventory / support / reporting scenarios; **fail** in **`reports/wa-module-results.jsonl`**; **`reports/remediation.md`** lists endpoint + **`rest/*`** artifact |
| Likely cause | Weak **`test-data.json`** (no **`productId`** / **`planogramId`**); **403** missing role; commerce **503**; stock **quantityBefore** stale; price book missing product row |
| Where to look | **`reports/summary.md`** (tables by module); **`reports/wa-module-results.jsonl`**; **`test-events.jsonl`** |
| Log file | **`rest/*.response.json`**, **`*.request.json`**, **`*.meta.json`** |
| Safe fix | Run **`--setup-only`** or **`--reuse-data`** with a full Phase 3 capture; re-export **`ADMIN_TOKEN`**; **`E2E_TARGET=production`** skips order/cancel/refund/cash **POST** mutations |
| --reuse-data | Yes — reuse IDs; re-login if JWT not in **`secrets.private.json`** |
| --fresh-data | Yes — avoids SKU / idempotency collisions |

## Web admin setup (`run-web-admin-flows.sh` / WA-SETUP-01)

| Field | Detail |
|-------|--------|
| Symptom | Early **exit 2** (“E2E_ALLOW_WRITES”); **fail** on auth; **skip** rows in `test-events.jsonl` for planogram/operator/inventory |
| Likely cause | Writes off; missing token/org; org has no published planogram; operator login disallowed |
| Where to look | **`test-events.jsonl`**; **`rest/wa-*.response.json`**; **`reports/summary.md`** (when run standalone) |
| Log file | **`.e2e-runs/run-*/rest/*.meta.json`**, **`*.response.json`**, **`*.request.json`** |
| Safe fix | Enable **`E2E_ALLOW_WRITES`**; set **`ADMIN_TOKEN` + `E2E_ORGANIZATION_ID`** or email/password login; seed planogram; see **`docs/testing/e2e-test-data-guide.md`** (Web Admin section) |
| --reuse-data | Yes — reuse **`organizationId` / `siteId` / `machineId`** to avoid duplicate machines |
| --fresh-data | Yes — new sites/machines/SKUs when IDs collide or activation conflicts |

## Offline replay conflict

| Field | Detail |
|-------|--------|
| Symptom | Sequence / gap errors |
| Likely cause | Out-of-order upload |
| Where to look | Offline queue export; server logs |
| Log file | grpc offline sync |
| Safe fix | Resync full queue or reset scratch machine |
| --reuse-data | Sometimes |
| --fresh-data | **Often** for clean sequence |

---

## Run artifacts (`reports/` after finalize)

After **`run-all-local.sh`** completes (success or failure), open the run directory printed on stderr:

| Artifact | Purpose |
|----------|---------|
| **`reports/summary.md`** | What ran, environment snapshot, protocol tables, pass/fail/skip, merged coverage pointers |
| **`reports/remediation.md`** | One section per failure: **failure_id**, scenario/step, protocol, endpoint/RPC/topic, expected vs actual (redacted), evidence file, likely cause, suggested fix, **--reuse-data** safety, safe rerun command |
| **`reports/coverage.json`** | Machine-readable merge: Postman matrix, gRPC/MQTT JSONL payloads, Phase 8 rows, **`scenarioCoverage`** (harness step prefixes + Phase 8 outcomes) |
| **`test-data.redacted.json`** | Same keys as **`test-data.json`** with token-like values masked |
| **`reports/e2e-junit.xml`** | JUnit projection of **`events.jsonl`** for CI dashboards |
| **`reports/e2e-report-context.json`** | Non-secret snapshot of `BASE_URL`, `GRPC_ADDR`, MQTT broker string, write flags |
| **`improvement-findings.jsonl`** | Flow/API/design debt logged during the run (see intro above) |
| **`improvement-summary.md`** | Human-readable rollup of findings (also under run root) |
| **`optimization-backlog.md`** | Checkbox backlog by severity |
| **`flow-review-scorecard.json`** | Per-flow scores and finding counts |

Tokens may still appear in raw **`rest/*.response.body`** files — treat the whole **`.e2e-runs/`** tree as sensitive.

---

## Related

- **[`e2e-troubleshooting.md`](e2e-troubleshooting.md)**
