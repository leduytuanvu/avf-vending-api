# API / gRPC / MQTT full inventory

Generated: 2026-05-27T08:39:55.492385+00:00
Source commit: run from tools/generate_market_readiness_inventory.py

## Counts

| Surface | Count |
|---------|-------|
| REST operations | 329 |
| gRPC RPCs | 80 |
| MQTT channel definitions | 16 |
| Worker processes | 7 |

### REST by surface

- **canonical**: 286
- **legacy**: 43

### REST by audience

- **Admin Web**: 278
- **Authenticated**: 8
- **Machine App**: 36
- **Payment Provider**: 1
- **Public/System**: 6

## REST (from OpenAPI)

| Method | Path | Audience | Auth | Surface | operationId |
|--------|------|----------|------|---------|-------------|
| GET | `/health/live` | Public/System | none | canonical | DocOpHealthLive |
| GET | `/health/ready` | Public/System | none | canonical | DocOpHealthReady |
| GET | `/metrics` | Public/System | none | canonical | DocOpMetrics |
| GET | `/swagger/doc.json` | Public/System | none | canonical | DocOpSwaggerDocJSON |
| GET | `/swagger/index.html` | Public/System | none | canonical | DocOpSwaggerIndex |
| GET | `/v1/admin/activation-codes` | Admin Web | bearer | canonical | DocOpV1AdminActivationCodesCatalogList |
| POST | `/v1/admin/activation-codes` | Admin Web | bearer | canonical | DocOpV1AdminActivationCodesCatalogCreate |
| POST | `/v1/admin/activation-codes/{codeId}/revoke` | Admin Web | bearer | canonical | DocOpV1AdminActivationCodeCatalogRevoke |
| GET | `/v1/admin/anomalies` | Admin Web | bearer | canonical | DocOpV1AdminOperationalAnomaliesList |
| GET | `/v1/admin/anomalies/{anomalyId}` | Admin Web | bearer | canonical | DocOpV1AdminOperationalAnomalyGet |
| POST | `/v1/admin/anomalies/{anomalyId}/ignore` | Admin Web | bearer | canonical | DocOpV1AdminOperationalAnomalyIgnore |
| POST | `/v1/admin/anomalies/{anomalyId}/resolve` | Admin Web | bearer | canonical | DocOpV1AdminOperationalAnomalyResolve |
| GET | `/v1/admin/artifacts` | Admin Web | bearer | canonical | DocOpV1AdminArtifactsList |
| POST | `/v1/admin/artifacts` | Admin Web | bearer | canonical | DocOpV1AdminArtifactsReserve |
| DELETE | `/v1/admin/artifacts/{artifactId}` | Admin Web | bearer | canonical | DocOpV1AdminArtifactsDelete |
| GET | `/v1/admin/artifacts/{artifactId}` | Admin Web | bearer | canonical | DocOpV1AdminArtifactsGet |
| PUT | `/v1/admin/artifacts/{artifactId}/content` | Admin Web | bearer | canonical | DocOpV1AdminArtifactsPutContent |
| GET | `/v1/admin/artifacts/{artifactId}/download` | Admin Web | bearer | canonical | DocOpV1AdminArtifactsDownloadURL |
| GET | `/v1/admin/assignments` | Admin Web | bearer | canonical | DocOpV1AdminAssignmentsList |
| POST | `/v1/admin/assignments` | Admin Web | bearer | canonical | DocOpV1AdminAssignmentCreate |
| DELETE | `/v1/admin/assignments/{assignmentId}` | Admin Web | bearer | canonical | DocOpV1AdminAssignmentDelete |
| GET | `/v1/admin/assignments/{assignmentId}` | Admin Web | bearer | canonical | DocOpV1AdminAssignmentGet |
| GET | `/v1/admin/audit/events` | Admin Web | bearer | canonical | DocOpV1AdminAuditEventsList |
| GET | `/v1/admin/audit/events/{auditEventId}` | Admin Web | bearer | canonical | DocOpV1AdminAuditEventGet |
| GET | `/v1/admin/auth/users` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersList |
| POST | `/v1/admin/auth/users` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersCreate |
| GET | `/v1/admin/auth/users/{accountId}` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersGet |
| PATCH | `/v1/admin/auth/users/{accountId}` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersPatch |
| POST | `/v1/admin/auth/users/{accountId}/activate` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersActivate |
| POST | `/v1/admin/auth/users/{accountId}/deactivate` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersDeactivate |
| POST | `/v1/admin/auth/users/{accountId}/reset-password` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersResetPassword |
| POST | `/v1/admin/auth/users/{accountId}/revoke-sessions` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersRevokeSessions |
| PATCH | `/v1/admin/auth/users/{accountId}/roles` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersPatchRoles |
| POST | `/v1/admin/auth/users/{accountId}/roles` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersPostRoles |
| PUT | `/v1/admin/auth/users/{accountId}/roles` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersPutRoles |
| GET | `/v1/admin/auth/users/{accountId}/sessions` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersSessions |
| PATCH | `/v1/admin/auth/users/{accountId}/status` | Admin Web | bearer | canonical | DocOpV1AdminAuthUsersPatchStatus |
| GET | `/v1/admin/brands` | Admin Web | bearer | canonical | DocOpV1AdminBrandsList |
| POST | `/v1/admin/brands` | Admin Web | bearer | canonical | DocOpV1AdminBrandCreate |
| DELETE | `/v1/admin/brands/{brandId}` | Admin Web | bearer | canonical | DocOpV1AdminBrandDelete |
| PATCH | `/v1/admin/brands/{brandId}` | Admin Web | bearer | canonical | DocOpV1AdminBrandPatch |
| PUT | `/v1/admin/brands/{brandId}` | Admin Web | bearer | canonical | DocOpV1AdminBrandReplace |
| GET | `/v1/admin/categories` | Admin Web | bearer | canonical | DocOpV1AdminCategoriesList |
| POST | `/v1/admin/categories` | Admin Web | bearer | canonical | DocOpV1AdminCategoryCreate |
| DELETE | `/v1/admin/categories/{categoryId}` | Admin Web | bearer | canonical | DocOpV1AdminCategoryDelete |
| PATCH | `/v1/admin/categories/{categoryId}` | Admin Web | bearer | canonical | DocOpV1AdminCategoryPatch |
| PUT | `/v1/admin/categories/{categoryId}` | Admin Web | bearer | canonical | DocOpV1AdminCategoryReplace |
| GET | `/v1/admin/commands` | Admin Web | bearer | canonical | DocOpV1AdminCommandsList |
| GET | `/v1/admin/commands/{commandId}` | Admin Web | bearer | canonical | DocOpV1AdminCommandGet |
| POST | `/v1/admin/commands/{commandId}/cancel` | Admin Web | bearer | canonical | DocOpV1AdminCommandCancel |
| … | *279 more* | | | | |

Full list: `build/reports/api-grpc-mqtt-full-inventory.json`

## gRPC

| Package | Service | RPC | Surface |
|---------|---------|-----|---------|
| avf.machine.v1 | MachineAuthService | ActivateMachine | legacy |
| avf.machine.v1 | MachineAuthService | ClaimActivation | legacy |
| avf.machine.v1 | MachineAuthService | RefreshMachineToken | legacy |
| avf.machine.v1 | MachineBootstrapService | GetBootstrap | canonical |
| avf.machine.v1 | MachineBootstrapService | CheckIn | canonical |
| avf.machine.v1 | MachineBootstrapService | AckConfigVersion | canonical |
| avf.machine.v1 | MachineBootstrapService | CheckForUpdates | canonical |
| avf.machine.v1 | MachineCatalogService | GetSaleCatalog | canonical |
| avf.machine.v1 | MachineCatalogService | SyncSaleCatalog | canonical |
| avf.machine.v1 | MachineCatalogService | GetCatalogSnapshot | canonical |
| avf.machine.v1 | MachineCatalogService | SyncCatalogBundle | canonical |
| avf.machine.v1 | MachineCatalogService | GetCatalogDelta | canonical |
| avf.machine.v1 | MachineCatalogService | AckCatalogVersion | canonical |
| avf.machine.v1 | MachineCatalogService | GetMediaManifest | canonical |
| avf.machine.v1 | MachineCommandService | GetPendingCommands | canonical |
| avf.machine.v1 | MachineCommandService | AckCommand | canonical |
| avf.machine.v1 | MachineCommandService | RejectCommand | canonical |
| avf.machine.v1 | MachineCommandService | GetAssignedUpdate | canonical |
| avf.machine.v1 | MachineCommandService | ReportUpdateStatus | canonical |
| avf.machine.v1 | MachineCommandService | ReportDiagnosticBundleResult | canonical |
| avf.machine.v1 | MachineCommerceService | CreateOrder | canonical |
| avf.machine.v1 | MachineCommerceService | CreatePaymentSession | canonical |
| avf.machine.v1 | MachineCommerceService | AttachPaymentResult | canonical |
| avf.machine.v1 | MachineCommerceService | ConfirmCashPayment | canonical |
| avf.machine.v1 | MachineCommerceService | CreateCashCheckout | canonical |
| avf.machine.v1 | MachineCommerceService | GetOrder | canonical |
| avf.machine.v1 | MachineCommerceService | GetOrderStatus | canonical |
| avf.machine.v1 | MachineCommerceService | StartVend | canonical |
| avf.machine.v1 | MachineCommerceService | ConfirmVendSuccess | canonical |
| avf.machine.v1 | MachineCommerceService | ReportVendSuccess | canonical |
| avf.machine.v1 | MachineCommerceService | ReportVendFailure | canonical |
| avf.machine.v1 | MachineCommerceService | CancelOrder | canonical |
| avf.machine.v1 | MachineSaleService | CreateSale | legacy_companion |
| avf.machine.v1 | MachineSaleService | AttachPayment | legacy_companion |
| avf.machine.v1 | MachineSaleService | ConfirmCashReceived | legacy_companion |
| avf.machine.v1 | MachineSaleService | StartVend | legacy_companion |
| avf.machine.v1 | MachineSaleService | CompleteVend | legacy_companion |
| avf.machine.v1 | MachineSaleService | FailVend | legacy_companion |
| avf.machine.v1 | MachineSaleService | CancelSale | legacy_companion |
| avf.machine.v1 | MachineInventoryService | PushInventoryDelta | canonical |
| avf.machine.v1 | MachineInventoryService | GetInventorySnapshot | canonical |
| avf.machine.v1 | MachineInventoryService | AckInventorySync | canonical |
| avf.machine.v1 | MachineInventoryService | GetPlanogram | canonical |
| avf.machine.v1 | MachineInventoryService | SubmitStockSnapshot | canonical |
| avf.machine.v1 | MachineInventoryService | SubmitFillResult | canonical |
| avf.machine.v1 | MachineInventoryService | SubmitFillReport | canonical |
| avf.machine.v1 | MachineInventoryService | SubmitRestock | canonical |
| avf.machine.v1 | MachineInventoryService | SubmitInventoryAdjustment | canonical |
| avf.machine.v1 | MachineInventoryService | SubmitStockAdjustment | canonical |
| avf.machine.v1 | MachineActivationService | ClaimActivation | canonical |
| avf.machine.v1 | MachineTokenService | RefreshMachineToken | canonical |
| avf.machine.v1 | MachineMediaService | GetMediaManifest | canonical |
| avf.machine.v1 | MachineMediaService | GetMediaDelta | canonical |
| avf.machine.v1 | MachineMediaService | AckMediaVersion | canonical |
| avf.machine.v1 | MachineOfflineSyncService | PushOfflineEvents | canonical |
| avf.machine.v1 | MachineOfflineSyncService | GetSyncCursor | canonical |
| avf.machine.v1 | MachineOperatorService | OpenOperatorSession | canonical |
| avf.machine.v1 | MachineOperatorService | CloseOperatorSession | canonical |
| avf.machine.v1 | MachineOperatorService | SubmitFillReport | canonical |
| avf.machine.v1 | MachineOperatorService | SubmitStockAdjustment | canonical |
| avf.machine.v1 | MachineOperatorService | LoginOperator | canonical |
| avf.machine.v1 | MachineOperatorService | LogoutOperator | canonical |
| avf.machine.v1 | MachineOperatorService | HeartbeatOperatorSession | canonical |
| avf.machine.v1 | MachineTelemetryService | PushTelemetryBatch | canonical |
| avf.machine.v1 | MachineTelemetryService | PushCriticalEvent | canonical |
| avf.machine.v1 | MachineTelemetryService | CheckIn | canonical |
| avf.machine.v1 | MachineTelemetryService | SubmitTelemetryBatch | canonical |
| avf.machine.v1 | MachineTelemetryService | ReconcileEvents | canonical |
| avf.machine.v1 | MachineTelemetryService | GetEventStatus | canonical |
| avf.internal.v1 | InternalCatalogQueryService | GetSaleCatalogSnapshot | canonical |
| avf.internal.v1 | InternalCommerceQueryService | GetOrderPaymentVendState | canonical |
| avf.internal.v1 | InternalInventoryQueryService | GetMachineSlotInventory | canonical |
| avf.internal.v1 | InternalMachineQueryService | GetMachineSummary | canonical |
| avf.internal.v1 | InternalMachineQueryService | GetMachineState | canonical |
| avf.internal.v1 | InternalMachineQueryService | GetMachineCabinetSlotSummary | canonical |
| avf.internal.v1 | InternalTelemetryQueryService | GetLatestMachineTelemetry | canonical |
| avf.internal.v1 | InternalTelemetryQueryService | GetMachineIncidentSummary | canonical |
| avf.internal.v1 | InternalPaymentQueryService | GetPaymentById | canonical |
| avf.internal.v1 | InternalPaymentQueryService | GetLatestPaymentForOrder | canonical |
| avf.internal.v1 | InternalReportingQueryService | GetSalesSummary | canonical |

## MQTT

| Channel | Direction | Layout | Surface |
|---------|-----------|--------|---------|
| `presence` | machine_to_backend | both | canonical |
| `state/heartbeat` | machine_to_backend | both | canonical |
| `telemetry/snapshot` | machine_to_backend | both | canonical |
| `telemetry/incident` | machine_to_backend | both | canonical |
| `events/vend` | machine_to_backend | both | canonical |
| `events/cash` | machine_to_backend | both | canonical |
| `events/inventory` | machine_to_backend | both | canonical |
| `commands/down` | backend_to_machine | both | legacy |
| `commands/ack` | backend_to_machine | both | canonical |
| `commands/receipt` | backend_to_machine | both | legacy |
| `commands/dispatch` | backend_to_machine | both | legacy |
| `shadow/desired` | backend_to_machine | both | canonical |
| `shadow/reported` | machine_to_backend | both | canonical |
| `telemetry` | machine_to_backend | both | legacy |
| `subscription:InboundDeviceTopicPatterns` | machine_to_backend | legacy | legacy |
| `subscription:InboundEnterpriseDeviceTopicPatterns` | machine_to_backend | enterprise | canonical |

## Workers

- **api** (`cmd/api/main.go`): HTTP + optional gRPC
- **worker** (`cmd/worker/main.go`): outbox, telemetry projection, payment timeout, retention
- **reconciler** (`cmd/reconciler/main.go`): order/vend/psp/refund reconciliation ticks
- **mqtt-ingest** (`cmd/mqtt-ingest/main.go`): MQTT subscribe + telemetry pipeline + command ack sweep
- **temporal-worker** (`cmd/temporal-worker/main.go`): Temporal workflows/activities
- **migrate** (`cmd/migrate/main.go`): goose migrations one-shot
- **outbox-replay** (`cmd/outbox-replay/main.go`): CLI outbox list/requeue
