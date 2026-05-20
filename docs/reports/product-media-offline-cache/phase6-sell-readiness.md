# Phase 6 — Product / machine sellable readiness

## Policy

| State | Primary media |
|-------|----------------|
| Product **inactive** / draft | Optional (`primary_image_id` may be null). |
| Product **active** | Required **and** must satisfy runtime readiness: active `product_images` + `product_media`, and when `media_asset_id` is set the asset must be `ready`. |
| **Sellable slot** (`max_quantity > 0`, `price_minor > 0`, product assigned) on **publish** (`PublishAsCurrent` or enterprise planogram validate/publish) | Same readiness as active sell line — enforced server-side. |

## Implementation

- **SQL**: `RuntimeProductPrimaryMediaReady` (`db/queries/runtime_catalog.sql`) — canonical readiness predicate aligned with `RuntimeListProductImagesForProducts`.
- **Validation**: `internal/app/sellreadiness` — `ValidateSellableSlotProductsPrimaryMedia`, `Compute`, `SellableSlotProductIDs`, `PrimaryMediaReadyMap`.
- **Planogram**: `validatePublishSnapshot` calls sell-readiness validation after structural checks.
- **REST slot publish**: `applySlotConfigSaveTx` validates when `publishAsCurrent` is true (draft-only saves unchanged).
- **Catalog admin**: `CreateProduct` / `UpdateProduct` require `RuntimeProductPrimaryMediaReady` when `active=true` (stricter than “primary_image_id non-null” alone).
- **Sale catalog projection**: availability for **active** lines requires primary media readiness; `missing_primary_media` appears in `unavailableReason` even when `include_images=false`.
- **Machine bootstrap (gRPC)**: `BootstrapCatalogProduct.primary_media_ready`; `RuntimeHints.sell_readiness` (`catalog_synced`, `media_synced`, `inventory_synced`, `ready_for_sale`, `readiness_issues`).
- **HTTP sale-catalog**: optional `readiness` object with camelCase fields mirroring JSON API style.

## Readiness semantics (server-side)

- **catalog_synced**: Published planogram version on `machines` matches `machine_current_snapshot.last_acknowledged_planogram_version_id`, and when a latest `machine_configs.config_revision` exists, `last_acknowledged_config_revision` must be ≥ that revision.
- **media_synced**: Every **sellable** current cabinet slot has a product whose primary media passes `RuntimeProductPrimaryMediaReady`.
- **inventory_synced**: Every sellable slot with a non-nil legacy `slot_index` has a matching legacy inventory projection row (quantity may be zero).
- **ready_for_sale**: All three flags true, machine status **active**, and a published planogram pointer exists.

Device-local cache state is **not** modeled in Postgres; clients should treat MQTT/media ACK paths as authoritative for “cached on disk”.

## Tests

- `internal/e2e/correctness/sell_readiness_policy_integration_test.go` (`TestP06_SellReadiness_*`).
- `go test ./...` (passes with `TEST_DATABASE_URL` for correctness tests).
