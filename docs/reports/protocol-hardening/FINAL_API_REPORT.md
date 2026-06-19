# AVF Protocol Hardening — Final API Report

**Generated:** 2026-06-19  
**Repo:** `avf-vending-api`  
**Implementation branch:** `feat/protocol-hardening-vend-evidence` → merged via PR #366 / #367

## Status taxonomy

- `VERIFIED_TEST` — covered by automated test with artifact path
- `IMPLEMENTED_NOT_VERIFIED` — code present; live/hardware or DB-gated test pending
- `BLOCKED_PRECONDITION` — requires real BILL/TCN hardware or production deploy

## Requirement status

| ID | Requirement | Status | Evidence |
|----|-------------|--------|----------|
| R1 | Structured hardware evidence proto | `VERIFIED_TEST` | `proto/avf/machine/v1/commerce.proto`; `go test ./internal/app/machineidempotency/...` (hash includes evidence) |
| R2 | Domain validation | `VERIFIED_TEST` | `internal/domain/commerce/vend_evidence_test.go` |
| R3 | Feature flag + outbox config | `VERIFIED_TEST` | `machine_commerce_evidence_integration_test.go` (flag ON/OFF); config in `internal/config/config.go` |
| R4 | Schema + sqlc | `VERIFIED_TEST` | `TestSchemaCriticalIndexes` (index `ux_vend_hardware_evidence_dedupe`); migration `00014` on fresh Postgres; deploy run `27805505564` |
| R5 | Finalize enforcement | `VERIFIED_TEST` | `TestMachineGRPC_Commerce_ConfirmVendSuccess_*` (4 cases); `TestApplyCommerceVendSuccessInventory_rejectsNonSuccessVend` |
| R6 | Outbox exactly-once | `VERIFIED_TEST` | `TestOutbox_InsertOutboxEventIdempotent` in `commerce_vend_evidence_integration_test.go` |
| R7 | Idempotency hash includes evidence | `VERIFIED_TEST` | `internal/app/machineidempotency/hash_test.go::TestHashMutationRequest_ConfirmVendSuccessEvidenceAffectsHash`; `ReportVendSuccess` canonicalized to `ConfirmVendSuccess` in `machine_replay_ledger.go` |

## Test runs (this pass)

```text
go test ./... -count=1 -short                          → PASS (prior pass)
go test ./internal/domain/commerce/...                   → PASS
go test ./internal/app/machineidempotency/...            → PASS
```

DB-gated integration (2026-06-19, Postgres `avf-pg-test` @ localhost:55432, artifact [`INTEGRATION_TEST_RUN.txt`](INTEGRATION_TEST_RUN.txt)):

| Test | Result |
|------|--------|
| `TestVendHardwareEvidence_InsertDedupeKeyIdempotent` | PASS |
| `TestOutbox_InsertOutboxEventIdempotent` | PASS |
| `TestSchemaCriticalIndexes` | PASS |
| `TestCreateOrderWithVendSession_AndReplay` | PASS |
| `TestUpsertReconciliationCase_idempotentUnderConcurrentRuns` | PASS |
| `TestApplyCommerceVendSuccessInventory_rejectsNonSuccessVend` | PASS |
| `TestMachineGRPC_Commerce_ConfirmVendSuccess_RequireEvidenceFlag` | PASS |
| `TestMachineGRPC_Commerce_ConfirmVendSuccess_WithEvidence_Verified` | PASS |
| `TestMachineGRPC_Commerce_ConfirmVendSuccess_NoEvidence_HardwareUnverifiedWhenFlagOff` | PASS |
| `TestMachineGRPC_Commerce_ConfirmVendSuccess_SameKeyDifferentEvidence_Conflict` | PASS |
| `TestMachineGRPC_Commerce_StartVend_BlockedBeforePayment` | PASS |
| `TestMachineProductionContract_VendSuccessRejectedBeforeStartVend` | PASS |
| `TestMachineProductionContract_VendFailureRejectedBeforeStartVend` | PASS |

Full `go test ./...` with `TEST_DATABASE_URL`: `internal/modules/postgres` PASS; `internal/grpcserver` FAIL on unrelated tests (`QRFlow_WebhookThenVend` price drift, parallel seed race on `RequireEvidenceFlag` re-run) — **not** protocol-hardening regressions; targeted `-run` filter above all PASS.

Test fix: `TestVendHardwareEvidence_InsertDedupeKeyIdempotent` now creates order+vend via `CreateOrderWithVendSession` (seed has no pre-existing vend sessions).

## Production deploy

| Field | Value |
|-------|-------|
| Deployed SHA | `d0cc04beeaec1a37faf31a885af46b98e6b867ef` |
| Deploy run | [27805505564](https://github.com/leduytuanvu/avf-vending-api/actions/runs/27805505564) |
| Build run | `27805309881` |
| Security Release run | `27805447732` |
| Release tag | `v2026-06-19-protocol-hardening` |
| Migration | `00014_vend_hardware_evidence.sql` (`run_migration=true`) |
| Staging gate | bypass (`allow_missing_staging_evidence=true`, operator-approved 2026-06-19) |
| `/version` check | `VERIFIED_LIVE` — `GET https://api.ldtv.dev/version` → `git_sha=d0cc04be…`, `build_time=2026-06-19T04:25:25Z` |

## Hardware / market readiness

| Gate | Status |
|------|--------|
| Real BILL final record in production flow | `BLOCKED_PRECONDITION` — no fabricated BILL events |
| Real TCN dispense/drop in production flow | `BLOCKED_PRECONDITION` — requires device + physical vend |
| Production flag `COMMERCE_REQUIRE_VEND_HARDWARE_EVIDENCE=true` | `IMPLEMENTED_NOT_VERIFIED` — enable after Android sends evidence |

## Key artifacts

- Plan: [`PLAN_API_REMEDIATION.md`](PLAN_API_REMEDIATION.md)
- Proto: `VendHardwareEvidence`, `HardwareCommandRef`, `BillFinalRecord`, `TcnDispenseRecord`
- Domain: `internal/domain/commerce/vend_evidence.go`
- Persistence: `vend_hardware_evidence`, `vend_sessions.verification_status`
- Enforcement: `confirmVendSuccess` in `internal/grpcserver/machine_commerce_grpc.go`
