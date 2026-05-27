# Canonical gRPC surface

Package and method names are stable for vending apps. **No proto removal** in this phase.

## Machine app (`avf.machine.v1`) — canonical

| Service | Responsibility |
|---------|----------------|
| `MachineActivationService` | Activation codes, machine identity |
| `MachineTokenService` | Token refresh |
| `MachineAuthService` | Facade over activation/token (compatibility entry) |
| `MachineBootstrapService` | First boot / check-in |
| `MachineCatalogService` | Sale catalog sync |
| `MachineMediaService` | Media manifest / download metadata |
| `MachineInventoryService` | Inventory sync |
| `MachineCommerceService` | Checkout / vend (primary commerce path) |
| `MachineSaleService` | Narrow sale helpers (legacy companion; prefer Commerce when both apply) |
| `MachineTelemetryService` | Telemetry / check-in |
| `MachineCommandService` | Command ack / status |
| `MachineOfflineSyncService` | Offline replay |
| `MachineOperatorService` | Operator session helpers |

Registration: `internal/grpcserver/machine_grpc_services.go`.

## Internal read models (`avf.internal.v1`) — canonical

Loopback-only query services: machine, telemetry, commerce, payment, catalog, inventory, reporting.

**Not registered:** duplicate `avf.v1` protos under `proto/avf/v1/` (compatibility stubs only).

## Legacy / compatibility

| Item | Notes |
|------|-------|
| `MachineAuthService` facade | Duplicates activation/token RPCs for older clients |
| `avf.v1.Internal*Query` protos | Generated but not mounted |
| Duplicate manifest RPCs | Prefer documented method in Postman matrix |

## Postman / E2E

gRPC templates mark canonical services in the enterprise matrix; legacy duplicates grouped under **Legacy and Compatibility** in guides.
