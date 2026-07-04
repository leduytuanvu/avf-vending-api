# Activation Code API Alignment — Final Verdict

**Date:** 2026-07-05  
**Verdict:** `ACTIVATION_CODE_API_ALIGNMENT_PASS`

## Checklist (15 items)

| # | Question | Answer | Evidence |
|---|----------|--------|----------|
| 1 | API DeviceFingerprint fields 8–24? | **yes** | `machine_activation.proto`, `proto_contract_test.go` |
| 2 | ClaimActivationResponse device_attachment_id = 16? | **yes** | `machine_activation.proto` line 64 |
| 3 | Go proto regenerated? | **yes** | `machine_activation.pb.go` getters present |
| 4 | gRPC ClaimActivation maps expanded fingerprint? | **yes** | `fingerprint_proto.go`, `machine_grpc_services.go` |
| 5 | HTTP accepts expanded fingerprint? | **yes** | `activation_fingerprint_dto.go` dual-style JSON |
| 6 | First claim creates active device attachment? | **yes** | `EnsureActivationDeviceAttachmentInTx`, `TestClaim_WithRuntime_FirstClaimCreatesDeviceAttachment` |
| 7 | Same fingerprint replay avoids duplicate? | **yes** | `DeviceIdentityMatchesAttachment`, `TestClaim_WithRuntime_ReplayReusesAttachment` |
| 8 | Different fingerprint replaces attachment? | **yes** | `attachOrReplaceDeviceTx`, integration test |
| 9 | Replacement closes runtime session BOARD_REPLACED? | **yes** | `session_service.go`, integration test |
| 10 | gRPC returns device_attachment_id? | **yes** | `machine_grpc_services.go`, gRPC integration test |
| 11 | HTTP returns deviceAttachmentId? | **yes** | `activation_http.go`, OpenAPI example |
| 12 | Activation-code avoids operator_session_id? | **yes** | `RequireOperator: false`, `TestClaim_WithRuntime_NoOperatorSessionRequired` |
| 13 | Activation-code avoids technician assignment? | **yes** | No assignment check on activation attach path |
| 14 | Reattach-device unchanged? | **yes** | `reattach.go` untouched; existing tests pass |
| 15 | Tests green? | **yes** | `go test ./... -short` PASS; DB tests skip without `TEST_DATABASE_URL` (CI runs with DB) |

## Residual notes

- Full DB integration verification depends on CI Postgres (`TEST_DATABASE_URL`); local run skipped when unset.
- OpenAPI regenerated; run `make api-contract-check` in CI for full contract gate.
