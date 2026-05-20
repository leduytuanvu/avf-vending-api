# AVF gRPC Request/Response Catalog

## /avf.internal.v1.InternalCatalogQueryService/GetSaleCatalogSnapshot

- **Proto:** `proto/avf/internal/v1/catalog_query.proto`
- **Request message:** `GetSaleCatalogSnapshotRequest`
- **Response message:** `GetSaleCatalogSnapshotResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}",
  "include_unavailable": false,
  "include_images": false,
  "if_none_match_config_version": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalCommerceQueryService/GetOrderPaymentVendState

- **Proto:** `proto/avf/internal/v1/commerce_query.proto`
- **Request message:** `GetOrderPaymentVendStateRequest`
- **Response message:** `GetOrderPaymentVendStateResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "order_id": "{{$guid}}",
  "slot_index": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalInventoryQueryService/GetMachineSlotInventory

- **Proto:** `proto/avf/internal/v1/inventory_query.proto`
- **Request message:** `GetMachineSlotInventoryRequest`
- **Response message:** `GetMachineSlotInventoryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalMachineQueryService/GetMachineCabinetSlotSummary

- **Proto:** `proto/avf/internal/v1/machine_query.proto`
- **Request message:** `GetMachineCabinetSlotSummaryRequest`
- **Response message:** `GetMachineCabinetSlotSummaryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalMachineQueryService/GetMachineState

- **Proto:** `proto/avf/internal/v1/machine_query.proto`
- **Request message:** `GetMachineStateRequest`
- **Response message:** `GetMachineStateResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalMachineQueryService/GetMachineSummary

- **Proto:** `proto/avf/internal/v1/machine_query.proto`
- **Request message:** `GetMachineSummaryRequest`
- **Response message:** `GetMachineSummaryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalPaymentQueryService/GetLatestPaymentForOrder

- **Proto:** `proto/avf/internal/v1/payment_query.proto`
- **Request message:** `GetLatestPaymentForOrderRequest`
- **Response message:** `GetLatestPaymentForOrderResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "order_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalPaymentQueryService/GetPaymentById

- **Proto:** `proto/avf/internal/v1/payment_query.proto`
- **Request message:** `GetPaymentByIdRequest`
- **Response message:** `GetPaymentByIdResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "payment_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalReportingQueryService/GetSalesSummary

- **Proto:** `proto/avf/internal/v1/reporting_query.proto`
- **Request message:** `GetSalesSummaryRequest`
- **Response message:** `GetSalesSummaryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "from_rfc3339": "",
  "to_rfc3339": "",
  "group_by": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalTelemetryQueryService/GetLatestMachineTelemetry

- **Proto:** `proto/avf/internal/v1/machine_query.proto`
- **Request message:** `GetLatestMachineTelemetryRequest`
- **Response message:** `GetLatestMachineTelemetryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.internal.v1.InternalTelemetryQueryService/GetMachineIncidentSummary

- **Proto:** `proto/avf/internal/v1/machine_query.proto`
- **Request message:** `GetMachineIncidentSummaryRequest`
- **Response message:** `GetMachineIncidentSummaryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}",
  "limit": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineActivationService/ClaimActivation

- **Proto:** `proto/avf/machine/v1/machine_activation.proto`
- **Request message:** `ClaimActivationRequest`
- **Response message:** `ClaimActivationResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "activation_code": "",
  "device_fingerprint": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineAuthService/ActivateMachine

- **Proto:** `proto/avf/machine/v1/auth.proto`
- **Request message:** `ActivateMachineRequest`
- **Response message:** `ActivateMachineResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "claim": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineAuthService/ClaimActivation

- **Proto:** `proto/avf/machine/v1/auth.proto`
- **Request message:** `MachineAuthServiceClaimActivationRequest`
- **Response message:** `MachineAuthServiceClaimActivationResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "claim": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineAuthService/RefreshMachineToken

- **Proto:** `proto/avf/machine/v1/auth.proto`
- **Request message:** `MachineAuthServiceRefreshMachineTokenRequest`
- **Response message:** `MachineAuthServiceRefreshMachineTokenResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "refresh": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineBootstrapService/AckConfigVersion

- **Proto:** `proto/avf/machine/v1/bootstrap.proto`
- **Request message:** `AckConfigVersionRequest`
- **Response message:** `AckConfigVersionResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "acknowledged_config_version": 0,
  "acknowledged_planogram_version_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineBootstrapService/CheckForUpdates

- **Proto:** `proto/avf/machine/v1/bootstrap.proto`
- **Request message:** `CheckForUpdatesRequest`
- **Response message:** `CheckForUpdatesResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "catalog_fingerprint": "",
  "pricing_fingerprint": "",
  "planogram_fingerprint": "",
  "media_fingerprint": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineBootstrapService/CheckIn

- **Proto:** `proto/avf/machine/v1/bootstrap.proto`
- **Request message:** `MachineBootstrapServiceCheckInRequest`
- **Response message:** `MachineBootstrapServiceCheckInResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "boot_id": "{{$guid}}",
  "network_state": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineBootstrapService/GetBootstrap

- **Proto:** `proto/avf/machine/v1/bootstrap.proto`
- **Request message:** `GetBootstrapRequest`
- **Response message:** `GetBootstrapResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCatalogService/AckCatalogVersion

- **Proto:** `proto/avf/machine/v1/catalog.proto`
- **Request message:** `AckCatalogVersionRequest`
- **Response message:** `AckCatalogVersionResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "acknowledged_catalog_version": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCatalogService/GetCatalogDelta

- **Proto:** `proto/avf/machine/v1/catalog.proto`
- **Request message:** `GetCatalogDeltaRequest`
- **Response message:** `GetCatalogDeltaResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}",
  "basis_catalog_version": "",
  "meta": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCatalogService/GetCatalogSnapshot

- **Proto:** `proto/avf/machine/v1/catalog.proto`
- **Request message:** `GetCatalogSnapshotRequest`
- **Response message:** `GetCatalogSnapshotResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "machine_id": "{{machineId}}",
  "include_unavailable": false,
  "include_images": false,
  "if_none_match_config_version": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCatalogService/GetMediaManifest

- **Proto:** `proto/avf/machine/v1/catalog.proto`
- **Request message:** `GetMediaManifestRequest`
- **Response message:** `GetMediaManifestResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}",
  "include_unavailable": false,
  "meta": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCatalogService/GetSaleCatalog

- **Proto:** `proto/avf/machine/v1/catalog.proto`
- **Request message:** `GetCatalogSnapshotRequest`
- **Response message:** `GetCatalogSnapshotResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "machine_id": "{{machineId}}",
  "include_unavailable": false,
  "include_images": false,
  "if_none_match_config_version": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCatalogService/SyncCatalogBundle

- **Proto:** `proto/avf/machine/v1/catalog.proto`
- **Request message:** `SyncCatalogBundleRequest`
- **Response message:** `SyncCatalogBundleResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "machine_id": "{{machineId}}",
  "include_unavailable": false,
  "include_images": false,
  "current_catalog_version": "",
  "current_media_manifest_version": "",
  "basis_product_ids": [],
  "basis_media_asset_keys": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCatalogService/SyncSaleCatalog

- **Proto:** `proto/avf/machine/v1/catalog.proto`
- **Request message:** `GetCatalogSnapshotRequest`
- **Response message:** `GetCatalogSnapshotResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "machine_id": "{{machineId}}",
  "include_unavailable": false,
  "include_images": false,
  "if_none_match_config_version": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommandService/AckCommand

- **Proto:** `proto/avf/machine/v1/command.proto`
- **Request message:** `AckCommandRequest`
- **Response message:** `AckCommandResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "command_id": "{{$guid}}",
  "command_sequence": 0,
  "receipt_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommandService/GetAssignedUpdate

- **Proto:** `proto/avf/machine/v1/command.proto`
- **Request message:** `GetAssignedUpdateRequest`
- **Response message:** `GetAssignedUpdateResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommandService/GetPendingCommands

- **Proto:** `proto/avf/machine/v1/command.proto`
- **Request message:** `GetPendingCommandsRequest`
- **Response message:** `GetPendingCommandsResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "after_command_sequence": 0,
  "limit": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommandService/RejectCommand

- **Proto:** `proto/avf/machine/v1/command.proto`
- **Request message:** `RejectCommandRequest`
- **Response message:** `RejectCommandResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "command_id": "{{$guid}}",
  "command_sequence": 0,
  "reason": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommandService/ReportDiagnosticBundleResult

- **Proto:** `proto/avf/machine/v1/command.proto`
- **Request message:** `ReportDiagnosticBundleResultRequest`
- **Response message:** `ReportDiagnosticBundleResultResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "request_id": "{{$guid}}",
  "storage_key": "",
  "storage_provider": "",
  "content_type": "",
  "size_bytes": 0,
  "sha256_hex": "",
  "metadata": {},
  "expires_at": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommandService/ReportUpdateStatus

- **Proto:** `proto/avf/machine/v1/command.proto`
- **Request message:** `ReportUpdateStatusRequest`
- **Response message:** `ReportUpdateStatusResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "campaign_id": "{{$guid}}",
  "status": "",
  "error_message": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/AttachPaymentResult

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `CreatePaymentSessionRequest`
- **Response message:** `CreatePaymentSessionResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "provider": "",
  "payment_state": "",
  "amount_minor": 0,
  "currency": "",
  "outbox_payload_json": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/CancelOrder

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `CancelOrderRequest`
- **Response message:** `CancelOrderResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "reason": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/ConfirmCashPayment

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `ConfirmCashPaymentRequest`
- **Response message:** `ConfirmCashPaymentResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/ConfirmVendSuccess

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `ConfirmVendSuccessRequest`
- **Response message:** `ConfirmVendSuccessResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "slot_index": 0,
  "correlation_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/CreateCashCheckout

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `ConfirmCashPaymentRequest`
- **Response message:** `ConfirmCashPaymentResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/CreateOrder

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `CreateOrderRequest`
- **Response message:** `CreateOrderResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "machine_id": "{{machineId}}",
  "product_id": "{{$guid}}",
  "slot": {},
  "currency": "",
  "meta": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/CreatePaymentSession

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `CreatePaymentSessionRequest`
- **Response message:** `CreatePaymentSessionResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "provider": "",
  "payment_state": "",
  "amount_minor": 0,
  "currency": "",
  "outbox_payload_json": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/GetOrder

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `GetOrderRequest`
- **Response message:** `GetOrderResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "order_id": "{{$guid}}",
  "slot_index": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/GetOrderStatus

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `GetOrderStatusRequest`
- **Response message:** `GetOrderStatusResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "order_id": "{{$guid}}",
  "slot_index": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/ReportVendFailure

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `ReportVendFailureRequest`
- **Response message:** `ReportVendFailureResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "slot_index": 0,
  "failure_reason": "",
  "correlation_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/ReportVendSuccess

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `ReportVendSuccessRequest`
- **Response message:** `ReportVendSuccessResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "slot_index": 0,
  "correlation_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineCommerceService/StartVend

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `StartVendRequest`
- **Response message:** `StartVendResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "slot_index": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/AckInventorySync

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `AckInventorySyncRequest`
- **Response message:** `AckInventorySyncResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "sync_cursor": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/GetInventorySnapshot

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `GetInventorySnapshotRequest`
- **Response message:** `GetInventorySnapshotResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/GetPlanogram

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `GetPlanogramRequest`
- **Response message:** `GetPlanogramResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/PushInventoryDelta

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `ReportInventoryDeltaRequest`
- **Response message:** `ReportInventoryDeltaResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "meta": {},
  "reason": "",
  "lines": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/SubmitFillReport

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `SubmitFillResultRequest`
- **Response message:** `SubmitFillResultResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "lines": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/SubmitFillResult

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `SubmitFillResultRequest`
- **Response message:** `SubmitFillResultResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "lines": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/SubmitInventoryAdjustment

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `SubmitInventoryAdjustmentRequest`
- **Response message:** `SubmitInventoryAdjustmentResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "reason": "",
  "lines": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/SubmitRestock

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `SubmitRestockRequest`
- **Response message:** `SubmitRestockResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "lines": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/SubmitStockAdjustment

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `SubmitInventoryAdjustmentRequest`
- **Response message:** `SubmitInventoryAdjustmentResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "reason": "",
  "lines": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineInventoryService/SubmitStockSnapshot

- **Proto:** `proto/avf/machine/v1/inventory.proto`
- **Request message:** `SubmitStockSnapshotRequest`
- **Response message:** `SubmitStockSnapshotResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "lines": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineMediaService/AckMediaVersion

- **Proto:** `proto/avf/machine/v1/media.proto`
- **Request message:** `AckMediaVersionRequest`
- **Response message:** `AckMediaVersionResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "acknowledged_media_fingerprint": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineMediaService/GetMediaDelta

- **Proto:** `proto/avf/machine/v1/media.proto`
- **Request message:** `GetMediaDeltaRequest`
- **Response message:** `GetMediaDeltaResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}",
  "basis_media_fingerprint": "",
  "meta": {},
  "include_unavailable": false
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineMediaService/GetMediaManifest

- **Proto:** `proto/avf/machine/v1/media.proto`
- **Request message:** `MachineMediaServiceGetMediaManifestRequest`
- **Response message:** `MachineMediaServiceGetMediaManifestResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}",
  "include_unavailable": false,
  "meta": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineOfflineSyncService/GetSyncCursor

- **Proto:** `proto/avf/machine/v1/offline_sync.proto`
- **Request message:** `GetSyncCursorRequest`
- **Response message:** `GetSyncCursorResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineOfflineSyncService/PushOfflineEvents

- **Proto:** `proto/avf/machine/v1/offline_sync.proto`
- **Request message:** `SyncOfflineEventsRequest`
- **Response message:** `SyncOfflineEventsResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "meta": {},
  "events": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineOperatorService/CloseOperatorSession

- **Proto:** `proto/avf/machine/v1/operator_grpc.proto`
- **Request message:** `CloseOperatorSessionRequest`
- **Response message:** `CloseOperatorSessionResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "session_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineOperatorService/HeartbeatOperatorSession

- **Proto:** `proto/avf/machine/v1/operator_grpc.proto`
- **Request message:** `HeartbeatOperatorSessionRequest`
- **Response message:** `HeartbeatOperatorSessionResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "session_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineOperatorService/LoginOperator

- **Proto:** `proto/avf/machine/v1/operator_grpc.proto`
- **Request message:** `LoginOperatorRequest`
- **Response message:** `LoginOperatorResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineOperatorService/LogoutOperator

- **Proto:** `proto/avf/machine/v1/operator_grpc.proto`
- **Request message:** `LogoutOperatorRequest`
- **Response message:** `LogoutOperatorResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineOperatorService/OpenOperatorSession

- **Proto:** `proto/avf/machine/v1/operator_grpc.proto`
- **Request message:** `OpenOperatorSessionRequest`
- **Response message:** `OpenOperatorSessionResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineOperatorService/SubmitFillReport

- **Proto:** `proto/avf/machine/v1/operator_grpc.proto`
- **Request message:** `SubmitFillReportRequest`
- **Response message:** `SubmitFillReportResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "fill": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineOperatorService/SubmitStockAdjustment

- **Proto:** `proto/avf/machine/v1/operator_grpc.proto`
- **Request message:** `SubmitStockAdjustmentRequest`
- **Response message:** `SubmitStockAdjustmentResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "adjustment": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineSaleService/AttachPayment

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `AttachPaymentRequest`
- **Response message:** `AttachPaymentResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "payment_session": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineSaleService/CancelSale

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `CancelOrderRequest`
- **Response message:** `CancelOrderResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "reason": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineSaleService/CompleteVend

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `ConfirmVendSuccessRequest`
- **Response message:** `ConfirmVendSuccessResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "slot_index": 0,
  "correlation_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineSaleService/ConfirmCashReceived

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `ConfirmCashReceivedRequest`
- **Response message:** `ConfirmCashReceivedResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "payment": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineSaleService/CreateSale

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `CreateSaleRequest`
- **Response message:** `CreateSaleResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "order": {}
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineSaleService/FailVend

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `ReportVendFailureRequest`
- **Response message:** `ReportVendFailureResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "slot_index": 0,
  "failure_reason": "",
  "correlation_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineSaleService/StartVend

- **Proto:** `proto/avf/machine/v1/commerce.proto`
- **Request message:** `StartVendRequest`
- **Response message:** `StartVendResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "order_id": "{{$guid}}",
  "slot_index": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineTelemetryService/CheckIn

- **Proto:** `proto/avf/machine/v1/telemetry.proto`
- **Request message:** `CheckInRequest`
- **Response message:** `CheckInResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "machine_id": "{{machineId}}",
  "android_id": "{{$guid}}",
  "sim_serial": "",
  "package_name": "",
  "version_name": "",
  "version_code": 0,
  "android_release": "",
  "sdk_int": 0,
  "manufacturer": "",
  "model": "",
  "timezone": "",
  "network_state": "",
  "boot_id": "{{$guid}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineTelemetryService/GetEventStatus

- **Proto:** `proto/avf/machine/v1/telemetry.proto`
- **Request message:** `GetEventStatusRequest`
- **Response message:** `GetEventStatusResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "idempotency_key": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineTelemetryService/PushCriticalEvent

- **Proto:** `proto/avf/machine/v1/telemetry.proto`
- **Request message:** `PushCriticalEventRequest`
- **Response message:** `PushCriticalEventResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "meta": {},
  "event": {},
  "severity": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineTelemetryService/PushTelemetryBatch

- **Proto:** `proto/avf/machine/v1/telemetry.proto`
- **Request message:** `PushTelemetryBatchRequest`
- **Response message:** `PushTelemetryBatchResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "meta": {},
  "events": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineTelemetryService/ReconcileEvents

- **Proto:** `proto/avf/machine/v1/telemetry.proto`
- **Request message:** `ReconcileEventsRequest`
- **Response message:** `ReconcileEventsResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "idempotency_keys": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineTelemetryService/SubmitTelemetryBatch

- **Proto:** `proto/avf/machine/v1/telemetry.proto`
- **Request message:** `SubmitTelemetryBatchRequest`
- **Response message:** `SubmitTelemetryBatchResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "context": {},
  "events": []
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.machine.v1.MachineTokenService/RefreshMachineToken

- **Proto:** `proto/avf/machine/v1/machine_token.proto`
- **Request message:** `RefreshMachineTokenRequest`
- **Response message:** `RefreshMachineTokenResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "refresh_token": ""
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.v1.InternalCommerceQueryService/GetOrderPaymentVendState

- **Proto:** `proto/avf/v1/internal_queries.proto`
- **Request message:** `GetOrderPaymentVendStateRequest`
- **Response message:** `GetOrderPaymentVendStateResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "order_id": "{{$guid}}",
  "slot_index": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.v1.InternalMachineQueryService/GetMachineCabinetSlotSummary

- **Proto:** `proto/avf/v1/internal_queries.proto`
- **Request message:** `GetMachineRequest`
- **Response message:** `GetMachineCabinetSlotSummaryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.v1.InternalMachineQueryService/GetMachineState

- **Proto:** `proto/avf/v1/internal_queries.proto`
- **Request message:** `GetMachineRequest`
- **Response message:** `GetMachineStateResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.v1.InternalMachineQueryService/GetMachineSummary

- **Proto:** `proto/avf/v1/internal_queries.proto`
- **Request message:** `GetMachineRequest`
- **Response message:** `GetMachineSummaryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.v1.InternalTelemetryQueryService/GetLatestMachineTelemetry

- **Proto:** `proto/avf/v1/internal_queries.proto`
- **Request message:** `GetMachineRequest`
- **Response message:** `GetLatestMachineTelemetryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}"
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```

## /avf.v1.InternalTelemetryQueryService/GetMachineIncidentSummary

- **Proto:** `proto/avf/v1/internal_queries.proto`
- **Request message:** `GetMachineIncidentSummaryRequest`
- **Response message:** `GetMachineIncidentSummaryResponse`
- **Metadata:** authorization, x-request-id

**Request example:**
```json
{
  "machine_id": "{{machineId}}",
  "limit": 0
}
```

**Response shape:**
```json
{
  "note": "Protobuf JSON encoding theo responseType; xem descriptor."
}
```
