# 02 — Implementation report

## Files changed

| File | Change |
|------|--------|
| `proto/avf/machine/v1/machine_activation.proto` | `machine_code = 17` on `ClaimActivationResponse` |
| `proto/avf/machine/v1/machine_token.proto` | `machine_code = 13` on `RefreshMachineTokenResponse` |
| `proto/avf/machine/v1/bootstrap.proto` | `machine_code = 11` on `BootstrapMachine` |
| `proto/avf/machine/v1/*.pb.go` | Regenerated |
| `internal/grpcserver/machine_grpc_services.go` | Map `MachineCode` in claim, refresh, bootstrap |
| `internal/grpcserver/machine_activation_grpc_integration_test.go` | INSERT `code`, assert `machine_code` |
| `internal/grpcserver/machine_code_grpc_integration_test.go` | Refresh, bootstrap, auth alias, JWT UUID tests |
| `docs/api/machine-grpc.md` | Display-only identity note |
| `docs/api/machine-grpc-production-contract.md` | Response fields + display identity policy |
| `docs/api/machine-identity-runtime-sessions.md` | UUID vs machine_code contract |
| `docs/api/machine-activation-implementation-handoff.md` | Android handoff note |
| `docs/api/android-proto-sync.md` | Regenerated |
| `tools/production_full_test/run_grpc_machine_code_prod.py` | Production smoke |

## Not changed

- Auth interceptors, JWT claim layout, MQTT topic builders
- DB migrations
- REST activation handlers (already returned `machineCode`)
