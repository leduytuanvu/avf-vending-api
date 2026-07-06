# Numeric Activation Code — Current State Audit

**Date:** 2026-07-06  
**Scope:** Pre-change audit for 6-digit numeric activation code migration.

## Where is activation code generated?

| Location | Function | Pre-change behavior |
|----------|----------|---------------------|
| [`internal/app/activation/service.go`](../../../internal/app/activation/service.go) | `randomActivationCode()` | `fmt.Sprintf("AVF-%06X-%06X", a, c)` — hex segments |
| [`internal/app/activation/service.go`](../../../internal/app/activation/service.go) | `CreateCode()` | Calls `randomActivationCode()`, hashes, inserts row |

No other generators found.

## Where is activation code normalized?

| Location | Pre-change behavior |
|----------|---------------------|
| `service.go` `normalizeActivationCode()` | `strings.ToUpper(strings.TrimSpace(s))` |
| `service.go` `hashActivationCode()` | Hashes normalized string |
| `service.go` `Claim()` | Normalizes input before hash lookup |

## Where is activation code hashed?

| Location | Algorithm |
|----------|-----------|
| `service.go` `hashActivationCode(pepper, code)` | HMAC-SHA256 over normalized code |
| DB column | `machine_activation_codes.code_hash bytea NOT NULL` |
| Unique index | `ux_machine_activation_codes_hash ON (code_hash)` — all rows |

Plaintext is never stored; only returned once on admin create.

## Where is activation code validated on claim?

| Layer | Pre-change validation |
|-------|----------------------|
| `service.go` `Claim()` | Empty-after-normalize only; no format gate |
| `activation_http.go` `postActivationClaim` | Delegates to service; maps `ErrInvalid` → `activation_invalid` |
| `machine_grpc_services.go` | Delegates to service; maps to gRPC invalid argument |

## REST admin endpoints that create activation codes

| Method | Path | Handler |
|--------|------|---------|
| POST | `/v1/admin/machines/{machineId}/activation-codes` | `postAdminCreateActivationCode` |
| POST | `/v1/admin/machine-codes/{machineCode}/activation-codes` | `postAdminCreateActivationCode` |
| POST | `/v1/admin/activation-codes` | `postAdminCatalogCreateActivationCode` |

## REST setup endpoint that claims activation codes

| Method | Path | Handler |
|--------|------|---------|
| POST | `/v1/setup/activation-codes/claim` | `postActivationClaim` (public, rate-limited) |

## gRPC methods that accept activation_code

| Service | RPC | Proto field |
|---------|-----|-------------|
| `MachineActivationService` | `ClaimActivation` | `activation_code` |
| `MachineAuthService` | `ActivateMachine` (alias) | same request type |

Proto: [`proto/avf/machine/v1/machine_activation.proto`](../../../proto/avf/machine/v1/machine_activation.proto)

## Production smoke scripts

| Script | Role |
|--------|------|
| `tools/production_full_test/bootstrap_test_data.py` | Creates + claims activation code during bootstrap |
| `tools/production_full_test/run_machine_code_activation_prod.py` | Machine-code admin + claim smoke |
| `tools/production_full_test/run_grpc_machine_code_prod.py` | gRPC claim with created code |
| `tools/production_full_test/run_grpc_full_production.py` | Uses registry activation code |
| `scripts/e2e/create-production-device-activation-code.sh` | Admin create only |
| `scripts/e2e/claim-production-device-activation-code.sh` | Claim only |

Pre-change: no assertion that activation code is 6-digit numeric.

## Postman / OpenAPI examples (old format)

| File | Example |
|------|---------|
| `tools/build_openapi.py` L5283,5311,5661 | `AVF-123456-ABCDEF`, `AVF-123456` |
| `docs/swagger/swagger.json` | Generated from above |
| `internal/httpserver/activation_fingerprint_dto_test.go` L68 | `AVF-000001-000002` |
| `docs/testing/grpc-local-test.md` | `AVF-XXXXXX-XXXXXX` |

Postman collection is generated via `tools/build_postman_collection.py` using `{{activationCode}}` variable (no hardcoded AVF activation in generator).

## Docs describing old activation code format

| Doc | Notes |
|-----|-------|
| `docs/runbooks/machine-activation.md` | Describes machineCode `AVF000001`; no explicit activation code format |
| `docs/api/machine-activation-implementation-handoff.md` | Design doc; no AVF-hex activation examples |
| `docs/testing/grpc-local-test.md` | Old AVF-hex grpcurl example |

## activationCode vs machineCode confusion?

**No code-path confusion found.**

| Concept | Pattern | File |
|---------|---------|------|
| `machineCode` | `^AVF[0-9]{6}$` e.g. `AVF000001` | `machineref.go` `activationMachineCodePattern` |
| `activationCode` (pre-change) | `AVF-%06X-%06X` | `service.go` `randomActivationCode` |

Note: `run_machine_code_activation_prod.py` misnames `ACTIVATION_CODE_RE` but applies it to **machineCode** only.

## Identity unchanged (confirmed)

- `machineId`: UUID runtime identity — unchanged
- `machineCode`: `AVF` + 6 digits — unchanged
- JWT / MQTT topic identity: machine UUID — unchanged

## Backward compatibility

Old AVF-style codes stored as uppercase-hashed; no plaintext migration possible. Post-deploy: strict 6-digit only; operators re-issue codes.
