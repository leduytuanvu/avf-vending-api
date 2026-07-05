# Full Surface Test Matrix

Generated from `tools/enterprise_flow/inventory_*.py` + `accepted_surface_exceptions.json`.

## Summary counts

| Surface | Inventory total | Accepted contract-only | Required executable |
|---------|-----------------|------------------------|---------------------|
| REST | 347 | 8 Chi-only OpenAPI gaps | 347 (Chi-only via production runner) |
| gRPC | 92 | 7 UNIMPLEMENTED contract | 85 live + 7 contract-pass |
| MQTT | 12 enterprise publish + ACL negatives | 1 code-only (shadow/desired) | production MQTT runner |

## REST — activation P0 routes

| route_id | method | path | auth | test_type | status |
|----------|--------|------|------|-----------|--------|
| REST-ACT-001 | GET | /v1/admin/machines/{machineId}/activation-codes | bearer admin | live-write-isolated | not_run |
| REST-ACT-002 | POST | /v1/admin/machines/{machineId}/activation-codes | bearer admin | live-write-isolated | not_run |
| REST-ACT-003 | DELETE | /v1/admin/machines/{machineId}/activation-codes/{activationCodeId} | bearer admin | live-write-isolated | not_run |
| REST-ACT-004 | GET | /v1/admin/machine-codes/{machineCode}/activation-codes | bearer admin | live-write-isolated | not_run |
| REST-ACT-005 | POST | /v1/admin/machine-codes/{machineCode}/activation-codes | bearer admin | live-write-isolated | not_run |
| REST-ACT-006 | DELETE | /v1/admin/machine-codes/{machineCode}/activation-codes/{activationCodeId} | bearer admin | live-write-isolated | not_run |
| REST-ACT-007 | GET | /v1/admin/activation-codes | bearer admin | live-write-isolated | not_run |
| REST-ACT-008 | POST | /v1/admin/activation-codes | bearer admin | live-write-isolated | not_run |
| REST-ACT-009 | POST | /v1/admin/activation-codes/{codeId}/revoke | bearer admin | live-write-isolated | not_run |
| REST-ACT-010 | POST | /v1/setup/activation-codes/claim | bearer admin | live-write-isolated | not_run |

Full REST inventory (347 ops): `D:\admin\development\avf\avf-vending-system\avf-vending-api\reports\enterprise-flow-verification\20260703T013119Z\REST_INVENTORY.json`.
Production runner: `tools/production_full_test/run_rest_full_production.py`.

## gRPC inventory

| route_id | service | method | auth | machine_scoped | test_type | status |
|----------|---------|--------|------|----------------|-----------|--------|
| GRPC-001 | avf.internal.v1.InternalCatalogQueryService | GetSaleCatalogSnapshot | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-002 | avf.internal.v1.InternalCommerceQueryService | GetOrderPaymentVendState | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-003 | avf.internal.v1.InternalInventoryQueryService | GetMachineSlotInventory | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-004 | avf.internal.v1.InternalMachineQueryService | GetMachineSummary | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-005 | avf.internal.v1.InternalMachineQueryService | GetMachineState | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-006 | avf.internal.v1.InternalMachineQueryService | GetMachineCabinetSlotSummary | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-007 | avf.internal.v1.InternalPaymentQueryService | GetPaymentById | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-008 | avf.internal.v1.InternalPaymentQueryService | GetLatestPaymentForOrder | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-009 | avf.internal.v1.InternalReportingQueryService | GetSalesSummary | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-010 | avf.internal.v1.InternalTelemetryQueryService | GetLatestMachineTelemetry | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-011 | avf.internal.v1.InternalTelemetryQueryService | GetMachineIncidentSummary | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-012 | avf.machine.v1.MachineActivationService | ClaimActivation | machine_jwt | no | live-readonly-or-isolated-write | not_run |
| GRPC-013 | avf.machine.v1.MachineAuthService | ActivateMachine | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-014 | avf.machine.v1.MachineAuthService | ClaimActivation | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-015 | avf.machine.v1.MachineAuthService | RefreshMachineToken | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-016 | avf.machine.v1.MachineBootstrapService | GetBootstrap | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-017 | avf.machine.v1.MachineBootstrapService | CheckIn | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-018 | avf.machine.v1.MachineBootstrapService | AckConfigVersion | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-019 | avf.machine.v1.MachineBootstrapService | CheckForUpdates | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-020 | avf.machine.v1.MachineCatalogService | GetSaleCatalog | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-021 | avf.machine.v1.MachineCatalogService | SyncSaleCatalog | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-022 | avf.machine.v1.MachineCatalogService | GetCatalogSnapshot | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-023 | avf.machine.v1.MachineCatalogService | SyncCatalogBundle | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-024 | avf.machine.v1.MachineCatalogService | GetCatalogDelta | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-025 | avf.machine.v1.MachineCatalogService | AckCatalogVersion | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-026 | avf.machine.v1.MachineCatalogService | GetMediaManifest | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-027 | avf.machine.v1.MachineCommandService | GetPendingCommands | machine_jwt | yes | contract-only | accepted_contract_only |
| GRPC-028 | avf.machine.v1.MachineCommandService | AckCommand | machine_jwt | yes | contract-only | accepted_contract_only |
| GRPC-029 | avf.machine.v1.MachineCommandService | RejectCommand | machine_jwt | yes | contract-only | accepted_contract_only |
| GRPC-030 | avf.machine.v1.MachineCommandService | GetAssignedUpdate | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-031 | avf.machine.v1.MachineCommandService | ReportUpdateStatus | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-032 | avf.machine.v1.MachineCommandService | ReportDiagnosticBundleResult | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-033 | avf.machine.v1.MachineCommerceService | CreateOrder | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-034 | avf.machine.v1.MachineCommerceService | CreateQuote | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-035 | avf.machine.v1.MachineCommerceService | CreateOrderFromQuote | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-036 | avf.machine.v1.MachineCommerceService | CreatePaymentSession | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-037 | avf.machine.v1.MachineCommerceService | AttachPaymentResult | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-038 | avf.machine.v1.MachineCommerceService | ConfirmCashPayment | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-039 | avf.machine.v1.MachineCommerceService | CreateCashCheckout | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-040 | avf.machine.v1.MachineCommerceService | GetOrder | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-041 | avf.machine.v1.MachineCommerceService | GetOrderStatus | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-042 | avf.machine.v1.MachineCommerceService | StartVend | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-043 | avf.machine.v1.MachineCommerceService | ConfirmVendSuccess | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-044 | avf.machine.v1.MachineCommerceService | ReportVendSuccess | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-045 | avf.machine.v1.MachineCommerceService | ReportVendFailure | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-046 | avf.machine.v1.MachineCommerceService | CancelOrder | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-047 | avf.machine.v1.MachineInventoryService | PushInventoryDelta | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-048 | avf.machine.v1.MachineInventoryService | GetInventorySnapshot | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-049 | avf.machine.v1.MachineInventoryService | AckInventorySync | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-050 | avf.machine.v1.MachineInventoryService | GetPlanogram | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-051 | avf.machine.v1.MachineInventoryService | SubmitStockSnapshot | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-052 | avf.machine.v1.MachineInventoryService | SubmitFillResult | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-053 | avf.machine.v1.MachineInventoryService | SubmitFillReport | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-054 | avf.machine.v1.MachineInventoryService | SubmitRestock | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-055 | avf.machine.v1.MachineInventoryService | SubmitInventoryAdjustment | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-056 | avf.machine.v1.MachineInventoryService | SubmitStockAdjustment | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-057 | avf.machine.v1.MachineMediaService | GetMediaManifest | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-058 | avf.machine.v1.MachineMediaService | GetMediaDelta | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-059 | avf.machine.v1.MachineMediaService | AckMediaVersion | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-060 | avf.machine.v1.MachineOfflineSyncService | PushOfflineEvents | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-061 | avf.machine.v1.MachineOfflineSyncService | GetSyncCursor | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-062 | avf.machine.v1.MachineOperatorService | OpenOperatorSession | machine_jwt | yes | contract-only | accepted_contract_only |
| GRPC-063 | avf.machine.v1.MachineOperatorService | CloseOperatorSession | machine_jwt | yes | contract-only | accepted_contract_only |
| GRPC-064 | avf.machine.v1.MachineOperatorService | SubmitFillReport | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-065 | avf.machine.v1.MachineOperatorService | SubmitStockAdjustment | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-066 | avf.machine.v1.MachineOperatorService | LoginOperator | machine_jwt | yes | contract-only | accepted_contract_only |
| GRPC-067 | avf.machine.v1.MachineOperatorService | LogoutOperator | machine_jwt | yes | contract-only | accepted_contract_only |
| GRPC-068 | avf.machine.v1.MachineOperatorService | HeartbeatOperatorSession | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-069 | avf.machine.v1.MachineRuntimeSessionService | StartRuntimeSession | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-070 | avf.machine.v1.MachineRuntimeSessionService | HeartbeatRuntimeSession | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-071 | avf.machine.v1.MachineRuntimeSessionService | EndRuntimeSession | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-072 | avf.machine.v1.MachineRuntimeSessionService | GetRuntimeSessionState | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-073 | avf.machine.v1.MachineSaleService | CreateSale | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-074 | avf.machine.v1.MachineSaleService | AttachPayment | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-075 | avf.machine.v1.MachineSaleService | ConfirmCashReceived | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-076 | avf.machine.v1.MachineSaleService | StartVend | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-077 | avf.machine.v1.MachineSaleService | CompleteVend | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-078 | avf.machine.v1.MachineSaleService | FailVend | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-079 | avf.machine.v1.MachineSaleService | CancelSale | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-080 | avf.machine.v1.MachineTelemetryService | PushTelemetryBatch | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-081 | avf.machine.v1.MachineTelemetryService | PushCriticalEvent | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-082 | avf.machine.v1.MachineTelemetryService | CheckIn | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-083 | avf.machine.v1.MachineTelemetryService | SubmitTelemetryBatch | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-084 | avf.machine.v1.MachineTelemetryService | ReconcileEvents | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-085 | avf.machine.v1.MachineTelemetryService | GetEventStatus | machine_jwt | yes | live-readonly-or-isolated-write | not_run |
| GRPC-086 | avf.machine.v1.MachineTokenService | RefreshMachineToken | machine_jwt | no | live-readonly-or-isolated-write | not_run |
| GRPC-087 | avf.v1.InternalCommerceQueryService | GetOrderPaymentVendState | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-088 | avf.v1.InternalMachineQueryService | GetMachineSummary | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-089 | avf.v1.InternalMachineQueryService | GetMachineState | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-090 | avf.v1.InternalMachineQueryService | GetMachineCabinetSlotSummary | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-091 | avf.v1.InternalTelemetryQueryService | GetLatestMachineTelemetry | mixed | mixed | live-readonly-or-isolated-write | not_run |
| GRPC-092 | avf.v1.InternalTelemetryQueryService | GetMachineIncidentSummary | mixed | mixed | live-readonly-or-isolated-write | not_run |

## MQTT enterprise publish tails

| route_id | topic_tail | direction | client | test_type | status |
|----------|------------|-----------|--------|-----------|--------|
| MQTT-001 | avf/prod/machines/{machineId}/commands/ack | publish | machine | live-isolated | not_run |
| MQTT-002 | avf/prod/machines/{machineId}/commands/receipt | publish | machine | live-isolated | not_run |
| MQTT-003 | avf/prod/machines/{machineId}/presence | publish | machine | live-isolated | not_run |
| MQTT-004 | avf/prod/machines/{machineId}/state/heartbeat | publish | machine | live-isolated | not_run |
| MQTT-005 | avf/prod/machines/{machineId}/telemetry | publish | machine | live-isolated | not_run |
| MQTT-006 | avf/prod/machines/{machineId}/telemetry/snapshot | publish | machine | live-isolated | not_run |
| MQTT-007 | avf/prod/machines/{machineId}/telemetry/incident | publish | machine | live-isolated | not_run |
| MQTT-008 | avf/prod/machines/{machineId}/events | publish | machine | live-isolated | not_run |
| MQTT-009 | avf/prod/machines/{machineId}/events/vend | publish | machine | live-isolated | not_run |
| MQTT-010 | avf/prod/machines/{machineId}/events/cash | publish | machine | live-isolated | not_run |
| MQTT-011 | avf/prod/machines/{machineId}/events/inventory | publish | machine | live-isolated | not_run |
| MQTT-012 | avf/prod/machines/{machineId}/shadow/reported | publish | machine | live-isolated | not_run |

ACL negatives: cross-machine publish/subscribe, wildcard, JWT-as-password — `run_mqtt_full_production.py`.

## Evidence (after production run)

- `reports/production-full-api-grpc-mqtt/<UTC>/REST_FINAL_COVERAGE.json`
- `reports/production-full-api-grpc-mqtt/<UTC>/GRPC_FINAL_COVERAGE.json`
- `reports/production-full-api-grpc-mqtt/<UTC>/MQTT_FINAL_COVERAGE.json`
- `docs/reports/machine-code-activation-production/evidence/`
