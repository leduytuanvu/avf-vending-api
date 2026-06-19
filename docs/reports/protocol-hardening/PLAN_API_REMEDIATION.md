# AVF Protocol Hardening — API Remediation Plan

**Status:** Implementation in progress (2026-06).  
**Scope:** `avf-vending-api` machine commerce vend finalization for cash-only vending.

## Goal

Backend must satisfy AVF Protocol Hardening:

1. No double order finalization / inventory decrement.
2. No vend success without real hardware evidence correlation (when flag ON).
3. Idempotent replay/conflict on machine mutations.
4. Failure/ambiguous routes to refund/reconciliation; never marks success.
5. Outbox publishes vend/cash/refund/reconciliation events exactly once.

## Design decisions (owner-approved)

| Decision | Choice |
|----------|--------|
| Enforcement | `COMMERCE_REQUIRE_VEND_HARDWARE_EVIDENCE`: ON → reject; OFF → complete with `verification_status=hardware_unverified` |
| Evidence storage | Table `vend_hardware_evidence` + `vend_sessions.verification_status` |

## Requirement map

| ID | Requirement | Primary files | DoD | Tests |
|----|-------------|---------------|-----|-------|
| R1 | Structured hardware evidence on wire | `proto/avf/machine/v1/commerce.proto` | `make proto` + `api-contract-check` green; optional fields back-compat | `machine_grpc_production_contract_test.go` |
| R2 | Domain validation | `internal/domain/commerce/vend_evidence.go` | `Validate(cashFlow)` pure, no IO | `vend_evidence_test.go` |
| R3 | Feature flag + outbox config | `internal/config/config.go` | Env parsed, plumbed to gRPC deps | config parse tests |
| R4 | Schema | `migrations/00014_vend_hardware_evidence.sql`, `db/schema/01_platform.sql`, `db/queries/commerce_vend_evidence.sql` | `make sqlc`, `check-migrations` | postgres integration |
| R5 | Finalize enforcement | `machine_commerce_grpc.go`, `service.go`, `commerce_vend_fulfillment.go`, `types.go` | Flag ON rejects evidence-less; one finalize/decrement on replay | grpc + postgres integration |
| R6 | Outbox exactly-once | `db/queries/reliability.sql`, fulfill tx | Duplicate finalize → single outbox row | postgres integration |
| R7 | Idempotency hash includes evidence | `machineidempotency/hash.go` (existing) | Different evidence → different hash | `hash_test.go` |

## Verification commands

```bash
make fmt-check vet test-short
make proto api-contract-check
make sqlc check-migrations check-uuid-v7 check-pgcrypto
go test ./internal/domain/commerce/... ./internal/app/machineidempotency/... ./internal/grpcserver/... ./internal/modules/postgres/...
```

## Reporting

See [`FINAL_API_REPORT.md`](FINAL_API_REPORT.md) for per-requirement status after implementation.
