# E2E remediation playbook

This playbook covers two different outcomes from a run:

1. **Hard failures** — the harness could not complete the scenario as written; fix before expecting a green pass.
2. **Flow improvement findings** — the scenario may still **pass**, but the flow, API, docs, data shape, or protocol should be improved; tracked separately in improvement reports.

---

## Hard failure vs improvement finding

### Hard failure (must fix before pass)

Use **`reports/remediation.md`** and **`events.jsonl`** as the source of truth.

- An **API / RPC / MQTT topic** call did not succeed as the scenario requires (wrong HTTP status, gRPC error, broker unreachable, timeout, etc.).
- The **expected state** was not reached (assertion failed, resource missing, wrong body shape for the check).
- The **test harness or environment** is broken in a way that prevents the step from completing (bad token, wrong `BASE_URL`, missing tool).

Until these are addressed, treat the run as **failed** regardless of improvement debt.

### Improvement finding (test may still pass)

Use **`improvement-findings.jsonl`**, **`reports/improvement-summary.md`**, and **`reports/optimization-backlog.md`**.

- The step or scenario may **exit successfully** or **skip** with a documented reason, but there is still a **product or engineering debt** to record (ambiguous contract, missing doc, idempotency gap, slow path, safety note).
- Findings are **non-blocking** by default (unless **`E2E_FAIL_ON_P0_FINDINGS`** / **`E2E_FAIL_ON_P1_FINDINGS`** causes finalize to exit non-zero).
- Optional skips and brittle workarounds should be logged as **P2/P3** (or **P1** if they block reliable automation), not omitted.

---

## Severity (P0–P3) for improvement findings

| Severity | Meaning |
|----------|--------|
| **P0** | **Money / inventory / production** risk; behavior that can corrupt ledgers, stock, or payments, or block **vending production reliability**. Treat as **must fix immediately**. Harness default: **`E2E_FAIL_ON_P0_FINDINGS=true`** fails the run at finalize when P0 rows exist (excluding explicit “no findings” markers). |
| **P1** | Blocks a **pilot** or **automation** goal, or indicates a **major correctness / testability** problem (missing RPC, broken idempotency contract, untraceable state). Optional: set **`E2E_FAIL_ON_P1_FINDINGS=true`** to fail the run on P1. |
| **P2** | **Optimization**, ergonomics, **missing docs**, too many round-trips, performance hotspots, protocol friction. |
| **P3** | **Cleanup / naming / minor** improvements (including small Postman or doc touch-ups). |

---

## Improvement artifacts (what each file is)

| File | Purpose |
|------|--------|
| **`improvement-findings.jsonl`** | Append-only JSON Lines: one object per finding (`finding_id`, `severity`, `category`, `flow_id`, `scenario_id`, `symptom`, `impact`, `recommendation`, `evidence_file`, …). Schema: **`tests/e2e/data/improvement-finding.schema.json`**. |
| **`reports/improvement-summary.md`** (mirror at run root) | Human-readable rollup of findings grouped for triage; read this first when the run exited **0** but you want debt visibility. |
| **`reports/optimization-backlog.md`** (mirror at run root) | Checkbox-oriented backlog derived from findings; use for grooming and splitting work across teams. |
| **`reports/flow-review-scorecard.json`** (mirror at run root) | Machine-readable per-flow rollup (counts / scores) for dashboards or CI summaries. |

**Finalize** always regenerates **`improvement-summary.md`**, **`optimization-backlog.md`**, and **`flow-review-scorecard.json`** (even when there are zero findings). **`improvement-findings.jsonl`** is appended during scenarios (and touched at finalize); compare across runs for regressions. Hard failures remain in **`reports/remediation.md`** and **`events.jsonl`**.

---

## After every run (workflow)

1. Open **`reports/summary.md`** (or root **`summary.md`**) for the overall pass/fail/skip picture and environment snapshot.
2. If any step **failed**, open **`reports/remediation.md`** first and fix **hard failures** (infra, tokens, product bugs, harness bugs).
3. Open **`reports/improvement-summary.md`** for **flow / API / docs** issues that did not necessarily fail the run.
4. Open **`reports/optimization-backlog.md`** and turn **P0 / P1 / P2** items into tickets (**backend**, **admin**, **android**, **docs**) with **`finding_id`** and evidence paths from **`.e2e-runs/run-*`**.
5. After fixes land, **rerun** with **`--reuse-data path/to/test-data.json`** when IDs are still valid (or **`--fresh-data`** after collisions or scratch reset).
6. For **review-only** passes (no mutations), use **`./tests/e2e/run-flow-review.sh`** (see **[`e2e-local-test-guide.md`](e2e-local-test-guide.md)**). See **Safe production read-only** below.

### Safe production read-only

- Full E2E phases may perform **writes** when **`E2E_ALLOW_WRITES=true`**; **never** point those at production unless your runbook explicitly allows it and **`E2E_PRODUCTION_WRITE_CONFIRMATION`** is set per **`e2e_common.sh`**.
- **`./tests/e2e/run-flow-review.sh`** always sets **`E2E_ALLOW_WRITES=false`**: **`--static-only`** never calls mutating APIs; **`--reuse-data`** only performs **read-only** GETs and optional **GetBootstrap** gRPC. You may still use it against **production** **only** when (a) organizational policy allows read probes, (b) tokens are **read-scoped**, and (c) responses are treated as sensitive. It does not remove other risks (e.g. logging PII); treat **`.e2e-runs/`** as secret-bearing.

---

## Example findings (illustrative)

| Severity | Example |
|----------|--------|
| **P0** | Duplicate offline event replay creates a **duplicate payment** (idempotency / ledger bug). |
| **P1** | **gRPC** surface missing or **Unimplemented** for **media delta ACK** expected by the vending app contract. |
| **P2** | **Slot assignment** requires **too many API calls**; flow should be batched or documented as a single “publish” snapshot. |
| **P3** | **Postman** request name or folder label is **unclear** vs the canonical path in **[`e2e-flow-coverage.md`](e2e-flow-coverage.md)**. |

---

## Suggested Cursor prompt (from `optimization-backlog.md`)

Use this when you want an agent to implement backlog items without scope creep:

```text
Open optimization-backlog.md from my latest .e2e-runs/run-* directory (or paste the checkboxes).

- Fix **P0** improvement findings first, then P1, then P2. Defer P3 unless trivial.
- Propose a **minimal file scope** (backend / admin / android / docs) per finding; do not change unrelated modules.
- For each P0/P1 item: **acceptance checklist** (behavior, API or proto change, observability), **tests to run** (Go/unit, local E2E command with --reuse-data if applicable).
- Do not refactor or rename unrelated code; keep PRs reviewable.
```

---

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

After **`run-all-local.sh`** or any runner that calls finalize, open the run directory printed on stderr:

| Artifact | Purpose |
|----------|---------|
| **`reports/summary.md`** | What ran, environment snapshot, protocol tables, pass/fail/skip, merged coverage pointers |
| **`reports/remediation.md`** | **Hard failures** only: scenario/step, evidence paths, suggested reruns (**[`# Hard failure vs improvement finding`](#hard-failure-vs-improvement-finding)**) |
| **`reports/coverage.json`** | Machine-readable merge: Postman matrix, gRPC/MQTT JSONL payloads, Phase 8 rows, **`scenarioCoverage`**; **`flowReview`** when **`run-flow-review.sh`** ran |
| **`test-data.redacted.json`** | Same keys as **`test-data.json`** with token-like values masked |
| **`reports/e2e-junit.xml`** | JUnit projection of **`events.jsonl`** for CI dashboards |
| **`reports/e2e-report-context.json`** | Non-secret snapshot of `BASE_URL`, `GRPC_ADDR`, MQTT broker string, write flags |

**Improvement-only outputs** (details: **[# Improvement artifacts (what each file is)](#improvement-artifacts-what-each-file-is)**): **`improvement-findings.jsonl`**, **`reports/improvement-summary.md`**, **`reports/optimization-backlog.md`**, **`reports/flow-review-scorecard.json`** (mirrors at run root).

Tokens may still appear in raw **`rest/*.response.body`** files — treat the whole **`.e2e-runs/`** tree as sensitive.

---

## Related

- **[`e2e-troubleshooting.md`](e2e-troubleshooting.md)**
