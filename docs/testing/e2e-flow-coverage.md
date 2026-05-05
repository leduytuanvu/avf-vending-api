# E2E flow coverage matrix (foundation)

This document maps **business flows** to **REST (Web Admin / Postman)**, **gRPC (Vending Machine App)**, and **MQTT** contracts. It aligns with **[`field-test-cases.md`](field-test-cases.md)** (field pilot), **[`docs/api/mqtt-contract.md`](../api/mqtt-contract.md)** (topics), **[`docs/swagger/swagger.json`](../swagger/swagger.json)** (admin + machine REST), and **`proto/avf/machine/v1/*.proto`**.

**Automation status:** all rows are **`planned`** until `tests/e2e/` harness lands.

**Postman:** the checked-in **[`docs/postman/avf-vending-api.postman_collection.json`](../postman/avf-vending-api.postman_collection.json)** currently emphasizes **Public** health/version; import OpenAPI from `{{swagger_url}}` (`GET /swagger/doc.json`) for full admin routing parity. Where a dedicated named request does not exist yet, the column references the **canonical path**.

---

## Legend

| Column | Meaning |
|--------|---------|
| **Safety level** | Intended environment posture for automation (no production secrets in repos). |
| **Cleanup** | How to restore local/staging state after a run. |

---

## Matrix

| flow_id | flow_name | owner | protocol | endpoint_or_rpc_or_topic | Postman request name if REST | required test data | expected result | automation status | safety level | cleanup strategy |
|--------|-----------|-------|----------|--------------------------|--------------------------------|-------------------|-----------------|-------------------|--------------|------------------|
| WA-001 | Public API health & version | platform | REST | `GET /health/live`, `GET /health/ready`, `GET /version` | GET /health/live (collection) | None | 200, ready gate passes | planned | read-only | None |
| WA-002 | Admin login & session | web-admin | REST | `POST /v1/auth/login`, `GET /v1/auth/me` | (import from OpenAPI) | Admin user, password | JWT; `me` reflects roles | planned | write-local-only | Logout / expiry |
| WA-003 | Org & site scaffolding | web-admin | REST | `GET/POST` org/sites per OpenAPI under `/v1/admin/organizations/...` | (import from OpenAPI) | Org admin, org name | Org + site IDs | planned | write-local-only | Delete test org or reset DB scratch |
| WA-004 | Catalog: categories | web-admin | REST | `/v1/admin/categories`, `/v1/admin/categories/{categoryId}` | (import from OpenAPI) | Admin JWT | CRUD per policy | planned | write-local-only | Delete created categories |
| WA-005 | Catalog: brands & products | web-admin | REST | `/v1/admin/brands`, `/v1/admin/products` (per swagger) | (import from OpenAPI) | Admin JWT, SKU | Product visible in list/detail | planned | write-local-only | Archive/delete products |
| WA-006 | Machine registration & lifecycle | web-admin | REST | `/v1/admin/machines`, `/v1/admin/machines/{machineId}`, enable/disable/retire | (import from OpenAPI) | Org, site, serial | Machine row + status | planned | write-local-only | Retire + purge or reset |
| WA-007 | Activation codes (per-machine & org) | web-admin | REST | `/v1/admin/machines/{machineId}/activation-codes`, `/v1/admin/organizations/{organizationId}/activation-codes` | (import from OpenAPI) | Machine, policy | Code issued, revocable | planned | write-local-only | Revoke codes |
| WA-008 | Planogram draft & publish | web-admin | REST | `/v1/admin/machines/{machineId}/planograms/draft`, `/publish` | (import from OpenAPI) | Machine, slots, products | New `catalog_version` on machine | planned | write-local-only | Republish prior or reset |
| WA-009 | Slot & inventory admin views | web-admin | REST | `/v1/admin/machines/{machineId}/slots`, `/inventory`, `/inventory-events`, stock adjustments | (import from OpenAPI) | Machine, slot IDs | Quantities match actions | planned | write-local-only | Reverse adjustments in scratch |
| WA-010 | Remote commands (admin → machine) | web-admin | mixed | REST: `/v1/admin/commands`, `/v1/admin/organizations/{organizationId}/commands`; MQTT: see PF-004 | (import from OpenAPI) | Machine active, command type | Ledger row; optional MQTT delivery | planned | write-staging-only | Cancel command or complete ACK path |
| WA-011 | Commerce reconciliation (read paths) | web-admin | REST | `/v1/admin/organizations/{organizationId}/commerce/reconciliation` (per swagger) | (import from OpenAPI) | Org, date range, orders | Totals consistent | planned | read-only | None |
| WA-012 | Finance daily close | web-admin | REST | `/v1/admin/finance/daily-close` | (import from OpenAPI) | Org, period | Close record | planned | write-local-only | Void per runbook if supported |
| WA-013 | Media upload & complete | web-admin | REST | `/v1/admin/media/*`, `/v1/admin/media/assets/*` | (import from OpenAPI) | Asset bytes or presign flow | Media attached | planned | write-local-only | Delete media assets |
| WA-014 | Diagnostics bundle request | web-admin | mixed | REST: `/v1/admin/machines/{machineId}/diagnostics/requests`; gRPC: `MachineCommandService.ReportDiagnosticBundleResult` | (import from OpenAPI) | Machine JWT + admin request | Bundle stored / linked | planned | write-local-only | TTL expiry |
| WA-015 | Audit & anomaly triage | web-admin | REST | `/v1/admin/audit/events`, org anomalies routes | (import from OpenAPI) | Events in window | Resolve/ignore flows | planned | write-staging-only | Mark resolved |
| WA-016 | Operator REST (kiosk helper) | support | REST | `/v1/machines/{machineId}/operator-sessions/*`, `sale-catalog`, `shadow` | (import from OpenAPI) | Machine/session tokens per policy | Session timeline | planned | write-local-only | Logout session |
| VM-001 | Claim activation & machine JWT | vending-app | gRPC | `MachineAuthService.ActivateMachine` / `ClaimActivation`; `MachineTokenService.RefreshMachineToken` | — | Valid activation code | `machine_token` + claims | planned | write-local-only | Revoke credential admin-side |
| VM-002 | Bootstrap & config version | vending-app | gRPC | `MachineBootstrapService.GetBootstrap`, `AckConfigVersion` (per bootstrap proto) | — | Activated machine | Config + version | planned | write-local-only | Replay-safe |
| VM-003 | Catalog snapshot & delta | vending-app | gRPC | `MachineCatalogService.GetCatalogSnapshot`, `GetCatalogDelta` | — | Published planogram | `catalog_version`; basis match | planned | read-only | None |
| VM-004 | Inventory (machine-scoped) | vending-app | gRPC | `MachineInventoryService` RPCs (per inventory.proto) | — | Slots with stock | Levels match admin | planned | write-local-only | Admin stock adjust |
| VM-005 | Sale: PSP QR order path | vending-app | gRPC | `MachineCommerceService.CreateOrder`, `CreatePaymentSession`, `GetOrder`, `GetOrderStatus` | — | Product, slot, sandbox PSP | Order → paid → vend | planned | write-local-only | Refund path WA or VM |
| VM-006 | Sale: MachineSaleService path | vending-app | gRPC | `MachineSaleService.CreateSale`, `AttachPayment`, `StartVend`, `CompleteVend` | — | Same as VM-005 | Terminal vend success | planned | write-local-only | Cancel/refund |
| VM-007 | Cash tender | vending-app | gRPC | `ConfirmCashPayment` / `CreateCashCheckout`; `ConfirmCashReceived` | — | Cash-enabled SKU | Order completed without PSP | planned | write-local-only | Ledger cleanup scratch |
| VM-008 | Vend success idempotency | vending-app | gRPC | `StartVend`, `ConfirmVendSuccess` / `ReportVendSuccess` | — | Paid order | Single inventory decrement | planned | write-local-only | See troubleshooting |
| VM-009 | Vend failure & refund | vending-app | mixed | `ReportVendFailure`; admin refund REST per swagger | — | Paid order | Refund or failed vend state | planned | write-local-only | Financial reconciliation scratch |
| VM-010 | Telemetry batch & critical | vending-app | gRPC | `MachineTelemetryService.PushTelemetryBatch`, `PushCriticalEvent`, `CheckIn`, `SubmitTelemetryBatch` | — | Idempotency keys | Accepted; duplicates surfaced | planned | write-local-only | N/A |
| VM-011 | Offline replay & reconcile | vending-app | gRPC | `MachineOfflineSyncService` (per offline_sync.proto); `ReconcileEvents` telemetry | — | `offline_sequence`, client ids | REPLAYED / ordering | planned | write-local-only | Fresh machine or reset |
| VM-012 | Media fetch (machine) | vending-app | gRPC | `MachineMediaService` (per media.proto) | — | Catalog manifest URLs | Bytes + hash | planned | read-only | Cache clear on device |
| VM-013 | Operator gRPC | vending-app | gRPC | `MachineOperatorService` (per operator_grpc.proto) | — | Machine JWT | Operator actions logged | planned | write-local-only | End session |
| PF-001 | MQTT command dispatch (API → device) | platform | MQTT | Legacy: `{prefix}/{machineId}/commands/dispatch`; Enterprise: `{prefix}/machines/{machineId}/commands` | — | Active machine, broker ACL | Payload on wire QoS1 | planned | prod-safe-test-machine-only | Cancel follow-up |
| PF-002 | MQTT command ACK / receipt | platform | MQTT | `{prefix}/.../commands/ack`, `commands/receipt` | — | `command_id`, `sequence`, `machine_id` | Ledger **acked** | planned | prod-safe-test-machine-only | None |
| PF-003 | MQTT telemetry & events ingress | platform | MQTT | `{prefix}/+/telemetry`, `telemetry/snapshot`, `events/*`, `shadow/reported`, … | — | Device certs | Ingest accepts | planned | prod-safe-test-machine-only | Ops purge if test volume |
| PF-004 | Admin command → MQTT correlation | platform | mixed | WA-010 + PF-001 + PF-002 | — | Same `command_id` | End-to-end ack | planned | write-staging-only | Retry or cancel |
| SP-001 | Rate limit / error envelope | support | REST | Any `/v1/**` with throttling (if enabled) | — | Burst client | 429 JSON envelope | planned | read-only | Backoff |
| SP-002 | Idempotency conflict (REST) | support | REST | Mutating routes with `Idempotency-Key` | — | Same body diff | 409 `illegal_transition` / mismatch | planned | write-local-only | Use fresh key |

---

## gRPC inventory (machine app)

| Service (package `avf.machine.v1`) | Purpose |
|-----------------------------------|---------|
| `MachineAuthService` | Activation, claim, refresh |
| `MachineActivationService` | Activation flows (see machine_activation.proto) |
| `MachineTokenService` | Token operations |
| `MachineBootstrapService` | Initial config |
| `MachineCatalogService` | Catalog snapshot/delta |
| `MachineInventoryService` | Stock levels |
| `MachineCommerceService` | Orders, PSP sessions, vend |
| `MachineSaleService` | Native sale API |
| `MachineCommandService` | Updates/diagnostics; legacy polling deprecated |
| `MachineTelemetryService` | Telemetry ingest |
| `MachineOfflineSyncService` | Offline bundles |
| `MachineMediaService` | Media |
| `MachineOperatorService` | Operator workflows |

**Internal query services** (`proto/avf/internal/v1/*`) are **not** exposed to vending apps in production; omit from public E2E unless testing privileged tooling in a controlled environment.

---

## Related

- **[`local-e2e.md`](local-e2e.md)** — existing Go/Postgres correctness suites
- **[`field-test-cases.md`](field-test-cases.md)** — pilot case IDs (FT-*)
- **[`e2e-local-test-guide.md`](e2e-local-test-guide.md)** — how to run future shell-driven E2E
