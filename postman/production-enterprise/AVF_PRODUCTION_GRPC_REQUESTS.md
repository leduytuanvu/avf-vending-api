# AVF Production gRPC requests

Target: `{{grpcTarget}}` (TLS, ALPN h2). Machine JWT in metadata `authorization: Bearer <redacted>`.

Postman Desktop: New → gRPC → server URL → import proto from `proto/avf/machine/v1/` → invoke.

Catalog generated from `proto/avf/machine/v1` + `RegisterMachineGRPCServices` (machine edge only).
E2E-verified flows reference `tests/e2e/production/e2e-manifest-grpc.yaml`. Newman does not run gRPC.

| Service | RPC | E2E flow | Verdict | Notes |
|---------|-----|----------|---------|-------|
| MachineActivationService | ClaimActivation | — | COVERED |  |
| MachineAuthService | ActivateMachine | — | COVERED | Alias of ClaimActivation |
| MachineAuthService | ClaimActivation | — | COVERED |  |
| MachineAuthService | RefreshMachineToken | — | COVERED |  |
| MachineBootstrapService | AckConfigVersion | — | COVERED |  |
| MachineBootstrapService | CheckForUpdates | — | COVERED |  |
| MachineBootstrapService | CheckIn | GRPC-BOOT-002 | COVERED |  |
| MachineBootstrapService | GetBootstrap | GRPC-BOOT-001 | COVERED |  |
| MachineCatalogService | AckCatalogVersion | GRPC-CAT-003 | COVERED |  |
| MachineCatalogService | GetCatalogDelta | GRPC-CAT-002 | COVERED |  |
| MachineCatalogService | GetCatalogSnapshot | — | COVERED |  |
| MachineCatalogService | GetMediaManifest | — | COVERED |  |
| MachineCatalogService | GetSaleCatalog | GRPC-CAT-001-inner | COVERED | Alias of GetCatalogSnapshot |
| MachineCatalogService | SyncCatalogBundle | — | COVERED |  |
| MachineCatalogService | SyncSaleCatalog | — | COVERED | Alias of GetCatalogSnapshot |
| MachineCommandService | AckCommand | — | COVERED |  |
| MachineCommandService | GetAssignedUpdate | — | COVERED |  |
| MachineCommandService | GetPendingCommands | — | COVERED |  |
| MachineCommandService | RejectCommand | — | COVERED |  |
| MachineCommandService | ReportDiagnosticBundleResult | — | COVERED |  |
| MachineCommandService | ReportUpdateStatus | — | COVERED |  |
| MachineCommerceService | AttachPaymentResult | — | COVERED | Alias of CreatePaymentSession |
| MachineCommerceService | CancelOrder | — | COVERED |  |
| MachineCommerceService | ConfirmCashPayment | — | COVERED |  |
| MachineCommerceService | ConfirmVendSuccess | — | COVERED | Alias of ReportVendSuccess |
| MachineCommerceService | CreateCashCheckout | — | COVERED | Alias of ConfirmCashPayment |
| MachineCommerceService | CreateOrder | — | COVERED |  |
| MachineCommerceService | CreatePaymentSession | — | COVERED |  |
| MachineCommerceService | GetOrder | — | COVERED |  |
| MachineCommerceService | GetOrderStatus | — | COVERED |  |
| MachineCommerceService | ReportVendFailure | — | COVERED |  |
| MachineCommerceService | ReportVendSuccess | — | COVERED |  |
| MachineCommerceService | StartVend | — | COVERED |  |
| MachineInventoryService | AckInventorySync | GRPC-INV-002 | COVERED |  |
| MachineInventoryService | GetInventorySnapshot | GRPC-INV-001 | COVERED |  |
| MachineInventoryService | GetPlanogram | — | COVERED |  |
| MachineInventoryService | PushInventoryDelta | — | COVERED |  |
| MachineInventoryService | SubmitFillReport | — | COVERED | Alias of SubmitFillResult |
| MachineInventoryService | SubmitFillResult | — | COVERED |  |
| MachineInventoryService | SubmitInventoryAdjustment | — | COVERED |  |
| MachineInventoryService | SubmitRestock | — | COVERED |  |
| MachineInventoryService | SubmitStockAdjustment | — | COVERED | Alias of SubmitInventoryAdjustment |
| MachineInventoryService | SubmitStockSnapshot | — | COVERED |  |
| MachineMediaService | AckMediaVersion | GRPC-MED-003-inner | COVERED |  |
| MachineMediaService | GetMediaDelta | GRPC-MED-002-inner | COVERED |  |
| MachineMediaService | GetMediaManifest | GRPC-MED-001-inner | COVERED |  |
| MachineOfflineSyncService | GetSyncCursor | — | COVERED |  |
| MachineOfflineSyncService | PushOfflineEvents | — | COVERED |  |
| MachineOperatorService | CloseOperatorSession | — | COVERED |  |
| MachineOperatorService | HeartbeatOperatorSession | — | COVERED |  |
| MachineOperatorService | LoginOperator | — | COVERED |  |
| MachineOperatorService | LogoutOperator | — | COVERED |  |
| MachineOperatorService | OpenOperatorSession | — | COVERED |  |
| MachineOperatorService | SubmitFillReport | — | COVERED | Alias of SubmitFillResult |
| MachineOperatorService | SubmitStockAdjustment | — | COVERED | Alias of SubmitInventoryAdjustment |
| MachineTelemetryService | CheckIn | — | COVERED |  |
| MachineTelemetryService | GetEventStatus | — | COVERED |  |
| MachineTelemetryService | PushCriticalEvent | — | COVERED |  |
| MachineTelemetryService | PushTelemetryBatch | — | COVERED |  |
| MachineTelemetryService | ReconcileEvents | — | COVERED |  |
| MachineTelemetryService | SubmitTelemetryBatch | — | COVERED |  |
| MachineTokenService | RefreshMachineToken | GRPC-TOKEN-001 | COVERED |  |

## E2E flow reference (grpcurl)

### GRPC-TOKEN-001 — MachineTokenService/RefreshMachineToken

- Service: `MachineTokenService` RPC: `RefreshMachineToken`
- Metadata: (none)

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"refreshToken": "{{machineRefreshToken}}"}' {{grpcTarget}} MachineTokenService.RefreshMachineToken
```

### GRPC-BOOT-001 — MachineBootstrapService/GetBootstrap

- Service: `MachineBootstrapService` RPC: `GetBootstrap`
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineBootstrapService.GetBootstrap
```

### GRPC-BOOT-002 — MachineBootstrapService/CheckIn

- Service: `MachineBootstrapService` RPC: `CheckIn`
- Metadata: authorization: Bearer <machineAccessToken>; idempotency-key

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"bootId": "{{run_prefix}}-boot", "networkState": "online", "attributes": {"source": "e2e-prod-grpc"}}' {{grpcTarget}} MachineBootstrapService.CheckIn
```

### GRPC-CAT-001 — Catalog sync + published item assertions

- Service: `` RPC: ``
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} None.None
```

### GRPC-CAT-002 — MachineCatalogService/GetCatalogDelta

- Service: `MachineCatalogService` RPC: `GetCatalogDelta`
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"machineId": "{{machineId}}", "basisCatalogVersion": "{{catalogVersion}}"}' {{grpcTarget}} MachineCatalogService.GetCatalogDelta
```

### GRPC-CAT-003 — MachineCatalogService/AckCatalogVersion

- Service: `MachineCatalogService` RPC: `AckCatalogVersion`
- Metadata: authorization: Bearer <machineAccessToken>; idempotency-key

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"acknowledgedCatalogVersion": "{{catalogVersion}}"}' {{grpcTarget}} MachineCatalogService.AckCatalogVersion
```

### GRPC-MED-001 — Media manifest + download cache

- Service: `` RPC: ``
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} None.None
```

### GRPC-INV-001 — MachineInventoryService/GetInventorySnapshot (baseline)

- Service: `MachineInventoryService` RPC: `GetInventorySnapshot`
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService.GetInventorySnapshot
```

### GRPC-INV-002 — MachineInventoryService/AckInventorySync

- Service: `MachineInventoryService` RPC: `AckInventorySync`
- Metadata: authorization: Bearer <machineAccessToken>; idempotency-key

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"syncCursor": "{{run_prefix}}-inv-cursor"}' {{grpcTarget}} MachineInventoryService.AckInventorySync
```

### GRPC-COMM-CASH-001 — Commerce cash path (CreateOrder → ConfirmCash → Vend)

- Service: `` RPC: ``
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} None.None
```

### GRPC-COMM-QR-001 — Commerce QR path + REST webhook

- Service: `` RPC: ``
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} None.None
```

### GRPC-COMM-FAIL-001 — Vend failure path

- Service: `` RPC: ``
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} None.None
```

### GRPC-COMM-CANCEL-001 — Cancel order idempotency

- Service: `` RPC: ``
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} None.None
```

### GRPC-IDEM-001 — CreateOrder idempotency replay

- Service: `` RPC: ``
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} None.None
```

### GRPC-OFFLINE-001 — Offline event replay

- Service: `` RPC: ``
- Metadata: authorization: Bearer <machineAccessToken>

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} None.None
```


## Reference RPC (all production-registered)

### MachineActivationService/ClaimActivation

- Proto: `machine_activation.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineActivationService/ClaimActivation
```

### MachineAuthService/ActivateMachine

- Proto: `auth.proto` — verdict: **COVERED**
- Note: Alias of ClaimActivation

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineAuthService/ActivateMachine
```

### MachineAuthService/ClaimActivation

- Proto: `auth.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineAuthService/ClaimActivation
```

### MachineAuthService/RefreshMachineToken

- Proto: `auth.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineAuthService/RefreshMachineToken
```

### MachineBootstrapService/AckConfigVersion

- Proto: `bootstrap.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineBootstrapService/AckConfigVersion
```

### MachineBootstrapService/CheckForUpdates

- Proto: `bootstrap.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineBootstrapService/CheckForUpdates
```

### MachineCatalogService/GetCatalogSnapshot

- Proto: `catalog.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCatalogService/GetCatalogSnapshot
```

### MachineCatalogService/GetMediaManifest

- Proto: `catalog.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCatalogService/GetMediaManifest
```

### MachineCatalogService/SyncCatalogBundle

- Proto: `catalog.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCatalogService/SyncCatalogBundle
```

### MachineCatalogService/SyncSaleCatalog

- Proto: `catalog.proto` — verdict: **COVERED**
- Note: Alias of GetCatalogSnapshot

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCatalogService/SyncSaleCatalog
```

### MachineCommandService/AckCommand

- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/AckCommand
```

### MachineCommandService/GetAssignedUpdate

- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/GetAssignedUpdate
```

### MachineCommandService/GetPendingCommands

- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/GetPendingCommands
```

### MachineCommandService/RejectCommand

- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/RejectCommand
```

### MachineCommandService/ReportDiagnosticBundleResult

- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/ReportDiagnosticBundleResult
```

### MachineCommandService/ReportUpdateStatus

- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/ReportUpdateStatus
```

### MachineCommerceService/AttachPaymentResult

- Proto: `commerce.proto` — verdict: **COVERED**
- Note: Alias of CreatePaymentSession

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/AttachPaymentResult
```

### MachineCommerceService/CancelOrder

- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/CancelOrder
```

### MachineCommerceService/ConfirmCashPayment

- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/ConfirmCashPayment
```

### MachineCommerceService/ConfirmVendSuccess

- Proto: `commerce.proto` — verdict: **COVERED**
- Note: Alias of ReportVendSuccess

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/ConfirmVendSuccess
```

### MachineCommerceService/CreateCashCheckout

- Proto: `commerce.proto` — verdict: **COVERED**
- Note: Alias of ConfirmCashPayment

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/CreateCashCheckout
```

### MachineCommerceService/CreateOrder

- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/CreateOrder
```

### MachineCommerceService/CreatePaymentSession

- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/CreatePaymentSession
```

### MachineCommerceService/GetOrder

- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/GetOrder
```

### MachineCommerceService/GetOrderStatus

- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/GetOrderStatus
```

### MachineCommerceService/ReportVendFailure

- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/ReportVendFailure
```

### MachineCommerceService/ReportVendSuccess

- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/ReportVendSuccess
```

### MachineCommerceService/StartVend

- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/StartVend
```

### MachineInventoryService/GetPlanogram

- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/GetPlanogram
```

### MachineInventoryService/PushInventoryDelta

- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/PushInventoryDelta
```

### MachineInventoryService/SubmitFillReport

- Proto: `inventory.proto` — verdict: **COVERED**
- Note: Alias of SubmitFillResult

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitFillReport
```

### MachineInventoryService/SubmitFillResult

- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitFillResult
```

### MachineInventoryService/SubmitInventoryAdjustment

- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitInventoryAdjustment
```

### MachineInventoryService/SubmitRestock

- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitRestock
```

### MachineInventoryService/SubmitStockAdjustment

- Proto: `inventory.proto` — verdict: **COVERED**
- Note: Alias of SubmitInventoryAdjustment

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitStockAdjustment
```

### MachineInventoryService/SubmitStockSnapshot

- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitStockSnapshot
```

### MachineOfflineSyncService/GetSyncCursor

- Proto: `offline_sync.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOfflineSyncService/GetSyncCursor
```

### MachineOfflineSyncService/PushOfflineEvents

- Proto: `offline_sync.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOfflineSyncService/PushOfflineEvents
```

### MachineOperatorService/CloseOperatorSession

- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/CloseOperatorSession
```

### MachineOperatorService/HeartbeatOperatorSession

- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/HeartbeatOperatorSession
```

### MachineOperatorService/LoginOperator

- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/LoginOperator
```

### MachineOperatorService/LogoutOperator

- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/LogoutOperator
```

### MachineOperatorService/OpenOperatorSession

- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/OpenOperatorSession
```

### MachineOperatorService/SubmitFillReport

- Proto: `operator_grpc.proto` — verdict: **COVERED**
- Note: Alias of SubmitFillResult

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/SubmitFillReport
```

### MachineOperatorService/SubmitStockAdjustment

- Proto: `operator_grpc.proto` — verdict: **COVERED**
- Note: Alias of SubmitInventoryAdjustment

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/SubmitStockAdjustment
```

### MachineTelemetryService/CheckIn

- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/CheckIn
```

### MachineTelemetryService/GetEventStatus

- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/GetEventStatus
```

### MachineTelemetryService/PushCriticalEvent

- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/PushCriticalEvent
```

### MachineTelemetryService/PushTelemetryBatch

- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/PushTelemetryBatch
```

### MachineTelemetryService/ReconcileEvents

- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/ReconcileEvents
```

### MachineTelemetryService/SubmitTelemetryBatch

- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/SubmitTelemetryBatch
```
