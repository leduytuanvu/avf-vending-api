# Activation Code API Alignment — Current State Audit

**Date:** 2026-07-05  
**Branch (working tree):** `chore/cleanup-repomix-docs` (uncommitted activation changes)  
**Scope:** Android activation-code runtime alignment contract

## Files inspected

| File | Role |
|------|------|
| `proto/avf/machine/v1/machine_activation.proto` | Proto contract |
| `proto/avf/machine/v1/machine_activation.pb.go` | Generated Go proto |
| `internal/app/activation/service.go` | Claim flow, domain models, attachment integration |
| `internal/app/activation/fingerprint_proto.go` | Proto → domain fingerprint mapper |
| `internal/app/activation/reattach.go` | Technician/admin reattach (must remain unchanged) |
| `internal/grpcserver/machine_grpc_services.go` | gRPC ClaimActivation + auth aliases |
| `internal/httpserver/activation_http.go` | HTTP public claim handler |
| `internal/app/machineruntime/activation_attach.go` | Activation-specific attachment helper |
| `internal/app/machineruntime/session_service.go` | `attachOrReplaceDeviceTx`, BOARD_REPLACED close |
| `internal/app/machineruntime/overview.go` | `DeviceIdentityFromFingerprint` dual-key parsing |
| `db/queries/machine_device_attachments.sql` | Attachment CRUD |
| `db/queries/machine_runtime_app_sessions.sql` | Runtime session close |
| `internal/app/activation/service_integration_test.go` | Existing claim integration tests (no runtime wired) |
| `internal/app/activation/reattach_test.go` | Reattach policy unit tests |
| `internal/app/machineruntime/activation_attach_test.go` | Identity match unit tests |
| `internal/app/activation/fingerprint_proto_test.go` | Proto mapper unit tests |
| `internal/app/activation/claim_result_test.go` | ClaimResult field unit test |

## Item status (pre-implementation audit)

| # | Requirement | Status | Evidence |
|---|-------------|--------|----------|
| 1 | Expanded `DeviceFingerprint` proto fields 8–24 | **PASS** | `machine_activation.proto` lines 22–38 |
| 2 | `ClaimActivationResponse.device_attachment_id = 16` | **PASS** | `machine_activation.proto` line 64 |
| 3 | Go proto regenerated | **PASS** | `GetAndroidSerial`, `GetBoardSerial`, `GetBootId`, `GetSdkInt`, `GetDeviceAttachmentId` in `.pb.go` |
| 4 | `activation.DeviceFingerprint` expanded (24 fields) | **PASS** | `service.go` struct |
| 5 | `ClaimResult.DeviceAttachmentID` | **PASS** | `service.go` `ClaimResult` |
| 6 | gRPC maps expanded fingerprint | **PASS** | `DeviceFingerprintFromProto` + `machine_grpc_services.go` |
| 7 | Activation claim creates/reuses attachment | **PASS** | `deliverActivationClaim` → `EnsureActivationDeviceAttachmentInTx` |
| 8 | gRPC returns `device_attachment_id` | **PASS** | `machine_grpc_services.go` sets `DeviceAttachmentId` |
| 9 | HTTP returns `deviceAttachmentId` | **PASS** | `activation_http.go` response map |
| 10 | HTTP accepts snake_case fingerprint fields | **FAIL** | `publicClaimBody` decodes camelCase-only struct tags; snake_case dropped |
| 11 | DB-backed attachment behavior tests | **FAIL** | `service_integration_test.go` does not wire `SetMachineRuntime`; no create/reuse/replace/session-close assertions |
| 12 | gRPC integration test for attachment + MQTT | **FAIL** | No integration test asserting `device_attachment_id` + MQTT creds on claim |
| 13 | OpenAPI documents `deviceAttachmentId` | **FAIL** | `tools/build_openapi.py` claim 200 example omits field |
| 14 | Activation-code path avoids operator/technician | **PASS** | `EnsureActivationDeviceAttachmentInTx` uses `RequireOperator: false`; claim context optional |
| 15 | Reattach-device behavior unchanged | **PASS** | `reattach.go` untouched; still requires operator for technician reattach |

## Summary

Phases 1–6 are implemented in the working tree. Remaining gaps before `ACTIVATION_CODE_API_ALIGNMENT_PASS`:

1. HTTP dual-style fingerprint JSON normalization (camelCase + snake_case).
2. DB-backed integration tests with `machineruntime` wired.
3. gRPC integration test for `device_attachment_id` and MQTT credentials.
4. OpenAPI regeneration for `deviceAttachmentId` in claim response.
5. Phase 8 reports and CI verification gates.
