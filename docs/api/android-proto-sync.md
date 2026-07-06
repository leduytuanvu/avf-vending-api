# Android proto sync index (generated)

**Generated:** `2026-07-06T00:13:43Z` by `scripts/ci/generate_android_proto_sync_doc.py`

Copy `proto/avf/machine/v1/*.proto` into the Android app (Buf, Gradle protobuf, or manual sync).
Canonical runtime contract: [`machine-grpc-production-contract.md`](machine-grpc-production-contract.md).

| Service | RPC | Request | Response | Status |
|---------|-----|---------|----------|--------|
| `MachineActivationService` | `ClaimActivation` | `ClaimActivationRequest` | `ClaimActivationResponse` | active |
| `MachineAuthService` | `ActivateMachine` | `ActivateMachineRequest` | `ActivateMachineResponse` | active |
| `MachineAuthService` | `ClaimActivation` | `MachineAuthServiceClaimActivationRequest` | `MachineAuthServiceClaimActivationResponse` | active |
| `MachineAuthService` | `RefreshMachineToken` | `MachineAuthServiceRefreshMachineTokenRequest` | `MachineAuthServiceRefreshMachineTokenResponse` | active |
| `MachineBootstrapService` | `AckConfigVersion` | `AckConfigVersionRequest` | `AckConfigVersionResponse` | active |
| `MachineBootstrapService` | `CheckForUpdates` | `CheckForUpdatesRequest` | `CheckForUpdatesResponse` | active |
| `MachineBootstrapService` | `CheckIn` | `MachineBootstrapServiceCheckInRequest` | `MachineBootstrapServiceCheckInResponse` | active |
| `MachineBootstrapService` | `GetBootstrap` | `GetBootstrapRequest` | `GetBootstrapResponse` | active |
| `MachineCatalogService` | `AckCatalogVersion` | `AckCatalogVersionRequest` | `AckCatalogVersionResponse` | active |
| `MachineCatalogService` | `GetCatalogDelta` | `GetCatalogDeltaRequest` | `GetCatalogDeltaResponse` | active |
| `MachineCatalogService` | `GetCatalogSnapshot` | `GetCatalogSnapshotRequest` | `GetCatalogSnapshotResponse` | active |
| `MachineCatalogService` | `GetMediaManifest` | `GetMediaManifestRequest` | `GetMediaManifestResponse` | active |
| `MachineCatalogService` | `GetSaleCatalog` | `GetCatalogSnapshotRequest` | `GetCatalogSnapshotResponse` | active |
| `MachineCatalogService` | `SyncCatalogBundle` | `SyncCatalogBundleRequest` | `SyncCatalogBundleResponse` | active |
| `MachineCatalogService` | `SyncSaleCatalog` | `GetCatalogSnapshotRequest` | `GetCatalogSnapshotResponse` | active |
| `MachineCommandService` | `AckCommand` | `AckCommandRequest` | `AckCommandResponse` | deprecated |
| `MachineCommandService` | `GetAssignedUpdate` | `GetAssignedUpdateRequest` | `GetAssignedUpdateResponse` | active |
| `MachineCommandService` | `GetPendingCommands` | `GetPendingCommandsRequest` | `GetPendingCommandsResponse` | deprecated |
| `MachineCommandService` | `RejectCommand` | `RejectCommandRequest` | `RejectCommandResponse` | deprecated |
| `MachineCommandService` | `ReportDiagnosticBundleResult` | `ReportDiagnosticBundleResultRequest` | `ReportDiagnosticBundleResultResponse` | active |
| `MachineCommandService` | `ReportUpdateStatus` | `ReportUpdateStatusRequest` | `ReportUpdateStatusResponse` | active |
| `MachineCommerceService` | `AttachPayment` | `AttachPaymentRequest` | `AttachPaymentResponse` | active |
| `MachineCommerceService` | `AttachPaymentResult` | `CreatePaymentSessionRequest` | `CreatePaymentSessionResponse` | active |
| `MachineCommerceService` | `CancelOrder` | `CancelOrderRequest` | `CancelOrderResponse` | active |
| `MachineCommerceService` | `CancelSale` | `CancelOrderRequest` | `CancelOrderResponse` | active |
| `MachineCommerceService` | `CompleteVend` | `ConfirmVendSuccessRequest` | `ConfirmVendSuccessResponse` | active |
| `MachineCommerceService` | `ConfirmCashPayment` | `ConfirmCashPaymentRequest` | `ConfirmCashPaymentResponse` | active |
| `MachineCommerceService` | `ConfirmCashReceived` | `ConfirmCashReceivedRequest` | `ConfirmCashReceivedResponse` | active |
| `MachineCommerceService` | `ConfirmVendSuccess` | `ConfirmVendSuccessRequest` | `ConfirmVendSuccessResponse` | active |
| `MachineCommerceService` | `CreateCashCheckout` | `ConfirmCashPaymentRequest` | `ConfirmCashPaymentResponse` | active |
| `MachineCommerceService` | `CreateOrder` | `CreateOrderRequest` | `CreateOrderResponse` | active |
| `MachineCommerceService` | `CreateOrderFromQuote` | `CreateOrderFromQuoteRequest` | `CreateOrderFromQuoteResponse` | active |
| `MachineCommerceService` | `CreatePaymentSession` | `CreatePaymentSessionRequest` | `CreatePaymentSessionResponse` | active |
| `MachineCommerceService` | `CreateQuote` | `CreateQuoteRequest` | `CreateQuoteResponse` | active |
| `MachineCommerceService` | `CreateSale` | `CreateSaleRequest` | `CreateSaleResponse` | active |
| `MachineCommerceService` | `FailVend` | `ReportVendFailureRequest` | `ReportVendFailureResponse` | active |
| `MachineCommerceService` | `GetOrder` | `GetOrderRequest` | `GetOrderResponse` | active |
| `MachineCommerceService` | `GetOrderStatus` | `GetOrderStatusRequest` | `GetOrderStatusResponse` | active |
| `MachineCommerceService` | `ReportVendFailure` | `ReportVendFailureRequest` | `ReportVendFailureResponse` | active |
| `MachineCommerceService` | `ReportVendSuccess` | `ReportVendSuccessRequest` | `ReportVendSuccessResponse` | active |
| `MachineCommerceService` | `StartVend` | `StartVendRequest` | `StartVendResponse` | active |
| `MachineCommerceService` | `StartVend` | `StartVendRequest` | `StartVendResponse` | active |
| `MachineInventoryService` | `AckInventorySync` | `AckInventorySyncRequest` | `AckInventorySyncResponse` | active |
| `MachineInventoryService` | `GetInventorySnapshot` | `GetInventorySnapshotRequest` | `GetInventorySnapshotResponse` | active |
| `MachineInventoryService` | `GetPlanogram` | `GetPlanogramRequest` | `GetPlanogramResponse` | active |
| `MachineInventoryService` | `PushInventoryDelta` | `ReportInventoryDeltaRequest` | `ReportInventoryDeltaResponse` | active |
| `MachineInventoryService` | `SubmitFillReport` | `SubmitFillResultRequest` | `SubmitFillResultResponse` | active |
| `MachineInventoryService` | `SubmitFillResult` | `SubmitFillResultRequest` | `SubmitFillResultResponse` | active |
| `MachineInventoryService` | `SubmitInventoryAdjustment` | `SubmitInventoryAdjustmentRequest` | `SubmitInventoryAdjustmentResponse` | active |
| `MachineInventoryService` | `SubmitRestock` | `SubmitRestockRequest` | `SubmitRestockResponse` | active |
| `MachineInventoryService` | `SubmitStockAdjustment` | `SubmitInventoryAdjustmentRequest` | `SubmitInventoryAdjustmentResponse` | active |
| `MachineInventoryService` | `SubmitStockSnapshot` | `SubmitStockSnapshotRequest` | `SubmitStockSnapshotResponse` | active |
| `MachineMediaService` | `AckMediaVersion` | `AckMediaVersionRequest` | `AckMediaVersionResponse` | active |
| `MachineMediaService` | `GetMediaDelta` | `GetMediaDeltaRequest` | `GetMediaDeltaResponse` | active |
| `MachineMediaService` | `GetMediaManifest` | `MachineMediaServiceGetMediaManifestRequest` | `MachineMediaServiceGetMediaManifestResponse` | active |
| `MachineOfflineSyncService` | `GetSyncCursor` | `GetSyncCursorRequest` | `GetSyncCursorResponse` | active |
| `MachineOfflineSyncService` | `PushOfflineEvents` | `SyncOfflineEventsRequest` | `SyncOfflineEventsResponse` | active |
| `MachineOperatorService` | `CloseOperatorSession` | `CloseOperatorSessionRequest` | `CloseOperatorSessionResponse` | active |
| `MachineOperatorService` | `HeartbeatOperatorSession` | `HeartbeatOperatorSessionRequest` | `HeartbeatOperatorSessionResponse` | active |
| `MachineOperatorService` | `LoginOperator` | `LoginOperatorRequest` | `LoginOperatorResponse` | active |
| `MachineOperatorService` | `LogoutOperator` | `LogoutOperatorRequest` | `LogoutOperatorResponse` | active |
| `MachineOperatorService` | `OpenOperatorSession` | `OpenOperatorSessionRequest` | `OpenOperatorSessionResponse` | active |
| `MachineOperatorService` | `SubmitFillReport` | `SubmitFillReportRequest` | `SubmitFillReportResponse` | active |
| `MachineOperatorService` | `SubmitStockAdjustment` | `SubmitStockAdjustmentRequest` | `SubmitStockAdjustmentResponse` | active |
| `MachineRuntimeSessionService` | `EndRuntimeSession` | `EndRuntimeSessionRequest` | `EndRuntimeSessionResponse` | active |
| `MachineRuntimeSessionService` | `GetRuntimeSessionState` | `GetRuntimeSessionStateRequest` | `GetRuntimeSessionStateResponse` | active |
| `MachineRuntimeSessionService` | `HeartbeatRuntimeSession` | `HeartbeatRuntimeSessionRequest` | `HeartbeatRuntimeSessionResponse` | active |
| `MachineRuntimeSessionService` | `StartRuntimeSession` | `StartRuntimeSessionRequest` | `StartRuntimeSessionResponse` | active |
| `MachineSaleService` | `AttachPayment` | `AttachPaymentRequest` | `AttachPaymentResponse` | active |
| `MachineSaleService` | `AttachPaymentResult` | `CreatePaymentSessionRequest` | `CreatePaymentSessionResponse` | active |
| `MachineSaleService` | `CancelOrder` | `CancelOrderRequest` | `CancelOrderResponse` | active |
| `MachineSaleService` | `CancelSale` | `CancelOrderRequest` | `CancelOrderResponse` | active |
| `MachineSaleService` | `CompleteVend` | `ConfirmVendSuccessRequest` | `ConfirmVendSuccessResponse` | active |
| `MachineSaleService` | `ConfirmCashPayment` | `ConfirmCashPaymentRequest` | `ConfirmCashPaymentResponse` | active |
| `MachineSaleService` | `ConfirmCashReceived` | `ConfirmCashReceivedRequest` | `ConfirmCashReceivedResponse` | active |
| `MachineSaleService` | `ConfirmVendSuccess` | `ConfirmVendSuccessRequest` | `ConfirmVendSuccessResponse` | active |
| `MachineSaleService` | `CreateCashCheckout` | `ConfirmCashPaymentRequest` | `ConfirmCashPaymentResponse` | active |
| `MachineSaleService` | `CreateOrder` | `CreateOrderRequest` | `CreateOrderResponse` | active |
| `MachineSaleService` | `CreateOrderFromQuote` | `CreateOrderFromQuoteRequest` | `CreateOrderFromQuoteResponse` | active |
| `MachineSaleService` | `CreatePaymentSession` | `CreatePaymentSessionRequest` | `CreatePaymentSessionResponse` | active |
| `MachineSaleService` | `CreateQuote` | `CreateQuoteRequest` | `CreateQuoteResponse` | active |
| `MachineSaleService` | `CreateSale` | `CreateSaleRequest` | `CreateSaleResponse` | active |
| `MachineSaleService` | `FailVend` | `ReportVendFailureRequest` | `ReportVendFailureResponse` | active |
| `MachineSaleService` | `GetOrder` | `GetOrderRequest` | `GetOrderResponse` | active |
| `MachineSaleService` | `GetOrderStatus` | `GetOrderStatusRequest` | `GetOrderStatusResponse` | active |
| `MachineSaleService` | `ReportVendFailure` | `ReportVendFailureRequest` | `ReportVendFailureResponse` | active |
| `MachineSaleService` | `ReportVendSuccess` | `ReportVendSuccessRequest` | `ReportVendSuccessResponse` | active |
| `MachineSaleService` | `StartVend` | `StartVendRequest` | `StartVendResponse` | active |
| `MachineSaleService` | `StartVend` | `StartVendRequest` | `StartVendResponse` | active |
| `MachineTelemetryService` | `CheckIn` | `CheckInRequest` | `CheckInResponse` | active |
| `MachineTelemetryService` | `GetEventStatus` | `GetEventStatusRequest` | `GetEventStatusResponse` | active |
| `MachineTelemetryService` | `PushCriticalEvent` | `PushCriticalEventRequest` | `PushCriticalEventResponse` | active |
| `MachineTelemetryService` | `PushTelemetryBatch` | `PushTelemetryBatchRequest` | `PushTelemetryBatchResponse` | active |
| `MachineTelemetryService` | `ReconcileEvents` | `ReconcileEventsRequest` | `ReconcileEventsResponse` | active |
| `MachineTelemetryService` | `SubmitTelemetryBatch` | `SubmitTelemetryBatchRequest` | `SubmitTelemetryBatchResponse` | active |
| `MachineTokenService` | `RefreshMachineToken` | `RefreshMachineTokenRequest` | `RefreshMachineTokenResponse` | active |

**Total RPCs:** 96
