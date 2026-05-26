# AVF Production gRPC requests

Target: `{{grpcTarget}}` (TLS, ALPN h2). Machine JWT in metadata `authorization: Bearer <redacted>`.

Postman Desktop: New → gRPC → server URL → import proto from `proto/avf/machine/v1/` → invoke.

Catalog generated from `proto/avf/machine/v1` + `RegisterMachineGRPCServices` (machine edge only).
E2E-verified flows reference `tests/e2e/production/e2e-manifest-grpc.yaml`. Newman does not run gRPC.

| Service | RPC | Actor | E2E flow | Verdict | Notes |
|---------|-----|-------|----------|---------|-------|
| MachineActivationService | ClaimActivation | MACHINE_APP | — | COVERED |  |
| MachineAuthService | ActivateMachine | MACHINE_APP | — | COVERED | Alias of ClaimActivation |
| MachineAuthService | ClaimActivation | MACHINE_APP | — | COVERED |  |
| MachineAuthService | RefreshMachineToken | MACHINE_APP | — | COVERED |  |
| MachineBootstrapService | AckConfigVersion | MACHINE_APP | — | COVERED |  |
| MachineBootstrapService | CheckForUpdates | MACHINE_APP | — | COVERED |  |
| MachineBootstrapService | CheckIn | MACHINE_APP | GRPC-BOOT-002 | COVERED |  |
| MachineBootstrapService | GetBootstrap | MACHINE_APP | GRPC-BOOT-001 | COVERED |  |
| MachineCatalogService | AckCatalogVersion | MACHINE_APP | GRPC-CAT-003 | COVERED |  |
| MachineCatalogService | GetCatalogDelta | MACHINE_APP | GRPC-CAT-002 | COVERED |  |
| MachineCatalogService | GetCatalogSnapshot | MACHINE_APP | — | COVERED |  |
| MachineCatalogService | GetMediaManifest | MACHINE_APP | — | COVERED |  |
| MachineCatalogService | GetSaleCatalog | MACHINE_APP | GRPC-CAT-001-inner | COVERED | Alias of GetCatalogSnapshot |
| MachineCatalogService | SyncCatalogBundle | MACHINE_APP | — | COVERED |  |
| MachineCatalogService | SyncSaleCatalog | MACHINE_APP | — | COVERED | Alias of GetCatalogSnapshot |
| MachineCommandService | AckCommand | MACHINE_APP | — | COVERED |  |
| MachineCommandService | GetAssignedUpdate | MACHINE_APP | — | COVERED |  |
| MachineCommandService | GetPendingCommands | MACHINE_APP | — | COVERED |  |
| MachineCommandService | RejectCommand | MACHINE_APP | — | COVERED |  |
| MachineCommandService | ReportDiagnosticBundleResult | MACHINE_APP | — | COVERED |  |
| MachineCommandService | ReportUpdateStatus | MACHINE_APP | — | COVERED |  |
| MachineCommerceService | AttachPaymentResult | PAYMENT | — | COVERED | Alias of CreatePaymentSession |
| MachineCommerceService | CancelOrder | MACHINE_APP | — | COVERED |  |
| MachineCommerceService | ConfirmCashPayment | MACHINE_APP | — | COVERED |  |
| MachineCommerceService | ConfirmVendSuccess | MACHINE_APP | — | COVERED | Alias of ReportVendSuccess |
| MachineCommerceService | CreateCashCheckout | MACHINE_APP | — | COVERED | Alias of ConfirmCashPayment |
| MachineCommerceService | CreateOrder | MACHINE_APP | — | COVERED |  |
| MachineCommerceService | CreatePaymentSession | PAYMENT | — | COVERED |  |
| MachineCommerceService | GetOrder | MACHINE_APP | — | COVERED |  |
| MachineCommerceService | GetOrderStatus | MACHINE_APP | — | COVERED |  |
| MachineCommerceService | ReportVendFailure | MACHINE_APP | — | COVERED |  |
| MachineCommerceService | ReportVendSuccess | MACHINE_APP | — | COVERED |  |
| MachineCommerceService | StartVend | MACHINE_APP | — | COVERED |  |
| MachineInventoryService | AckInventorySync | MACHINE_APP | GRPC-INV-002 | COVERED |  |
| MachineInventoryService | GetInventorySnapshot | MACHINE_APP | GRPC-INV-001 | COVERED |  |
| MachineInventoryService | GetPlanogram | MACHINE_APP | — | COVERED |  |
| MachineInventoryService | PushInventoryDelta | MACHINE_APP | — | COVERED |  |
| MachineInventoryService | SubmitFillReport | MACHINE_APP | — | COVERED | Alias of SubmitFillResult |
| MachineInventoryService | SubmitFillResult | MACHINE_APP | — | COVERED |  |
| MachineInventoryService | SubmitInventoryAdjustment | MACHINE_APP | — | COVERED |  |
| MachineInventoryService | SubmitRestock | MACHINE_APP | — | COVERED |  |
| MachineInventoryService | SubmitStockAdjustment | MACHINE_APP | — | COVERED | Alias of SubmitInventoryAdjustment |
| MachineInventoryService | SubmitStockSnapshot | MACHINE_APP | — | COVERED |  |
| MachineMediaService | AckMediaVersion | MACHINE_APP | GRPC-MED-003-inner | COVERED |  |
| MachineMediaService | GetMediaDelta | MACHINE_APP | GRPC-MED-002-inner | COVERED |  |
| MachineMediaService | GetMediaManifest | MACHINE_APP | GRPC-MED-001-inner | COVERED |  |
| MachineOfflineSyncService | GetSyncCursor | MACHINE_APP | — | COVERED |  |
| MachineOfflineSyncService | PushOfflineEvents | MACHINE_APP | — | COVERED |  |
| MachineOperatorService | CloseOperatorSession | TECHNICIAN | — | COVERED |  |
| MachineOperatorService | HeartbeatOperatorSession | TECHNICIAN | — | COVERED |  |
| MachineOperatorService | LoginOperator | TECHNICIAN | — | COVERED |  |
| MachineOperatorService | LogoutOperator | TECHNICIAN | — | COVERED |  |
| MachineOperatorService | OpenOperatorSession | TECHNICIAN | — | COVERED |  |
| MachineOperatorService | SubmitFillReport | TECHNICIAN | — | COVERED | Alias of SubmitFillResult |
| MachineOperatorService | SubmitStockAdjustment | TECHNICIAN | — | COVERED | Alias of SubmitInventoryAdjustment |
| MachineTelemetryService | CheckIn | MACHINE_APP | — | COVERED |  |
| MachineTelemetryService | GetEventStatus | MACHINE_APP | — | COVERED |  |
| MachineTelemetryService | PushCriticalEvent | MACHINE_APP | — | COVERED |  |
| MachineTelemetryService | PushTelemetryBatch | MACHINE_APP | — | COVERED |  |
| MachineTelemetryService | ReconcileEvents | MACHINE_APP | — | COVERED |  |
| MachineTelemetryService | SubmitTelemetryBatch | MACHINE_APP | — | COVERED |  |
| MachineTokenService | RefreshMachineToken | MACHINE_APP | GRPC-TOKEN-001 | COVERED |  |

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

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineActivationService/ClaimActivation
- Proto: `machine_activation.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineActivationService/ClaimActivation
```

### MachineAuthService/ActivateMachine

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineAuthService/ActivateMachine
- Proto: `auth.proto` — verdict: **COVERED**
- Note: Alias of ClaimActivation

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineAuthService/ActivateMachine
```

### MachineAuthService/ClaimActivation

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineAuthService/ClaimActivation
- Proto: `auth.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineAuthService/ClaimActivation
```

### MachineAuthService/RefreshMachineToken

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineAuthService/RefreshMachineToken
- Proto: `auth.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineAuthService/RefreshMachineToken
```

### MachineBootstrapService/AckConfigVersion

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineBootstrapService/AckConfigVersion
- Proto: `bootstrap.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineBootstrapService/AckConfigVersion
```

### MachineBootstrapService/CheckForUpdates

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineBootstrapService/CheckForUpdates
- Proto: `bootstrap.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineBootstrapService/CheckForUpdates
```

### MachineCatalogService/GetCatalogSnapshot

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCatalogService/GetCatalogSnapshot
- Proto: `catalog.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCatalogService/GetCatalogSnapshot
```

### MachineCatalogService/GetMediaManifest

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCatalogService/GetMediaManifest
- Proto: `catalog.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCatalogService/GetMediaManifest
```

### MachineCatalogService/SyncCatalogBundle

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCatalogService/SyncCatalogBundle
- Proto: `catalog.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCatalogService/SyncCatalogBundle
```

### MachineCatalogService/SyncSaleCatalog

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCatalogService/SyncSaleCatalog
- Proto: `catalog.proto` — verdict: **COVERED**
- Note: Alias of GetCatalogSnapshot

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCatalogService/SyncSaleCatalog
```

### MachineCommandService/AckCommand

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommandService/AckCommand
- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/AckCommand
```

### MachineCommandService/GetAssignedUpdate

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommandService/GetAssignedUpdate
- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/GetAssignedUpdate
```

### MachineCommandService/GetPendingCommands

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommandService/GetPendingCommands
- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/GetPendingCommands
```

### MachineCommandService/RejectCommand

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommandService/RejectCommand
- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/RejectCommand
```

### MachineCommandService/ReportDiagnosticBundleResult

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommandService/ReportDiagnosticBundleResult
- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/ReportDiagnosticBundleResult
```

### MachineCommandService/ReportUpdateStatus

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommandService/ReportUpdateStatus
- Proto: `command.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommandService/ReportUpdateStatus
```

### MachineCommerceService/AttachPaymentResult

- **Used by:** Payment Provider / Webhook Provider
- **Primary actor:** payment provider
- **Purpose:** Online payment session (phase 2)
- Proto: `commerce.proto` — verdict: **COVERED**
- Note: Alias of CreatePaymentSession

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/AttachPaymentResult
```

### MachineCommerceService/CancelOrder

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/CancelOrder
- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/CancelOrder
```

### MachineCommerceService/ConfirmCashPayment

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/ConfirmCashPayment
- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/ConfirmCashPayment
```

### MachineCommerceService/ConfirmVendSuccess

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/ConfirmVendSuccess
- Proto: `commerce.proto` — verdict: **COVERED**
- Note: Alias of ReportVendSuccess

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/ConfirmVendSuccess
```

### MachineCommerceService/CreateCashCheckout

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/CreateCashCheckout
- Proto: `commerce.proto` — verdict: **COVERED**
- Note: Alias of ConfirmCashPayment

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/CreateCashCheckout
```

### MachineCommerceService/CreateOrder

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/CreateOrder
- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/CreateOrder
```

### MachineCommerceService/CreatePaymentSession

- **Used by:** Payment Provider / Webhook Provider
- **Primary actor:** payment provider
- **Purpose:** Online payment session (phase 2)
- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/CreatePaymentSession
```

### MachineCommerceService/GetOrder

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/GetOrder
- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/GetOrder
```

### MachineCommerceService/GetOrderStatus

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/GetOrderStatus
- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/GetOrderStatus
```

### MachineCommerceService/ReportVendFailure

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/ReportVendFailure
- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/ReportVendFailure
```

### MachineCommerceService/ReportVendSuccess

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/ReportVendSuccess
- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/ReportVendSuccess
```

### MachineCommerceService/StartVend

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineCommerceService/StartVend
- Proto: `commerce.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineCommerceService/StartVend
```

### MachineInventoryService/GetPlanogram

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineInventoryService/GetPlanogram
- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/GetPlanogram
```

### MachineInventoryService/PushInventoryDelta

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineInventoryService/PushInventoryDelta
- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/PushInventoryDelta
```

### MachineInventoryService/SubmitFillReport

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineInventoryService/SubmitFillReport
- Proto: `inventory.proto` — verdict: **COVERED**
- Note: Alias of SubmitFillResult

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitFillReport
```

### MachineInventoryService/SubmitFillResult

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineInventoryService/SubmitFillResult
- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitFillResult
```

### MachineInventoryService/SubmitInventoryAdjustment

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineInventoryService/SubmitInventoryAdjustment
- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitInventoryAdjustment
```

### MachineInventoryService/SubmitRestock

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineInventoryService/SubmitRestock
- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitRestock
```

### MachineInventoryService/SubmitStockAdjustment

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineInventoryService/SubmitStockAdjustment
- Proto: `inventory.proto` — verdict: **COVERED**
- Note: Alias of SubmitInventoryAdjustment

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitStockAdjustment
```

### MachineInventoryService/SubmitStockSnapshot

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineInventoryService/SubmitStockSnapshot
- Proto: `inventory.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineInventoryService/SubmitStockSnapshot
```

### MachineOfflineSyncService/GetSyncCursor

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineOfflineSyncService/GetSyncCursor
- Proto: `offline_sync.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOfflineSyncService/GetSyncCursor
```

### MachineOfflineSyncService/PushOfflineEvents

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineOfflineSyncService/PushOfflineEvents
- Proto: `offline_sync.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOfflineSyncService/PushOfflineEvents
```

### MachineOperatorService/CloseOperatorSession

- **Used by:** Technician App / Operator
- **Primary actor:** technician
- **Purpose:** Operator workflow via MachineOperatorService/CloseOperatorSession
- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/CloseOperatorSession
```

### MachineOperatorService/HeartbeatOperatorSession

- **Used by:** Technician App / Operator
- **Primary actor:** technician
- **Purpose:** Operator workflow via MachineOperatorService/HeartbeatOperatorSession
- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/HeartbeatOperatorSession
```

### MachineOperatorService/LoginOperator

- **Used by:** Technician App / Operator
- **Primary actor:** technician
- **Purpose:** Operator workflow via MachineOperatorService/LoginOperator
- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/LoginOperator
```

### MachineOperatorService/LogoutOperator

- **Used by:** Technician App / Operator
- **Primary actor:** technician
- **Purpose:** Operator workflow via MachineOperatorService/LogoutOperator
- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/LogoutOperator
```

### MachineOperatorService/OpenOperatorSession

- **Used by:** Technician App / Operator
- **Primary actor:** technician
- **Purpose:** Operator workflow via MachineOperatorService/OpenOperatorSession
- Proto: `operator_grpc.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/OpenOperatorSession
```

### MachineOperatorService/SubmitFillReport

- **Used by:** Technician App / Operator
- **Primary actor:** technician
- **Purpose:** Operator workflow via MachineOperatorService/SubmitFillReport
- Proto: `operator_grpc.proto` — verdict: **COVERED**
- Note: Alias of SubmitFillResult

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/SubmitFillReport
```

### MachineOperatorService/SubmitStockAdjustment

- **Used by:** Technician App / Operator
- **Primary actor:** technician
- **Purpose:** Operator workflow via MachineOperatorService/SubmitStockAdjustment
- Proto: `operator_grpc.proto` — verdict: **COVERED**
- Note: Alias of SubmitInventoryAdjustment

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineOperatorService/SubmitStockAdjustment
```

### MachineTelemetryService/CheckIn

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineTelemetryService/CheckIn
- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/CheckIn
```

### MachineTelemetryService/GetEventStatus

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineTelemetryService/GetEventStatus
- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/GetEventStatus
```

### MachineTelemetryService/PushCriticalEvent

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineTelemetryService/PushCriticalEvent
- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/PushCriticalEvent
```

### MachineTelemetryService/PushTelemetryBatch

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineTelemetryService/PushTelemetryBatch
- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/PushTelemetryBatch
```

### MachineTelemetryService/ReconcileEvents

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineTelemetryService/ReconcileEvents
- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/ReconcileEvents
```

### MachineTelemetryService/SubmitTelemetryBatch

- **Used by:** Vending Machine App
- **Primary actor:** machine app
- **Purpose:** Machine runtime: MachineTelemetryService/SubmitTelemetryBatch
- Proto: `telemetry.proto` — verdict: **COVERED**

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {{grpcTarget}} MachineTelemetryService/SubmitTelemetryBatch
```
