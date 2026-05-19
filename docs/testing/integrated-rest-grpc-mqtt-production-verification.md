# Integrated REST / gRPC / MQTT — production verification guide

**Normative role:** This document is an **evidence-driven verification checklist**, not a substitute for release approval. A **production PASS** verdict is **invalid** unless every row in **§ Pass / fail evidence matrix** records **real artifact paths** (committed CI logs, or archived copies of local outputs) and **actual result = PASS**.

**Companion:** Vietnamese execution order for the large REST collection remains **[`05_PRODUCTION_TEST_EXECUTION_ORDER.md`](05_PRODUCTION_TEST_EXECUTION_ORDER.md)** (gates, headers, folder flow).

---

## 1. Repository snapshot reference

| Field | Value |
|--------|--------|
| **Canonical tree** | Git `HEAD` at verification time |
| **Repomix** | `repomix-output.xml` is **gitignored** in this repo; there is **no** checked-in repomix snapshot. Operators may attach an external repomix bundle to change-control tickets. **Counts below do not depend on repomix** — they come from `docs/swagger/swagger.json` and `python postman/full-production-suite/generate_full_postman_suite.py`. |
| **Count refresh command** | From repo root: `python postman/full-production-suite/generate_full_postman_suite.py` — stdout ends with `openapi_operations`, `postman_requests`, `grpc_templates`, `mqtt_templates`. |

---

## 2. Canonical inventory (regenerated against current tree)

These numbers are **tied to one regeneration**; re-run the generator after OpenAPI/proto/MQTT topic changes.

| Inventory item | Count | Source of truth |
|------------------|------:|-----------------|
| **REST OpenAPI operations** | **325** | `docs/swagger/swagger.json` paths × methods; mirrored as `openapi_operations` in generator stdout |
| **REST operations carrying idempotency metadata** | **89** | `openapi_idempotency_ops` in generator stdout (`generate_full_postman_suite.py`) |
| **Postman REST requests (full suite)** | **325** | `postman/full-production-suite/AVF_REST_365_FULL.postman_collection.json` (leaf requests); equals `postman_requests` in generator stdout |
| **gRPC method templates** | **85** | `postman/full-production-suite/grpc/grpc_request_templates.json` |
| **MQTT topic / flow templates** | **28** | `postman/full-production-suite/mqtt/mqtt_request_templates.json` |

**Parity rule:** `openapi_operations == postman_requests == 325` is the **contract parity gate** for REST. **85 / 28** are template counts for gRPC/MQTT artifacts used by Postman/import tooling; runtime proof still requires **§ Execution evidence** commands below.

---

## 3. Negative, idempotency, and security — explicit test cases

Map harness/collection coverage to **explicit** cases (replace **PARTIAL** adjectives with accountable expectations). Where automation lives only in Postman **folder 15** (`15 Negative - Auth - Permission - Idempotency`), record Newman exports as evidence.

| ID | Case | Protocol | How to execute | Expected signal |
|----|------|-----------|----------------|-----------------|
| **NEG-01** | Missing bearer token | REST | Authenticated admin route **without** `Authorization` | **401** (or documented policy **403**) |
| **NEG-02** | Invalid / expired bearer token | REST | Valid shape, wrong signature or expired JWT | **401** |
| **NEG-03** | Insufficient RBAC role | REST | Token valid but missing permission for gated route | **403** |
| **NEG-04** | Duplicate idempotency key | REST | Same **safe** write twice with identical `Idempotency-Key` | Second call **replays** same outcome (**2xx** + same logical resource id), **no double write** |
| **NEG-05** | Invalid UUID path parameter | REST | Malformed `{uuid}` segment on an existing route | **400** / **404** per handler contract |
| **NEG-06** | Missing required JSON body field | REST | Omit mandatory field on **POST/PATCH** | **400** validation error |
| **NEG-07** | Invalid enum / discriminator | REST | Out-of-range enum string | **400** |
| **NEG-08** | Stable resource not found | REST | Valid UUID, non-existent row | **404** |
| **NEG-09** | Payment webhook invalid HMAC | REST | `POST` PSP webhook with wrong signature | **401** / **403** / **400** per [`payment-webhook-security.md`](../api/payment-webhook-security.md) |
| **NEG-10** | MQTT invalid topic / malformed payload | MQTT | Publish to wrong prefix or invalid JSON envelope | Broker ACL deny / ingest reject / no phantom command — capture broker log + ingest metric |
| **NEG-11** | gRPC unauthenticated | gRPC | Omit `authorization` metadata on protected RPC | **`Unauthenticated`** |
| **NEG-12** | gRPC invalid machine token | gRPC | Wrong bearer on machine surface | **`Unauthenticated`** / **`PermissionDenied`** per service |
| **NEG-13** | gRPC missing idempotency metadata | gRPC | Required idempotent RPC without keys | **`InvalidArgument`** / documented policy |

**Evidence:** Newman JSON/JUnit for REST negatives; `reports/grpc-contract-results.jsonl` / scenario logs for gRPC; MQTT broker/client logs for **NEG-10**.

---

## 4. Execution evidence required (exact commands)

Run from **repository root**. Use **bash** (Git Bash on Windows). Archive outputs you need for audits — **`.e2e-runs/` is gitignored**.

### 4.1 Codegen & contracts

```bash
sqlc generate
python tools/build_openapi.py
python postman/full-production-suite/generate_full_postman_suite.py
```

**Expected:** exit code **0**; generator ends with `VALIDATION_PASS` and counts **325 / 325 / 85 / 28**.

### 4.2 Go quality gates

```bash
gofmt -w $(find . -name '*.go' -not -path './vendor/*')
go vet ./...
go test ./... -count=1
```

**Expected:** `gofmt` silent success; `go vet` **0**; `go test` **0**.

### 4.3 Bash E2E harness

```bash
bash tests/e2e/run-grpc-local.sh
bash tests/e2e/run-mqtt-local.sh
bash tests/e2e/run-all-local.sh --fresh-data
```

**Expected:** each exits **0**; summary lines **Failed: 0** (and **Skipped: 0** unless a documented optional probe skips).

### 4.4 Newman — full REST collection (325 requests)

Preferred wrapper (honors repo paths and reporter layout):

```bash
mkdir -p evidence/rest
POSTMAN_COLLECTION=postman/full-production-suite/AVF_REST_365_FULL.postman_collection.json \
POSTMAN_ENV=postman/full-production-suite/AVF_PRODUCTION.postman_environment.json \
E2E_RUN_DIR="$(pwd)/evidence/rest/run-newman-$(date -u +%Y%m%dT%H%M%SZ)" \
bash tests/e2e/postman/run-newman.sh
```

**Expected:** Newman exit **0**; artifacts under `$E2E_RUN_DIR/rest/` including **`newman-report.json`**, **`newman-junit.xml`**, **`newman-cli.log`**.

Equivalent bare invocation:

```bash
newman run postman/full-production-suite/AVF_REST_365_FULL.postman_collection.json \
  -e postman/full-production-suite/AVF_PRODUCTION.postman_environment.json \
  --reporters cli,json,junit \
  --reporter-json-export evidence/rest/newman-report.json \
  --reporter-junit-export evidence/rest/newman-junit.xml
```

**Gate reminder:** requests that mutate state require Postman env **`allow_destructive`**, **`canaryMode`**, or **`readiness`** per **[`05_PRODUCTION_TEST_EXECUTION_ORDER.md`](05_PRODUCTION_TEST_EXECUTION_ORDER.md)**.

---

## 5. Pass / fail evidence matrix

Fill **`actual result`** and **`evidence path`** only after a real run. **`PENDING`** means **not READY**.

| Group | Command / gate | Expected result | Evidence path (template) | Actual result |
|-------|----------------|-----------------|---------------------------|---------------|
| Codegen | `sqlc generate` | Exit **0** | CI log or `evidence/sqlc.txt` | **PENDING** |
| OpenAPI | `python tools/build_openapi.py` | Exit **0**; `docs/swagger/swagger.json` updated | CI log + diff | **PENDING** |
| Full suite gen | `python postman/full-production-suite/generate_full_postman_suite.py` | `VALIDATION_PASS`; counts **325 / 85 / 28** | CI log | **PENDING** |
| Go format | `gofmt -w $(find . -name '*.go' -not -path './vendor/*')` | No dirty `*.go` needed | `git diff --exit-code` | **PENDING** |
| Go vet | `go vet ./...` | Exit **0** | CI log | **PENDING** |
| Go test | `go test ./... -count=1` | Exit **0** | CI log | **PENDING** |
| REST · Newman full | §4.4 wrapper | Exit **0** | `.e2e-runs/.../rest/newman-report.json` or `evidence/rest/...` | **PENDING** |
| REST · E2E phase | Phase inside `run-all-local.sh` (`rest-local-suite`) | Step **passed** in console | `.e2e-runs/<run>/reports/summary.md`, `coverage-postman.json` | **PENDING** |
| gRPC · E2E | `bash tests/e2e/run-grpc-local.sh` | Exit **0** | `.e2e-runs/<run>/reports/grpc-contract-summary.md` | **PENDING** |
| MQTT · E2E | `bash tests/e2e/run-mqtt-local.sh` | Exit **0** | `.e2e-runs/<run>/reports/mqtt-contract-summary.md` | **PENDING** |
| Integrated | `bash tests/e2e/run-all-local.sh --fresh-data` | Exit **0**; failed **0** | `.e2e-runs/<run>/reports/summary.md` | **PENDING** |
| Negative matrix | **NEG-01 … NEG-13** | Each expected signal observed | Newman / grpc / MQTT logs (attach paths) | **PENDING** |
| Hygiene grep | `bash scripts/ci/git_grep_retired_partition_literals.sh` | Exit **0**, **`PASS: no matches.`** | CI log or `evidence/hygiene-grep.txt` | **PENDING** |

---

## 6. Retired partition identifiers — grep gate

Run from repo root. **Expected:** script exits **0**, prints **`PASS: no matches.`**, and emits **no** matching source lines.

The regex alternation is **assembled at runtime** (pattern fragments are not spelled contiguously in the script body) so the gate stays copy/paste safe and repo scans stay consistent:

```bash
bash scripts/ci/git_grep_retired_partition_literals.sh
```

**Latest verification (documented at guide authoring time):** **`PASS`** — **zero** hits on tracked sources.

**Justification:** Substrings such as the English word “organization” alone are **not** flagged; only the **retired mechanical literals** enforced by the script fail the gate.

---

## 7. Remaining gaps (honest scope limits)

1. **Business-flow matrix** in **[`e2e-flow-coverage.md`](e2e-flow-coverage.md)** still labels some narrative flows **partial** — **325 REST operations** does not guarantee every narrative row is exercised at **PASS** in one harness run.
2. **Production** Newman/E2E requires live **`BASE_URL`**, secrets, broker ACLs, and PSP sandbox alignment — this guide only defines **what evidence must exist**, not network reachability.
3. **NEG-10 / MQTT** may require broker ACL logs outside the repo; attach external artifacts to the evidence bundle.
4. **Repomix** is not versioned here — rely on **git SHA + generator manifest** (`postman/full-production-suite/manifest.json` after regeneration).

---

## 8. Final verdict rule

| Verdict | Condition |
|---------|-----------|
| **NOT_READY** | Any matrix row **PENDING**, missing artifact path, or hygiene grep **FAIL**. |
| **READY_FOR_PRODUCTION_PROOF** | All §5 rows **PASS** with **non-empty evidence paths**, Newman **0** failures, E2E summaries **Failed: 0**, hygiene grep script (**§6**) **PASS**, and release manager sign-off outside this file. |

**Do not** stamp **READY** with only “commands succeeded locally once” unless artifacts are **preserved** for audit (CI artifacts satisfy this).
