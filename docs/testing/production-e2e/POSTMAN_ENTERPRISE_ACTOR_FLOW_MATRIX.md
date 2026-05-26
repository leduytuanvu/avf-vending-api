# Postman enterprise actor & market flow matrix

Generated: 2026-05-26T20:08:39Z

## Market release flows (folder 90)

| ID | Title | Actors | REST | gRPC | MQTT |
|----|-------|--------|------|------|------|
| 90.01 | Admin creates sellable product with image | Admin Web | REST-AUTH-001,REST-CATALOG-001..005,REST-MEDIA-PIPE,REST-MEDIA-001,REST-MEDIA-002 |  |  |
| 90.02 | Admin creates site and machine | Admin Web | REST-SITE-001,REST-MACHINE-001..006 |  |  |
| 90.03 | Technician activates and prepares machine | Technician App | REST-MACHINE-003,REST-OP-001,REST-OP-002 | GRPC-TOKEN-001 | MQTT-CONN-001 |
| 90.04 | Admin assigns product to machine slot | Admin Web | REST-PLANO-000..006 |  |  |
| 90.05 | Admin fills stock | Admin Web,Technician | REST-INV-001,REST-OP-001 | GRPC-INV-001,GRPC-INV-002 |  |
| 90.06 | Machine app syncs bootstrap/catalog/media | Vending Machine App |  | GRPC-BOOT-001,GRPC-CAT-001..003,GRPC-MED-001 |  |
| 90.07 | Machine app caches images for offline display | Vending Machine App |  | GRPC-MED-001 |  |
| 90.08 | Machine app logs/diagnostics telemetry | Vending Machine App | (APP_LOG_NOT_IMPLEMENTED — no dedicated REST app-log endpoint) | GRPC-BOOT-002,GRPC-TELEMETRY via CheckIn | MQTT-TEL-001..004 |
| 90.09 | Customer buys cash/manual (no online payment) | Vending Machine App |  | GRPC-COMM-CASH-001 |  |
| 90.10 | Machine confirms vend success | Vending Machine App |  | GRPC-COMM-CASH-001 |  |
| 90.11 | Inventory decrement and reports update | Backend,Admin Web | REST-REPORT-001..005 | GRPC-INV-002 | MQTT-TEL-004 |
| 90.12 | Backend sends MQTT command to machine | Admin Web,Backend | MQTT-CMD dispatch via REST |  | MQTT-CMD-001 |
| 90.13 | Machine ACKs MQTT command | Vending Machine App |  |  | MQTT-CMD-001,MQTT-NEG-004 |
| 90.14 | Machine online/offline telemetry | Vending Machine App,Monitoring |  | GRPC-BOOT-002 | MQTT-TEL-001,MQTT-TEL-002 |
| 90.15 | Technician restocks and cycle-counts | Technician App | REST-OP-001,REST-OP-002 | GRPC-INV-001 |  |
| 90.16 | Admin reads sales/fleet/inventory reports | Admin Web,Operations | REST-REPORT-001..005 |  | MQTT-READ-001 |
| 90.17 | Vend failure and reconciliation | Vending Machine App,Support |  | GRPC-COMM-FAIL-001,GRPC-COMM-CANCEL-001 |  |
| 90.18 | Offline queue replay and idempotency | Vending Machine App |  | GRPC-OFFLINE-001,GRPC-IDEM-001 |  |
| 90.19 | Cleanup E2E production data | E2E automation,Admin Web | REST-CLEANUP flows in manifest |  |  |
| 90.20 | Final market release checklist | QA,Release Manager | All folders 01-19 + 97/98 documented | 12 Vending Machine App gRPC catalog | 13 canonical MQTT topics |

## REST actor map (sample)

| Flow | Actor | Used by | Market | Folder |
|------|-------|---------|--------|--------|
| REST-AUDIT-001 | ADMIN_WEB | Admin Web | important | 17 - Audit Logs Diagnostics/Admin Audit Logs |
| REST-AUTH-001 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-AUTH-002 | ADMIN_WEB | Admin Web | critical | 02 - Auth/02 Current User |
| REST-AUTH-003 | ADMIN_WEB | Admin Web | critical | 02 - Auth/04 Negative Auth |
| REST-AUTH-004 | ADMIN_WEB | Admin Web | critical | 02 - Auth/02 Current User |
| REST-AUTH-005 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-AUTH-LOGOUT | ADMIN_WEB | Admin Web | critical | 02 - Auth/05 Logout Revoke |
| REST-AUTH-REFRESH | ADMIN_WEB | Admin Web | critical | 02 - Auth/03 Refresh Token |
| REST-CATALOG-001 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-CATALOG-002 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-CATALOG-003 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-CATALOG-004 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-CATALOG-005 | ADMIN_WEB | Admin Web | critical | 06 - Product/Get Product |
| REST-COMMERCE-001 | ADMIN_WEB | Admin Web | important | 97 - Online Payment Guarded/Payment Reconciliation Guarded |
| REST-COMMERCE-002 | ADMIN_WEB | Admin Web | important | 97 - Online Payment Guarded/Payment Reconciliation Guarded |
| REST-COMMERCE-003 | PAYMENT | Payment Provider / Webhook Provider | payment-phase-2 | 97 - Online Payment Guarded/Webhook Guarded |
| REST-COMMERCE-003-DUP | PAYMENT | Payment Provider / Webhook Provider | payment-phase-2 | 97 - Online Payment Guarded/Webhook Guarded |
| REST-COMMERCE-004 | PAYMENT | Payment Provider / Webhook Provider | payment-phase-2 | 97 - Online Payment Guarded/Payment Reconciliation Guarded |
| REST-COMMERCE-005 | PAYMENT | Payment Provider / Webhook Provider | payment-phase-2 | 97 - Online Payment Guarded/Payment Reconciliation Guarded |
| REST-COMMERCE-006 | ADMIN_WEB | Admin Web | important | 97 - Online Payment Guarded/Payment Reconciliation Guarded |
| REST-COV-DEL-0001 | PUBLIC | Security QA | important | 18 - Route Coverage Smoke/Public Smoke |
| REST-COV-DEL-0002 | PUBLIC | Security QA | important | 18 - Route Coverage Smoke/Public Smoke |
| REST-COV-DEL-0003 | ADMIN_WEB | Admin Web | critical | 04 - Brand/Delete Archive Brand |
| REST-COV-DEL-0004 | ADMIN_WEB | Admin Web | critical | 03 - Category/Delete Archive Category |
| REST-COV-DEL-0005 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 10 - Activation/Create Activation Code |
| REST-COV-DEL-0006 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 09 - Machine/Delete Archive Machine |
| REST-COV-DEL-0007 | ADMIN_WEB | Admin Web | critical | 07 - Media/Media Detail |
| REST-COV-DEL-0008 | ADMIN_WEB | Admin Web | critical | 07 - Media/Media Detail |
| REST-COV-DEL-0009 | PUBLIC | Security QA | important | 18 - Route Coverage Smoke/Public Smoke |
| REST-COV-DEL-0010 | PUBLIC | Security QA | important | 18 - Route Coverage Smoke/Public Smoke |
| REST-COV-DEL-0011 | ADMIN_WEB | Admin Web | critical | 06 - Product/Delete Archive Product |
| REST-COV-DEL-0012 | ADMIN_WEB | Admin Web | critical | 06 - Product/Product Image Attach |
| REST-COV-DEL-0013 | ADMIN_WEB | Admin Web | critical | 06 - Product/Product Image Attach |
| REST-COV-DEL-0014 | PUBLIC | Security QA | important | 18 - Route Coverage Smoke/Public Smoke |
| REST-COV-DEL-0015 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 08 - Site/Delete Archive Site |
| REST-COV-DEL-0016 | ADMIN_WEB | Admin Web | critical | 05 - Tag/Delete Archive Tag |
| REST-COV-DEL-0017 | PUBLIC | Security QA | important | 18 - Route Coverage Smoke/Public Smoke |
| REST-COV-DEL-0018 | PUBLIC | Security QA | important | 18 - Route Coverage Smoke/Public Smoke |
| REST-COV-DEL-0019 | ADMIN_WEB | Admin Web | critical | 02 - Auth/04 Negative Auth |
| REST-COV-DEL-0020 | ADMIN_WEB | Admin Web | critical | 02 - Auth/04 Negative Auth |
| REST-COV-GET-0021 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0022 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0023 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0024 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0025 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0026 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0027 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0028 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0029 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0030 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0031 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0032 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0033 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0034 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0035 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0036 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0037 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0038 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0039 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0040 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0041 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0042 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0043 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0044 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0045 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0046 | ADMIN_WEB | Admin Web, Technician App / Operator | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0047 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0048 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0049 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0050 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0051 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0052 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0053 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0054 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0055 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0056 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0057 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0058 | ADMIN_WEB | Admin Web | critical | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0059 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |
| REST-COV-GET-0060 | ADMIN_WEB | Admin Web | important | 98 - Optional Contract Disabled/Config Required Features |

## gRPC actor map

| Service | RPC | Actor | E2E |
|---------|-----|-------|-----|
| MachineAuthService | ActivateMachine | MACHINE_APP | NO |
| MachineAuthService | ClaimActivation | MACHINE_APP | NO |
| MachineAuthService | RefreshMachineToken | MACHINE_APP | NO |
| MachineBootstrapService | GetBootstrap | MACHINE_APP | YES |
| MachineBootstrapService | CheckIn | MACHINE_APP | YES |
| MachineBootstrapService | AckConfigVersion | MACHINE_APP | NO |
| MachineBootstrapService | CheckForUpdates | MACHINE_APP | NO |
| MachineCatalogService | GetSaleCatalog | MACHINE_APP | YES |
| MachineCatalogService | SyncSaleCatalog | MACHINE_APP | NO |
| MachineCatalogService | GetCatalogSnapshot | MACHINE_APP | NO |
| MachineCatalogService | SyncCatalogBundle | MACHINE_APP | NO |
| MachineCatalogService | GetCatalogDelta | MACHINE_APP | YES |
| MachineCatalogService | AckCatalogVersion | MACHINE_APP | YES |
| MachineCatalogService | GetMediaManifest | MACHINE_APP | NO |
| MachineCommandService | GetPendingCommands | MACHINE_APP | NO |
| MachineCommandService | AckCommand | MACHINE_APP | NO |
| MachineCommandService | RejectCommand | MACHINE_APP | NO |
| MachineCommandService | GetAssignedUpdate | MACHINE_APP | NO |
| MachineCommandService | ReportUpdateStatus | MACHINE_APP | NO |
| MachineCommandService | ReportDiagnosticBundleResult | MACHINE_APP | NO |
| MachineCommerceService | CreateOrder | MACHINE_APP | NO |
| MachineCommerceService | CreatePaymentSession | PAYMENT | NO |
| MachineCommerceService | AttachPaymentResult | PAYMENT | NO |
| MachineCommerceService | ConfirmCashPayment | MACHINE_APP | NO |
| MachineCommerceService | CreateCashCheckout | MACHINE_APP | NO |
| MachineCommerceService | GetOrder | MACHINE_APP | NO |
| MachineCommerceService | GetOrderStatus | MACHINE_APP | NO |
| MachineCommerceService | StartVend | MACHINE_APP | NO |
| MachineCommerceService | ConfirmVendSuccess | MACHINE_APP | NO |
| MachineCommerceService | ReportVendSuccess | MACHINE_APP | NO |
| MachineCommerceService | ReportVendFailure | MACHINE_APP | NO |
| MachineCommerceService | CancelOrder | MACHINE_APP | NO |
| MachineInventoryService | PushInventoryDelta | MACHINE_APP | NO |
| MachineInventoryService | GetInventorySnapshot | MACHINE_APP | YES |
| MachineInventoryService | AckInventorySync | MACHINE_APP | YES |
| MachineInventoryService | GetPlanogram | MACHINE_APP | NO |
| MachineInventoryService | SubmitStockSnapshot | MACHINE_APP | NO |
| MachineInventoryService | SubmitFillResult | MACHINE_APP | NO |
| MachineInventoryService | SubmitFillReport | MACHINE_APP | NO |
| MachineInventoryService | SubmitRestock | MACHINE_APP | NO |
| MachineInventoryService | SubmitInventoryAdjustment | MACHINE_APP | NO |
| MachineInventoryService | SubmitStockAdjustment | MACHINE_APP | NO |
| MachineActivationService | ClaimActivation | MACHINE_APP | NO |
| MachineTokenService | RefreshMachineToken | MACHINE_APP | YES |
| MachineMediaService | GetMediaManifest | MACHINE_APP | YES |
| MachineMediaService | GetMediaDelta | MACHINE_APP | YES |
| MachineMediaService | AckMediaVersion | MACHINE_APP | YES |
| MachineOfflineSyncService | PushOfflineEvents | MACHINE_APP | NO |
| MachineOfflineSyncService | GetSyncCursor | MACHINE_APP | NO |
| MachineOperatorService | OpenOperatorSession | TECHNICIAN | NO |
| MachineOperatorService | CloseOperatorSession | TECHNICIAN | NO |
| MachineOperatorService | SubmitFillReport | TECHNICIAN | NO |
| MachineOperatorService | SubmitStockAdjustment | TECHNICIAN | NO |
| MachineOperatorService | LoginOperator | TECHNICIAN | NO |
| MachineOperatorService | LogoutOperator | TECHNICIAN | NO |
| MachineOperatorService | HeartbeatOperatorSession | TECHNICIAN | NO |
| MachineTelemetryService | PushTelemetryBatch | MACHINE_APP | NO |
| MachineTelemetryService | PushCriticalEvent | MACHINE_APP | NO |
| MachineTelemetryService | CheckIn | MACHINE_APP | NO |
| MachineTelemetryService | SubmitTelemetryBatch | MACHINE_APP | NO |
| MachineTelemetryService | ReconcileEvents | MACHINE_APP | NO |
| MachineTelemetryService | GetEventStatus | MACHINE_APP | NO |

## MQTT actor map

| Topic | Actor | Direction |
|-------|-------|-----------|
| `presence` | MQTT_MACHINE | publish |
| `state/heartbeat` | MQTT_MACHINE | publish |
| `telemetry/snapshot` | MQTT_MACHINE | publish |
| `telemetry/incident` | MQTT_MACHINE | publish |
| `events/vend` | MQTT_MACHINE | publish |
| `events/cash` | MQTT_MACHINE | publish |
| `events/inventory` | MQTT_MACHINE | publish |
| `commands/ack` | MQTT_MACHINE | publish |
| `commands/receipt` | MQTT_MACHINE | publish |
| `shadow/reported` | MQTT_MACHINE | publish |
| `shadow/desired` | MQTT_MACHINE | publish |
| `commands` | MQTT_BACKEND | subscribe |
| `telemetry` | MQTT_MACHINE | publish |
