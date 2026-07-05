# Production Test Report — Machine-Code Activation

Date: 2026-07-06  
Report UTC: `20260706T034900Z`  
Production base URL: `https://api.ldtv.dev`

## Production SHA (pre-deploy verification)

| Check | Result |
|-------|--------|
| `GET /version` | `git_sha`: `22e56f0f972cc94031d95371ce79007f57cf6fb8` |
| Deploy tag | `v20260705-22e56f0` (machine-code activation from PR #423) |

Post-merge deploy (PR #426) pending — see deployment section in final verdict.

---

## Preflight

| Test | Status | Evidence |
|------|--------|----------|
| GET /health/live | **pass** | activation smoke + full suite |
| GET /health/ready | **pass** | activation smoke |
| GET /version | **pass** | activation smoke |
| POST /v1/auth/login | **pass** | bootstrap + smoke |
| GET /v1/auth/me | **pass** | smoke |

---

## Activation-by-machineCode smoke

Runner: `tools/production_full_test/run_machine_code_activation_prod.py`  
Evidence: `docs/reports/machine-code-activation-production/evidence/activation_smoke_results.json`

Isolated test machine created with 6-digit code `AVF156537` (activation resolver requires `^AVF[0-9]{6}$`).

| Test | Status |
|------|--------|
| ensure_activation_test_machine | **pass** |
| POST /machine-codes/{code}/activation-codes | **pass** |
| POST /machines/{uuid}/activation-codes | **pass** |
| POST /machines/{code}/activation-codes | **pass** |
| POST /activation-codes body machineCode | **pass** |
| POST /activation-codes body machine_code | **pass** |
| GET /machine-codes/{code}/activation-codes | **pass** (no plaintext/hash) |
| DELETE /machine-codes/{code}/activation-codes/{id} | **pass** |
| POST /setup/activation-codes/claim | **pass** (`deviceAttachmentId` present) |

**Activation smoke fail_count: 0**

---

## Full REST / gRPC / MQTT production suite

Runner: `tools/production_full_test/run_production_full_suite.py --passes 1`  
Evidence: `reports/production-full-api-grpc-mqtt/20260706T034900Z/`

### Counts

| Surface | total | passed | failed | skipped | blocked | not_run |
|---------|-------|--------|--------|---------|---------|---------|
| **REST** | 363 | 363 | 0 | 0 | 0 | 0 |
| **gRPC** | 75 | 75 | 0 | 0 | 0 | 0 |
| **MQTT** | 17 | 17 | 0 | 0 | 0 | 0 |

### Additional gates

| Gate | Result |
|------|--------|
| DB state verification | **pass** (fail_count=0) |
| Security auth tests | **pass** (fail_count=0) |
| Fake pass audit | **pass** (no fake pass risk) |

### E2E flows (canonical runner)

| Flow | Result | Notes |
|------|--------|-------|
| A–H (7 flows) | **pass** | Admin setup, MQTT, bootstrap, lifecycle, reattach, compromised |
| I Offline replay idempotency | **fail** | Pre-existing: activation code invalid on second claim — **not machine-code activation regression** |

Canonical runner verdict: `BLOCKED_BY_VERIFICATION_GAPS` (E2E flow I + single pass vs 3-pass policy). **REST/gRPC/MQTT surface coverage: 100% pass, 0 fail.**

---

## Board replacement

| Check | Result |
|-------|--------|
| Local integration test | **pass** (requires DB) |
| Production claim returns `deviceAttachmentId` | **pass** (activation smoke) |
| Production board replacement inspection | **limited** — attachment created; full A/B fingerprint swap not run as dedicated prod test |

---

## Secret handling

All reports redact tokens, MQTT passwords, and activation plaintext. Evidence JSON stores metadata only.
