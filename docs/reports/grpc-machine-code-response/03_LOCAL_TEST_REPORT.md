# 03 — Local test report

**Date:** 2026-07-06  
**Environment:** Windows dev, Postgres testcontainers via `machineGRPCTestPool`

## Commands and results

```text
$ go test ./internal/grpcserver -run "MachineCode|ClaimActivation|RefreshMachineToken_Returns|GetBootstrap_Returns|JWTMachineID" -count=1
ok  	github.com/avf/avf-vending-api/internal/grpcserver	2.196s

$ go test ./internal/grpcserver -count=1
ok  	github.com/avf/avf-vending-api/internal/grpcserver	2.284s

$ go test ./internal/app/activation -count=1
ok  	github.com/avf/avf-vending-api/internal/app/activation	1.876s

$ python scripts/ci/generate_android_proto_sync_doc.py
OK: wrote docs/api/android-proto-sync.md (96 RPCs)

$ python scripts/ci/check_machine_grpc_docs.py
OK: machine gRPC docs mention 14 services

$ python scripts/ci/check_machine_grpc_production_contract.py
OK: production contract doc covers 14 services and 38 flow anchors

$ cd proto && go run github.com/bufbuild/buf/cmd/buf@v1.47.0 lint
(exit 0)
```

## New / extended tests

| Test | Asserts |
|------|---------|
| `TestMachineGRPC_ClaimActivation_ReturnsDeviceAttachmentAndMqttCredentials` | `machine_code == AVF000001`, MQTT username == UUID |
| `TestMachineGRPC_ActivateMachineAlias_ReturnsDeviceAttachmentId` | alias claim `machine_code` |
| `TestMachineGRPC_MachineAuthClaimActivationAlias_ReturnsMachineCode` | auth alias |
| `TestMachineGRPC_RefreshMachineToken_ReturnsMachineCode` | token service + auth alias |
| `TestMachineGRPC_GetBootstrap_ReturnsMachineCode` | `BootstrapMachine.machine_code` |
| `TestMachineGRPC_ClaimActivation_JWTMachineIDRemainsUUID` | JWT `machine_id` parseable UUID |

## Notes

- `check_proto_breaking.py` failed locally on Windows (`proto/.git` clone path); CI runs from Linux with repo-root git ref.
- Full `go test ./...` deferred to CI gate on PR.
