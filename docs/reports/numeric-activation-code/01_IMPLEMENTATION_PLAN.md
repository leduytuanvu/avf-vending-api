# Numeric Activation Code — Implementation Plan

**Date:** 2026-07-06

## Exact files to edit

### Core
- `internal/app/activation/code_format.go` (new)
- `internal/app/activation/code_format_test.go` (new)
- `internal/app/activation/service.go`
- `internal/httpserver/activation_http.go`

### Tests
- `internal/app/activation/service_integration_test.go`
- `internal/httpserver/activation_admin_http_test.go`
- `internal/httpserver/activation_claim_http_test.go` (new)
- `internal/httpserver/activation_fingerprint_dto_test.go`
- `internal/grpcserver/machine_activation_grpc_integration_test.go`

### Contracts / docs
- `tools/build_openapi.py`
- `internal/httpserver/swagger_operations.go`
- `docs/swagger/swagger.json` + `docs/swagger/docs.go` (regenerated)
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

## Before / after behavior

| Operation | Before | After |
|-----------|--------|-------|
| Create | Returns `AVF-12ABCD-34EF56` | Returns `342209` (6 digits, leading zeros allowed) |
| Claim valid | Accepts uppercase-normalized AVF-hex | Accepts exactly 6 digits (trim outer space only) |
| Claim invalid format | DB lookup → `activation_invalid` | Format gate → `activation_invalid` (no DB) |
| Hash input | Uppercase trimmed | Trim-only 6-digit string |
| Collision on insert | Fail | Retry up to 20, then `activation_code_generation_exhausted` |

## Backward compatibility

**Strict 6-digit only.** No temporary AVF acceptance. Unclaimed old codes obsolete.

## Test plan

1. Unit: `code_format_test.go` — all valid/invalid cases
2. Service integration: create format, claim accept/reject
3. HTTP integration: admin create format, claim accept/reject
4. gRPC integration: claim with 6-digit, reject AVF-style
5. Production smoke: format assert + negative claims

## Local verification commands

```powershell
go test ./internal/app/activation -count=1
go test ./internal/httpserver -count=1
go test ./internal/grpcserver -count=1
go test ./... -count=1
python tools/build_openapi.py
python tools/build_postman_collection.py
```

## Production deployment plan

1. PR → `develop`, CI green
2. PR `develop` → `main`, sync branches
3. Build and Push Images + Deploy Production workflow
4. Verify `/health/live`, `/health/ready`, `/version` SHA
5. Run `run_production_full_suite.py --passes 3`
6. Run `run_machine_code_activation_prod.py` + `run_grpc_machine_code_prod.py`

## Rollback plan

Redeploy previous SHA. Old AVF codes already unusable after forward deploy; operators create new 6-digit codes.

## Production verification checklist

- [ ] Admin create returns `activationCode` matching `^[0-9]{6}$`
- [ ] Claim with created code succeeds; machineId UUID, machineCode AVF...
- [ ] Claim AVF-style → `activation_invalid`
- [ ] Claim 5-digit / 7-digit → `activation_invalid`
- [ ] Full REST/gRPC/MQTT suite pass count = total
- [ ] Runner exit code 0

## Cleanup plan (post-GO)

Separate branch `chore/project-cleanup-after-numeric-activation`; no production deploy.
