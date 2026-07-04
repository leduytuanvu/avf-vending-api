# Activation Code API Alignment — Implementation Report

**Date:** 2026-07-05

## Summary

Completed backend alignment for activation-code claim: expanded fingerprint contract, `device_attachment_id` in responses, and safe device attachment lifecycle during public claim.

## Proto (Phase 1)

- Added `DeviceFingerprint` fields 8–24 (additive, existing 1–7 unchanged).
- Added `ClaimActivationResponse.device_attachment_id = 16` (after `mqtt_password = 15`).
- Regenerated `machine_activation.pb.go`.

## Domain (Phase 2)

- `activation.DeviceFingerprint` expanded to 24 fields with camelCase JSON tags.
- `activation.ClaimResult.DeviceAttachmentID *uuid.UUID` added.
- `DeviceFingerprintFromProto` maps all proto fields.

## Transport (Phase 3–6)

### gRPC

- `ClaimActivation` uses `DeviceFingerprintFromProto`.
- Sets `resp.DeviceAttachmentId` when attachment created/reused.
- `ActivateMachine` and `MachineAuthService.ClaimActivation` aliases wrap same response (attachment id included).

### HTTP

- Response includes `deviceAttachmentId` when present.
- New `fingerprintDTO` with dual-style JSON parsing (camelCase + snake_case).

## Attachment (Phase 4–5)

- `machineruntime.EnsureActivationDeviceAttachmentInTx`:
  - No active attachment → create via `attachOrReplaceDeviceTx`.
  - Matching fingerprint → reuse active attachment (no session close).
  - Different fingerprint → replace + `BOARD_REPLACED` runtime session close.
- Wired in `deliverActivationClaim` before transaction commit; rolls back with claim on failure.

## OpenAPI

- Claim 200 example updated with `deviceAttachmentId`, `mqttUsername`, `mqttPassword`, refresh fields.

## Guardrails verified

| Rule | Result |
|------|--------|
| Public pre-auth claim | Unchanged |
| No operator_session_id required | Activation attach uses `RequireOperator: false` |
| No technician assignment | Not checked on activation path |
| Reattach unchanged | `reattach.go` not modified |
| Aliases preserved | `ActivateMachine`, `ClaimActivation` intact |
| No secret logging | No new log lines for tokens/passwords |
