# AVF API Inventory (Canonical)

Generated from current code: OpenAPI swagger, proto files, MQTT topic matrix.

## Counts

- REST: 327
- gRPC: 86
- MQTT: 28

## REST

- `GET /health/live` — 00_Health_System
- `GET /health/ready` — 00_Health_System
- `GET /metrics` — 00_Health_System
- `GET /swagger/doc.json` — 00_Health_System
- `GET /swagger/index.html` — 00_Health_System
- … (327 total)

## gRPC

- `/avf.internal.v1.InternalCatalogQueryService/GetSaleCatalogSnapshot` — 02_Admin_Accounts_RBAC
- `/avf.internal.v1.InternalCommerceQueryService/GetOrderPaymentVendState` — 02_Admin_Accounts_RBAC
- `/avf.internal.v1.InternalInventoryQueryService/GetMachineSlotInventory` — 02_Admin_Accounts_RBAC
- `/avf.internal.v1.InternalMachineQueryService/GetMachineCabinetSlotSummary` — 02_Admin_Accounts_RBAC
- `/avf.internal.v1.InternalMachineQueryService/GetMachineState` — 02_Admin_Accounts_RBAC
- … (86 total)

## MQTT

- `{{mqttTopicPrefix}}/+/telemetry` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/+/presence` (publish) — Machine Heartbeat/Check-in
- `{{mqttTopicPrefix}}/+/state/heartbeat` (publish) — Machine Heartbeat/Check-in
- `{{mqttTopicPrefix}}/+/telemetry/snapshot` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/+/telemetry/incident` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/+/events/vend` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/+/events/cash` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/+/events/inventory` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/+/shadow/reported` (publish) — Config/Shadow
- `{{mqttTopicPrefix}}/+/shadow/desired` (publish) — Config/Shadow
- `{{mqttTopicPrefix}}/+/commands/receipt` (publish) — Command ACK
- `{{mqttTopicPrefix}}/+/commands/ack` (publish) — Command ACK
- `{{mqttTopicPrefix}}/machines/+/telemetry` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/machines/+/presence` (publish) — Machine Heartbeat/Check-in
- `{{mqttTopicPrefix}}/machines/+/state/heartbeat` (publish) — Machine Heartbeat/Check-in
- `{{mqttTopicPrefix}}/machines/+/telemetry/snapshot` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/machines/+/telemetry/incident` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/machines/+/events/vend` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/machines/+/events/cash` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/machines/+/events/inventory` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/machines/+/shadow/reported` (publish) — Config/Shadow
- `{{mqttTopicPrefix}}/machines/+/shadow/desired` (publish) — Config/Shadow
- `{{mqttTopicPrefix}}/machines/+/commands/receipt` (publish) — Command ACK
- `{{mqttTopicPrefix}}/machines/+/commands/ack` (publish) — Command ACK
- `{{mqttTopicPrefix}}/machines/+/events` (publish) — Telemetry Publish
- `{{mqttTopicPrefix}}/{{machineId}}/commands/dispatch` (publish) — Backend Commands
- `{{mqttTopicPrefix}}/{{machineId}}/commands/down` (publish) — Backend Commands
- `{{mqttTopicPrefix}}/machines/{{machineId}}/commands` (publish) — Backend Commands
