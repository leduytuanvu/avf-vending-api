# E2E flow coverage matrix (foundation)

This document maps **business flows** to **REST (Web Admin / Postman)**, **gRPC (Vending Machine App)**, and **MQTT** contracts. It aligns with **[`field-test-cases.md`](field-test-cases.md)** (field pilot), **[`docs/api/mqtt-contract.md`](../api/mqtt-contract.md)** (topics), **[`docs/swagger/swagger.json`](../swagger/swagger.json)** (admin + machine REST), and **`proto/avf/machine/v1/*.proto`**.

**Automation status:** Web Admin **setup** (**WA-SETUP-01**, `scenarios/01_web_admin_setup.sh`) and **business flows** (Phase 4: `10_*`–`13_*`, `./tests/e2e/run-web-admin-flows.sh --full`) cover much of **WA-002–WA-009** plus read-only **finance / reconciliation / audit / artifacts / sale-catalog / orders** where the API exposes routes. See **`reports/wa-module-results.jsonl`** and **`reports/summary.md`** (grouped by module) for last run. Remaining matrix rows are **`planned`** or manual.

**Machine gRPC (Phase 6):** **`./tests/e2e/run-grpc-local.sh`** exercises production-path **`avf.machine.v1`** RPCs via **grpcurl** (`scenarios/20_grpc_*.sh` … **`24_grpc_*.sh`**). Uses **`GRPC_PROTO_ROOT`** (defaults to repo **`proto/`**) when **`GRPC_USE_REFLECTION`≠true**, or **server reflection** when enabled. Results: **`reports/grpc-contract-results.jsonl`**, **`reports/grpc-contract-summary.md`** (pass / fail / skip per method).

**Vending machine app (field):** Production clients speak **gRPC** and **MQTT** (see matrix VM-001…VM-013, PF-*). **`./tests/e2e/run-vending-app-flows.sh --rest-equivalent`** runs **machine-scoped REST** that approximates the same journeys for **local QA / lab** only (`scenarios/02_*_rest.sh` … `08_*_rest.sh`). It is **not** the primary production protocol; **Web Admin REST** (`/v1/admin/*`) is also **not** what the field vending app calls directly.

**Runner modes:** `--setup-only` (default, same as orchestrated `run-all-local` child), `--full` (setup then Phase 4). **`--reuse-data`** works for **`--full`** when `test-data.json` already holds `organizationId`, `machineId`, `productId`, etc.

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
| WA-002 | Admin login & session | web-admin | REST | `POST /v1/auth/login`, `GET /v1/auth/me` | (import from OpenAPI) | Admin user, password | JWT; `me` reflects roles | **scripted** (login or `ADMIN_TOKEN`) | write-local-only | Logout / expiry |
| WA-003 | Org & site scaffolding | web-admin | REST | `GET/POST` org/sites per OpenAPI under `/v1/admin/organizations/...` | (import from OpenAPI) | Org admin, org name | Org + site IDs | **scripted** (reuse org; create site) | write-local-only | Delete test org or reset DB scratch |
| WA-004 | Catalog: categories | web-admin | REST | `/v1/admin/categories`, `/v1/admin/categories/{categoryId}` | (import from OpenAPI) | Admin JWT | CRUD per policy | **scripted** | write-local-only | Delete created categories |
| WA-005 | Catalog: brands & products | web-admin | REST | `/v1/admin/brands`, `/v1/admin/products` (per swagger) | (import from OpenAPI) | Admin JWT, SKU | Product visible in list/detail | **scripted** (brand/tag best-effort) | write-local-only | Archive/delete products |
| WA-006 | Machine registration & lifecycle | web-admin | REST | `/v1/admin/machines`, `/v1/admin/machines/{machineId}`, enable/disable/retire | (import from OpenAPI) | Org, site, serial | Machine row + status | **scripted** (create draft machine) | write-local-only | Retire + purge or reset |
| WA-007 | Activation codes (per-machine & org) | web-admin | REST | `/v1/admin/machines/{machineId}/activation-codes`, `/v1/admin/organizations/{organizationId}/activation-codes` | (import from OpenAPI) | Machine, policy | Code issued, revocable | **scripted** (org path) | write-local-only | Revoke codes |
| WA-008 | Planogram draft & publish | web-admin | REST | `/v1/admin/machines/{machineId}/planograms/draft`, `/publish` | (import from OpenAPI) | Machine, slots, products | New `catalog_version` on machine | **partial** (needs existing org planogram + operator session) | write-local-only | Republish prior or reset |
| WA-009 | Slot & inventory admin views | web-admin | REST | `/v1/admin/machines/{machineId}/slots`, `/inventory`, `/inventory-events`, stock adjustments | (import from OpenAPI) | Machine, slot IDs | Quantities match actions | **partial** (slots GET + stock-adjust POST when publish OK) | write-local-only | Reverse adjustments in scratch |
| WA-010 | Remote commands (admin → machine) | web-admin | mixed | REST: `/v1/admin/commands`, `/v1/admin/organizations/{organizationId}/commands`; MQTT: see PF-004 | (import from OpenAPI) | Machine active, command type | Ledger row; optional MQTT delivery | planned | write-staging-only | Cancel command or complete ACK path |
| WA-011 | Commerce reconciliation (read paths) | web-admin | REST | `/v1/admin/organizations/{organizationId}/commerce/reconciliation` (per swagger) | (import from OpenAPI) | Org, date range, orders | Totals consistent | **scripted** (list in `10_reporting_audit_reconciliation.sh`) | read-only | None |
| WA-012 | Finance daily close | web-admin | REST | `/v1/admin/finance/daily-close` | (import from OpenAPI) | Org, period | Close record | **partial** (GET list in `10_*`; POST close not automated) | write-local-only | Void per runbook if supported |
| WA-013 | Media upload & complete | web-admin | REST | `/v1/admin/media/*`, `/v1/admin/media/assets/*` | (import from OpenAPI) | Asset bytes or presign flow | Media attached | planned | write-local-only | Delete media assets |
| WA-014 | Diagnostics bundle request | web-admin | mixed | REST: `/v1/admin/machines/{machineId}/diagnostics/requests`; gRPC: `MachineCommandService.ReportDiagnosticBundleResult` | (import from OpenAPI) | Machine JWT + admin request | Bundle stored / linked | planned | write-local-only | TTL expiry |
| WA-015 | Audit & anomaly triage | web-admin | REST | `/v1/admin/audit/events`, org anomalies routes | (import from OpenAPI) | Events in window | Resolve/ignore flows | **partial** (audit events list in `10_*`) | read-only list | Mark resolved |
| WA-016 | Operator REST (kiosk helper) | support | REST | `/v1/machines/{machineId}/operator-sessions/*`, `sale-catalog`, `shadow` | (import from OpenAPI) | Machine/session tokens per policy | Session timeline | **partial** (sale-catalog GET in `12_*`) | write-local-only | Logout session |
| VM-001 | Claim activation & machine JWT | vending-app | gRPC | `MachineAuthService.ClaimActivation`; `MachineTokenService.RefreshMachineToken` | — | Valid activation code or existing token | `machine_token` + claims | **scripted** (**GRPC-20**, `20_grpc_machine_auth.sh`) | write-local-only | Revoke credential admin-side |
| VM-001b | Claim activation (alias service) | vending-app | gRPC | `MachineActivationService.ClaimActivation` | — | Same as VM-001 | Same | **skip** in harness (duplicate path documented in **GRPC-20**) | write-local-only | Revoke credential admin-side |
| VM-002 | Bootstrap & config version | vending-app | gRPC | `MachineBootstrapService.GetBootstrap`, `AckConfigVersion` | — | Activated machine | Config + version | **scripted** (**GRPC-21**) | write-local-only | Replay-safe |
| VM-003 | Catalog snapshot & delta | vending-app | gRPC | `MachineCatalogService.GetCatalogSnapshot`, `GetCatalogDelta`, `AckCatalogVersion`, `GetMediaManifest` | — | Published planogram | `catalog_version`; basis match | **scripted** (**GRPC-21**) | read-only | None |
| VM-004 | Inventory (machine-scoped) | vending-app | gRPC | `MachineInventoryService.PushInventoryDelta`, `GetInventorySnapshot` | — | Slots with stock | Levels match admin | **scripted** (**GRPC-23**) | write-local-only | Admin stock adjust |
| VM-005 | Sale: PSP QR order path | vending-app | gRPC | `MachineCommerceService.CreateOrder`, `CreatePaymentSession`, `GetOrder`, `GetOrderStatus` | — | Product, slot, sandbox PSP | Order → paid → vend | **partial** (**GRPC-22** skips `CreatePaymentSession` in favor of cash) | write-local-only | Refund path WA or VM |
| VM-006 | Sale: MachineSaleService path | vending-app | gRPC | `MachineSaleService` RPCs (per commerce.proto) | — | Same as VM-005 | Terminal vend success | planned | write-local-only | Cancel/refund |
| VM-007 | Cash tender | vending-app | gRPC | `MachineCommerceService.ConfirmCashPayment`; `MachineSaleService.ConfirmCashReceived` | — | Cash-enabled SKU | Order completed without PSP | **scripted** (**GRPC-22**) | write-local-only | Ledger cleanup scratch |
| VM-008 | Vend success idempotency | vending-app | gRPC | `StartVend`, `ConfirmVendSuccess` | — | Paid order | Single inventory decrement | **scripted** (**GRPC-22**) | write-local-only | See troubleshooting |
| VM-009 | Vend failure & refund | vending-app | mixed | `MachineCommerceService.ReportVendFailure` | — | Paid order | Failed vend state | **scripted** (**GRPC-22** failure probe) | write-local-only | Financial reconciliation scratch |
| VM-010 | Telemetry batch & critical | vending-app | gRPC | `MachineTelemetryService.PushTelemetryBatch`, `PushCriticalEvent`, `ReconcileEvents` | — | Idempotency keys | Accepted; duplicates surfaced | **scripted** (**GRPC-23**) | write-local-only | N/A |
| VM-011 | Offline replay & reconcile | vending-app | gRPC | `MachineOfflineSyncService.PushOfflineEvents`, `GetSyncCursor` | — | Client ids | Cursor + replay | **scripted** (**GRPC-23**) | write-local-only | Fresh machine or reset |
| VM-012 | Media fetch (machine) | vending-app | gRPC | `MachineMediaService.GetMediaManifest`, `GetMediaDelta`, `AckMediaVersion` | — | Catalog manifest URLs | Metadata + fingerprints | **scripted** (**GRPC-21**) | read-only | Cache clear on device |
| VM-013 | Operator gRPC | vending-app | gRPC | `MachineOperatorService` (per operator_grpc.proto) | — | Machine JWT | Operator actions logged | planned | write-local-only | End session |
| VM-REST-02 | Activation claim + bootstrap (QA) | vending-app | REST | `POST /v1/setup/activation-codes/claim`, `GET /v1/setup/machines/{id}/bootstrap` | (import from OpenAPI) | Activation code or reused `machineToken` | JWT + bootstrap hints | **scripted** | write-local-only | Revoke test codes |
| VM-REST-03 | Sale catalog + media URLs (QA) | vending-app | REST | `GET /v1/machines/{id}/sale-catalog?include_images=true` | — | Published planogram, slot A1 | Product + optional image fields | **scripted** | read-only | None |
| VM-REST-04 | Cash sale success (QA) | vending-app | REST | `POST /v1/commerce/cash-checkout`, `.../vend/start`, `.../vend/success`, `GET .../orders/{id}` | — | `machineId`, slot | Paid + vend OK; optional catalog qty | **scripted** | write-local-only | Refund / adjust scratch |
| VM-REST-06 | Vend failure + refund (QA) | vending-app | REST | vend failure + `POST .../refunds` when exposed | — | Paid order | Failed vend + refund/cancel state | **scripted** | write-local-only | Reconcile scratch |
| VM-REST-08 | Idempotent order create (QA) | vending-app | REST | `POST /v1/commerce/orders` + `Idempotency-Key` | — | Machine JWT | Duplicate → same order | **scripted** | write-local-only | Cancel test orders |
| GRPC-20 | gRPC auth & refresh | vending-app | gRPC | `20_grpc_machine_auth.sh` | — | `E2E_ACTIVATION_CODE` or secrets | JWT + refresh probes | **scripted** | write-local-only | Revoke / rotate |
| GRPC-21 | gRPC bootstrap + catalog + media | vending-app | gRPC | `21_grpc_bootstrap_catalog_media.sh` | — | Machine JWT | Snapshots + acks | **scripted** | write-local-only | Replay-safe |
| GRPC-22 | gRPC commerce (cash + vend) | vending-app | gRPC | `22_grpc_commerce_cash_sale.sh` | — | JWT, `productId`, writes | Orders + vend + failure | **scripted** | write-local-only | Ledger cleanup |
| GRPC-23 | gRPC inventory + telemetry + offline | vending-app | gRPC | `23_grpc_inventory_telemetry_offline.sh` | — | Machine JWT | Deltas + sync cursor | **scripted** | write-local-only | Scratch reset |
| GRPC-24 | gRPC command / OTA / diagnostics | vending-app | gRPC | `24_grpc_command_update_status.sh` | — | Machine JWT | Update + bundle ids | **scripted** | write-local-only | Cancel campaigns |
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
| `MachineAuthService` | Activation, claim, refresh (**GRPC-20**) |
| `MachineActivationService` | Public claim duplicate (**GRPC-20** records skip — use `MachineAuthService`) |
| `MachineTokenService` | Refresh without bearer (**GRPC-20**) |
| `MachineBootstrapService` | Initial config (**GRPC-21**) |
| `MachineCatalogService` | Catalog snapshot/delta (**GRPC-21**) |
| `MachineInventoryService` | Stock levels (**GRPC-23**) |
| `MachineCommerceService` | Orders, PSP sessions, vend (**GRPC-22**) |
| `MachineSaleService` | Native sale API |
| `MachineCommandService` | Updates + diagnostics; legacy polling deprecated (**GRPC-24**) |
| `MachineTelemetryService` | Telemetry ingest (**GRPC-23**) |
| `MachineOfflineSyncService` | Offline bundles (**GRPC-23**) |
| `MachineMediaService` | Media metadata (**GRPC-21**) |
| `MachineOperatorService` | Operator workflows |

**Internal query services** (`proto/avf/internal/v1/*`) are **not** exposed to vending apps in production; omit from public E2E unless testing privileged tooling in a controlled environment.

---

## Related

- **[`local-e2e.md`](local-e2e.md)** — existing Go/Postgres correctness suites
- **[`field-test-cases.md`](field-test-cases.md)** — pilot case IDs (FT-*)
- **[`e2e-local-test-guide.md`](e2e-local-test-guide.md)** — how to run future shell-driven E2E
