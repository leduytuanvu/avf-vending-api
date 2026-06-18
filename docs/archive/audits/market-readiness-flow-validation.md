# Market readiness — flow validation

Validated against code, E2E manifests (`tests/e2e/production/e2e-manifest*.yaml`), and existing integration tests. Status: **code path exists** unless marked **gap** or **requires live env**.

## Flow A — Admin product → machine sellable

| Step | Canonical surface | Evidence | Status |
|------|-------------------|----------|--------|
| 1 Admin login | `POST /v1/auth/login` | `e2e-manifest` REST-AUTH-001 | documented |
| 2 Category CRUD | `/v1/admin/categories` | REST-CATALOG-001 | documented |
| 3 Brand CRUD | `/v1/admin/brands` | REST-CATALOG-002 | documented |
| 4 Tag CRUD | `/v1/admin/tags` | REST-CATALOG-003 | documented |
| 5 Media upload | `/v1/admin/media/uploads/init`, complete, `/media/product-images` | `admin_media_http.go` | documented |
| 6 Create product | `POST /v1/admin/products` | REST-CATALOG-004 | documented |
| 7 Bind media | `POST /v1/admin/products/{id}/media` (canonical); `/image` legacy | aliases mounted | documented |
| 8 Machine + activation | `POST /v1/admin/machines`, activation codes | REST-MACHINE-* | documented |
| 9 Planogram/slots | topology/planogram/stock admin APIs | WA-* e2e scripts | partial in E2E matrix |
| 10 Publish revision | planogram publish/apply flows | operator/planogram handlers | **requires live E2E** |
| 11 Machine catalog gRPC | `MachineCatalogService.SyncCatalogBundle` | `machine_catalog_grpc_test.go` | **tested** |
| 12 Media manifest gRPC | `GetMediaManifest` | same test file | **tested** |
| 13 Sellable gates | active + media + slot + stock + revision | `SyncCatalogBundle_MissingMediaNotSellable` | **tested** |
| 14 MQTT not catalog transport | docs + architecture | `mqtt-contract.md` | documented |

**Gap:** Full admin→planogram→publish chain is **manifest/Postman** covered; automated integration test for entire chain is split across shell E2E and gRPC unit tests — run `tests/e2e/run-web-admin-flows.sh` locally with DB.

## Flow B — First install vending app

| Step | Surface | Evidence | Status |
|------|---------|----------|--------|
| 1 Fresh app | client responsibility | — | N/A in API |
| 2 Activation code | admin `POST .../activation-codes` | manifest | documented |
| 3 Activate | `MachineActivationService` gRPC | proto + grpc tests | documented |
| 4 Token | `MachineTokenService` / `MachineAuthService` | grpcserver | documented |
| 5 Bootstrap | `MachineBootstrapService`, legacy `GET /v1/setup/.../bootstrap` | legacy gated in prod | canonical gRPC |
| 6 Sync profile/config/catalog/inventory/media | gRPC services | bootstrap + catalog + inventory tests | **partial tests** |
| 7 MQTT enterprise | `MQTT_TOPIC_LAYOUT=enterprise` | `topics.go` | documented |
| 8 Telemetry | `MachineTelemetryService`, MQTT telemetry channels | router tests | **tested** |
| 9 Command + ACK | `MachineCommandService`, MQTT `commands` + `commands/ack` | mqtt + command tests | **tested** |
| 10 Offline | `MachineOfflineSyncService` | grpc + replay tests | **tested** |

**Gap:** End-to-end physical device install requires **staging machine credentials** — not run in this audit without tokens.

## Flow C — Sale / vend

| Step | Surface | Evidence | Status |
|------|---------|----------|--------|
| Checkout | `MachineCommerceService` gRPC (canonical) | proto | documented |
| Cash/online | commerce + payment modules | reconciler worker | documented |
| Vend result | commerce finalize + inventory | internal/e2e correctness | **requires TEST_DATABASE_URL** |
| Idempotency | order/vend keys | commerce webhook + grpc tests | **unit tests exist** |
| MQTT vend event | `events/vend` | `router_test.go` | **tested** |
| Outbox | worker outbox | `app/background` tests | **tested** |

**Gap:** Full vend loop needs `make test-e2e-local` or production canary with explicit approval.

## Flow D — Payment webhook

| Step | Evidence | Status |
|------|----------|--------|
| HMAC verify | `commerce_webhook_hmac_test.go` | **tested** |
| Production unsigned blocked | `commerce_webhook_public_policy_test.go` | **tested** |
| Idempotency | `commerce_webhook_public_test.go` | **tested** |

## Flow E — Inventory / operator

| Step | Evidence | Status |
|------|----------|--------|
| Operator session | REST operator + admin session APIs | manifest REST-OP-* | documented |
| Fill/stock adjust | admin inventory HTTP | e2e inventory scripts | **requires live E2E** |
| Machine sync stock | gRPC inventory sync | inventory grpc tests | partial |

## Summary

| Flow | Code readiness | Automated test | Live proof |
|------|----------------|----------------|------------|
| A Product→machine | Yes | Partial (gRPC catalog tests) | Needs E2E/prod token |
| B First install | Yes | Partial | Needs machine credentials |
| C Vend | Yes | Partial (DB e2e) | Needs controlled canary |
| D Webhook | Yes | Yes (unit) | Needs provider sandbox |
| E Operator | Yes | Partial | Needs E2E |
