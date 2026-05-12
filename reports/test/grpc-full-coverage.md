# gRPC Full Coverage

- gRPC addr: `127.0.0.1:9090`
- Server status: **blocked-tooling**
- Reason: scripts/test/run-grpc-full-coverage.sh was not executable on this host or grpcurl/server was unavailable
- Total methods: **85**

| Service | RPC | Priority | Class | Status | Reason |
|---|---|---|---|---|---|
| `InternalCatalogQueryService` | `GetSaleCatalogSnapshot` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalCommerceQueryService` | `GetOrderPaymentVendState` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `InternalInventoryQueryService` | `GetMachineSlotInventory` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalMachineQueryService` | `GetMachineSummary` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalMachineQueryService` | `GetMachineState` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalMachineQueryService` | `GetMachineCabinetSlotSummary` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalTelemetryQueryService` | `GetLatestMachineTelemetry` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalTelemetryQueryService` | `GetMachineIncidentSummary` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalPaymentQueryService` | `GetPaymentById` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalPaymentQueryService` | `GetLatestPaymentForOrder` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalReportingQueryService` | `GetSalesSummary` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineAuthService` | `ActivateMachine` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineAuthService` | `ClaimActivation` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineAuthService` | `RefreshMachineToken` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineBootstrapService` | `GetBootstrap` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineBootstrapService` | `CheckIn` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineBootstrapService` | `AckConfigVersion` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineBootstrapService` | `CheckForUpdates` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineCatalogService` | `GetSaleCatalog` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineCatalogService` | `SyncSaleCatalog` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineCatalogService` | `GetCatalogSnapshot` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineCatalogService` | `GetCatalogDelta` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineCatalogService` | `AckCatalogVersion` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineCatalogService` | `GetMediaManifest` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineCommandService` | `GetPendingCommands` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineCommandService` | `AckCommand` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineCommandService` | `RejectCommand` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineCommandService` | `GetAssignedUpdate` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineCommandService` | `ReportUpdateStatus` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineCommandService` | `ReportDiagnosticBundleResult` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `CreateOrder` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `CreatePaymentSession` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `AttachPaymentResult` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `ConfirmCashPayment` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `CreateCashCheckout` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `GetOrder` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `GetOrderStatus` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `StartVend` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `ConfirmVendSuccess` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `ReportVendSuccess` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `ReportVendFailure` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineCommerceService` | `CancelOrder` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineSaleService` | `CreateSale` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineSaleService` | `AttachPayment` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineSaleService` | `ConfirmCashReceived` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineSaleService` | `StartVend` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineSaleService` | `CompleteVend` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineSaleService` | `FailVend` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
| `MachineSaleService` | `CancelSale` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `PushInventoryDelta` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `GetInventorySnapshot` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `AckInventorySync` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `GetPlanogram` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `SubmitStockSnapshot` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `SubmitFillResult` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `SubmitFillReport` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `SubmitRestock` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `SubmitInventoryAdjustment` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineInventoryService` | `SubmitStockAdjustment` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineActivationService` | `ClaimActivation` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineTokenService` | `RefreshMachineToken` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineMediaService` | `GetMediaManifest` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineMediaService` | `GetMediaDelta` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineMediaService` | `AckMediaVersion` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineOfflineSyncService` | `PushOfflineEvents` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineOfflineSyncService` | `GetSyncCursor` | P0 | write | **partial** | Full grpcurl runner not executed yet |
| `MachineOperatorService` | `OpenOperatorSession` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineOperatorService` | `CloseOperatorSession` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineOperatorService` | `SubmitFillReport` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineOperatorService` | `SubmitStockAdjustment` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineOperatorService` | `LoginOperator` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineOperatorService` | `LogoutOperator` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineOperatorService` | `HeartbeatOperatorSession` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineTelemetryService` | `PushTelemetryBatch` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineTelemetryService` | `PushCriticalEvent` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineTelemetryService` | `CheckIn` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineTelemetryService` | `SubmitTelemetryBatch` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineTelemetryService` | `ReconcileEvents` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `MachineTelemetryService` | `GetEventStatus` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalMachineQueryService` | `GetMachineSummary` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalMachineQueryService` | `GetMachineState` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalMachineQueryService` | `GetMachineCabinetSlotSummary` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalTelemetryQueryService` | `GetLatestMachineTelemetry` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalTelemetryQueryService` | `GetMachineIncidentSummary` | P1 | read-only | **partial** | Full grpcurl runner not executed yet |
| `InternalCommerceQueryService` | `GetOrderPaymentVendState` | P0 | hardware-required | **partial** | Full grpcurl runner not executed yet |
