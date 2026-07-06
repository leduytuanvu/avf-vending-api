# Numeric Activation Code — Implementation Report

**Date:** 2026-07-06

## Summary

Implemented strict 6-digit numeric activation codes. Old `AVF-XXXXXX-XXXXXX` format is no longer generated or accepted on claim.

## Files changed

### Core
- `internal/app/activation/code_format.go` — constants, normalize, validate, random generator
- `internal/app/activation/code_format_test.go` — unit tests
- `internal/app/activation/service.go` — CreateCode retry on hash collision, Claim format gate, removed legacy generator
- `internal/httpserver/activation_http.go` — `activation_code_generation_exhausted` mapping

### Tests
- `internal/app/activation/service_integration_test.go` — format + rejection tests
- `internal/httpserver/activation_admin_http_test.go` — 6-digit create assertion
- `internal/httpserver/activation_claim_http_test.go` — claim accept/reject HTTP tests
- `internal/httpserver/activation_fingerprint_dto_test.go` — example updated
- `internal/grpcserver/machine_activation_grpc_integration_test.go` — format + gRPC rejection test

### Contracts / docs
- `tools/build_openapi.py`, `docs/swagger/swagger.json`, `docs/swagger/docs.go`
- `internal/httpserver/swagger_operations.go`
- `docs/runbooks/machine-activation.md`
- `docs/api/machine-identity-runtime-sessions.md`
- `docs/testing/grpc-local-test.md`

### Production / e2e
- `tools/production_full_test/run_machine_code_activation_prod.py`
- `tools/production_full_test/run_grpc_machine_code_prod.py`
- `tools/production_full_test/bootstrap_test_data.py`
- `tools/production_full_test/run_production_full_suite.py`
- `scripts/e2e/create-production-device-activation-code.sh`
- `tests/e2e/data/reusable-test-data.example.json`

### Reports
- `docs/reports/numeric-activation-code/00_CURRENT_STATE_AUDIT.md`
- `docs/reports/numeric-activation-code/01_IMPLEMENTATION_PLAN.md`

## Behavior

| Operation | New behavior |
|-----------|--------------|
| Create | Cryptographic 6-digit code (`000001`–`999999`), retry up to 20 on hash collision |
| Claim | Reject non-6-digit before DB lookup → `activation_invalid` |
| Hash | HMAC-SHA256 of trim-only code string |
| machineCode | Unchanged (`AVF000001`) |

## Postman

`python tools/build_postman_collection.py` failed locally: missing `postman/scripts/collection_prerequest.js` (Postman assets not present in workspace). OpenAPI generator uses `{{activationCode}}`; CI postman-check will run on PR.
