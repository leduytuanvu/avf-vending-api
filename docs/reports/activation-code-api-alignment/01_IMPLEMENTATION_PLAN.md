# Activation Code API Alignment — Implementation Plan

**Date:** 2026-07-05

## Objective

Align backend activation-code claim with Android runtime contract: expanded `DeviceFingerprint`, `device_attachment_id` on claim response, device attachment create/reuse/replace during claim, without breaking technician reattach.

## Phases

| Phase | Work | Status |
|-------|------|--------|
| 0 | Audit current state | Done — `00_CURRENT_STATE_AUDIT.md` |
| 1 | Proto fields 8–24 + `device_attachment_id = 16` | Done (additive) |
| 2 | Domain models `DeviceFingerprint`, `ClaimResult.DeviceAttachmentID` | Done |
| 3 | gRPC + HTTP fingerprint mapping | Done; HTTP dual-style DTO added |
| 4 | `EnsureActivationDeviceAttachmentInTx` helper | Done |
| 5 | Integrate helper in `deliverActivationClaim` transaction | Done |
| 6 | Return attachment id in gRPC + HTTP | Done |
| 7 | Tests (unit + DB integration + gRPC integration) | Done |
| 8 | Reports + PR | This document + follow-ups |

## Key design decisions

1. **Idempotent replay:** `DeviceIdentityMatchesAttachment` reuses active attachment; no duplicate rows.
2. **Board replacement:** Different fingerprint calls existing `attachOrReplaceDeviceTx` → marks old attachment `replaced`, closes runtime session `BOARD_REPLACED`.
3. **No operator for activation-code:** `RequireOperator: false` in activation attach path only; `reattach.go` unchanged.
4. **HTTP fingerprint:** Custom `fingerprintDTO.UnmarshalJSON` accepts camelCase and snake_case keys.
5. **Transaction safety:** Attachment work inside same tx as claim; rolls back on MQTT/audit failure before commit.

## Files changed (summary)

- Proto: `proto/avf/machine/v1/machine_activation.{proto,pb.go}`
- Domain: `internal/app/activation/{service.go,fingerprint_proto.go}`
- Runtime: `internal/app/machineruntime/activation_attach.go`
- Transport: `internal/grpcserver/machine_grpc_services.go`, `internal/httpserver/{activation_http.go,activation_fingerprint_dto.go}`
- OpenAPI: `tools/build_openapi.py`, `docs/swagger/swagger.json`
- Tests: new/updated files under `activation`, `httpserver`, `machineruntime`, `grpcserver`
- Docs: `docs/reports/activation-code-api-alignment/*`, `docs/api/android-proto-sync.md`

## Out of scope

- MachineRuntimeSessionService start behavior (unchanged)
- Technician/admin reattach policy
- EMQX provisioning logic (unchanged; only tested when client configured)
