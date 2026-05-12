# gRPC Full Coverage

- Generated At: `2026-05-12T08:23:55.203480+00:00`
- Grpc Addr: `127.0.0.1:9090`
- Server Status: `reachable`
- Total Methods: `85`
- Passed: `0`
- Failed: `0`
- Partial: `54`
- Blocked: `31`
- Server reason: grpcurl list succeeded

| Service | RPC | Priority | Class | Status | Reason |
|---|---|---|---|---|---|
| `InternalCatalogQueryService` | `GetSaleCatalogSnapshot` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalCommerceQueryService` | `GetOrderPaymentVendState` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| `InternalInventoryQueryService` | `GetMachineSlotInventory` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalMachineQueryService` | `GetMachineSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalMachineQueryService` | `GetMachineState` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalMachineQueryService` | `GetMachineCabinetSlotSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalTelemetryQueryService` | `GetLatestMachineTelemetry` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalTelemetryQueryService` | `GetMachineIncidentSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalPaymentQueryService` | `GetPaymentById` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| `InternalPaymentQueryService` | `GetLatestPaymentForOrder` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| `InternalReportingQueryService` | `GetSalesSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineAuthService` | `ActivateMachine` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineAuthService` | `ClaimActivation` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineAuthService` | `RefreshMachineToken` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineBootstrapService` | `GetBootstrap` | P0 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineBootstrapService` | `CheckIn` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineBootstrapService` | `AckConfigVersion` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineBootstrapService` | `CheckForUpdates` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineCatalogService` | `GetSaleCatalog` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineCatalogService` | `SyncSaleCatalog` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineCatalogService` | `GetCatalogSnapshot` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineCatalogService` | `GetCatalogDelta` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineCatalogService` | `AckCatalogVersion` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineCatalogService` | `GetMediaManifest` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineCommandService` | `GetPendingCommands` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineCommandService` | `AckCommand` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineCommandService` | `RejectCommand` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineCommandService` | `GetAssignedUpdate` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineCommandService` | `ReportUpdateStatus` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineCommandService` | `ReportDiagnosticBundleResult` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineCommerceService` | `CreateOrder` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineCommerceService` | `CreatePaymentSession` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| `MachineCommerceService` | `AttachPaymentResult` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| `MachineCommerceService` | `ConfirmCashPayment` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| `MachineCommerceService` | `CreateCashCheckout` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineCommerceService` | `GetOrder` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineCommerceService` | `GetOrderStatus` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineCommerceService` | `StartVend` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineCommerceService` | `ConfirmVendSuccess` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineCommerceService` | `ReportVendSuccess` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineCommerceService` | `ReportVendFailure` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineCommerceService` | `CancelOrder` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineSaleService` | `CreateSale` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineSaleService` | `AttachPayment` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| `MachineSaleService` | `ConfirmCashReceived` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineSaleService` | `StartVend` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineSaleService` | `CompleteVend` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineSaleService` | `FailVend` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| `MachineSaleService` | `CancelSale` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineInventoryService` | `PushInventoryDelta` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineInventoryService` | `GetInventorySnapshot` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineInventoryService` | `AckInventorySync` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineInventoryService` | `GetPlanogram` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineInventoryService` | `SubmitStockSnapshot` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineInventoryService` | `SubmitFillResult` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineInventoryService` | `SubmitFillReport` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineInventoryService` | `SubmitRestock` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineInventoryService` | `SubmitInventoryAdjustment` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineInventoryService` | `SubmitStockAdjustment` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineActivationService` | `ClaimActivation` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineTokenService` | `RefreshMachineToken` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineMediaService` | `GetMediaManifest` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineMediaService` | `GetMediaDelta` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineMediaService` | `AckMediaVersion` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineOfflineSyncService` | `PushOfflineEvents` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineOfflineSyncService` | `GetSyncCursor` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| `MachineOperatorService` | `OpenOperatorSession` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineOperatorService` | `CloseOperatorSession` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineOperatorService` | `SubmitFillReport` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineOperatorService` | `SubmitStockAdjustment` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineOperatorService` | `LoginOperator` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineOperatorService` | `LogoutOperator` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineOperatorService` | `HeartbeatOperatorSession` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineTelemetryService` | `PushTelemetryBatch` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineTelemetryService` | `PushCriticalEvent` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineTelemetryService` | `CheckIn` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineTelemetryService` | `SubmitTelemetryBatch` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineTelemetryService` | `ReconcileEvents` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `MachineTelemetryService` | `GetEventStatus` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalMachineQueryService` | `GetMachineSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalMachineQueryService` | `GetMachineState` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalMachineQueryService` | `GetMachineCabinetSlotSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalTelemetryQueryService` | `GetLatestMachineTelemetry` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalTelemetryQueryService` | `GetMachineIncidentSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| `InternalCommerceQueryService` | `GetOrderPaymentVendState` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
