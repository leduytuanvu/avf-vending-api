# Activation Code API Alignment — Test Report

**Date:** 2026-07-05

## Commands run

| Command | Result |
|---------|--------|
| `go build ./...` | PASS |
| `go test ./... -short` | PASS |
| `python tools/build_openapi.py` | PASS (swagger regenerated) |
| `python scripts/ci/generate_android_proto_sync_doc.py` | PASS |
| DB integration (`TEST_DATABASE_URL` set, local Postgres down) | FAIL connect (environment); tests skip when unset |

## New / updated tests

### Proto / contract (unit)

| Test | Package | Purpose |
|------|---------|---------|
| `TestProtoContract_DeviceFingerprintFields8Through24` | `activation` | Getters for fields 8–24 |
| `TestProtoContract_ClaimActivationResponseDeviceAttachmentId` | `activation` | Field 16 + MQTT fields |
| `TestDeviceFingerprintFromProto_allFields` | `activation` | Proto → domain mapping |

### HTTP (unit)

| Test | Package | Purpose |
|------|---------|---------|
| `TestFingerprintDTO_UnmarshalJSON_camelCase` | `httpserver` | camelCase fingerprint |
| `TestFingerprintDTO_UnmarshalJSON_snakeCase` | `httpserver` | snake_case fingerprint |
| `TestPublicClaimBody_acceptsSnakeCaseFingerprint` | `httpserver` | Full claim body parsing |

### Attachment identity (unit)

| Test | Package | Purpose |
|------|---------|---------|
| `TestDeviceIdentityMatchesAttachment_*` | `machineruntime` | Match / mismatch / incomplete replay |
| `TestClaimResult_includesDeviceAttachmentAndMqttFields` | `activation` | ClaimResult shape |

### Service integration (DB — `TEST_DATABASE_URL`)

| Test | Purpose |
|------|---------|
| `TestClaim_WithRuntime_FirstClaimCreatesDeviceAttachment` | Creates attachment + updates `current_device_attachment_id` |
| `TestClaim_WithRuntime_ReplayReusesAttachment` | Same fingerprint → same id, one active row |
| `TestClaim_WithRuntime_DifferentBoardReplacesAttachmentAndClosesSession` | Replace + `BOARD_REPLACED` |
| `TestClaim_WithRuntime_NoOperatorSessionRequired` | Claim without operator context |

### gRPC integration (DB — `TEST_DATABASE_URL`)

| Test | Purpose |
|------|---------|
| `TestMachineGRPC_ClaimActivation_ReturnsDeviceAttachmentAndMqttCredentials` | gRPC attachment id + MQTT creds (mock EMQX) |
| `TestMachineGRPC_ActivateMachineAlias_ReturnsDeviceAttachmentId` | Alias returns attachment id |

## Notes

- Integration tests follow existing repo pattern: skip in `-short` or when `TEST_DATABASE_URL` unset; CI provides Postgres.
- Reattach policy tests remain in `reattach_test.go` (unchanged).
