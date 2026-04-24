# Runtime sale catalog — implementation handoff

> **Stale / historical:** The sale-catalog route is **implemented** and documented in OpenAPI. Keep this file for design rationale only; do not treat it as a “to build” checklist.

**Route:** `GET /v1/machines/{machineId}/sale-catalog`

## Auth and tenant isolation

- Mount inside the existing Bearer-authenticated `/v1` group (same as bootstrap).
- **Shipped:** **`RequireMachineTenantAccess`** (`internal/httpserver/machine_tenant_middleware.go`) — DB-bound `machines.organization_id` + machine allow list; same pattern as bootstrap.
- Callers: machine JWT (`machine_ids` contains `machineId`) or org/technician JWT with org scope matching the machine row.

## Query parameters

| Param | Type | Default | Notes |
|-------|------|---------|--------|
| `if_none_match_config_version` | int | omit | Client’s last `configVersion` |
| `include_unavailable` | bool | `false` | If `false`, omit rows that fail availability rules |
| `include_images` | bool | `true` | If `false`, omit `image` object entirely |

Parse with `strconv` / `strings` consistent with other list endpoints.

## Config version and conditional response

- **`configVersion`**: use latest `machine_configs.config_revision` for this `machine_id` (max), or **`1`** if no row exists.
- **If** `if_none_match_config_version` is sent **and** equals current `configVersion`:
  - **Preferred:** respond **`304 Not Modified`** with **`ETag: W/"cv-{n}"`** and empty body (if Chi/handler pattern allows without breaking clients).
  - **Fallback (simpler):** respond **`200`** with body:
    ```json
    {
      "machineId": "...",
      "organizationId": "...",
      "siteId": "...",
      "configVersion": 7,
      "currency": "VND",
      "generatedAt": "...",
      "unchanged": true,
      "items": []
    }
    ```
- Always set **`Cache-Control: private, max-age=0, must-revalidate`** (or match existing API pattern) and **`ETag`** on full responses for intermediaries.

Add **sqlc** query in `db/queries/inventory_admin.sql` (or `machine_runtime.sql`):

```sql
-- name: SaleCatalogLatestConfigRevision :one
SELECT coalesce(max(config_revision), 0)::int AS config_revision
FROM machine_configs
WHERE machine_id = $1;
```

Use `coalesce(...,0)` then expose as `configVersion` with minimum **1** in Go if you want monotonic positive ints (`max(1, revision)`).

## Data sources (no commerce state machine)

Reuse inventory read paths already used for admin slot view:

1. **Slots + stock + price overlay:** [`inventoryadmin.ListSlotInventoryView`](../../internal/app/inventoryadmin/slot_inventory_view.go) (`InventoryAdminListMachineSlots` + `InventoryAdminListCurrentMachineSlotConfigsByMachine` + org currency).
2. **Machine org/site:** `GetMachineByID` or `InventoryAdminGetMachineOrg` + machine row for `site_id`.
3. **Product `active` + `attrs` (shortName):** batch-load `products` for distinct `product_id`s from slot rows (new sqlc query `CatalogGetProductsByIDs` or filter in app via existing catalog queries).
4. **Primary image:** `product_images` where `is_primary` (or first by `sort_order`) — columns today: `storage_key`, `cdn_url`, `created_at`. **No `content_hash` in DB:** expose stable **`contentHash`** as hex(`sha256(storage_key + "\n" + product_image.id::text)`) or similar **without** leaking bucket secrets. **`thumbUrl` / `displayUrl`:** use `cdn_url` when non-empty; if only `storage_key` exists, either omit URLs or plug into existing **artifact presign** helper if the repo has one (do not return raw internal keys as “public” URLs).

## Cabinet-only machines (no `machine_slot_state` rows)

`ListSlotInventoryView` only iterates **legacy** `machine_slot_state` rows. If a machine has **only** `machine_slot_configs` (current) and zero legacy state, the list can be empty.

**Required P0 behavior:** merge in **unmatched** `InventoryAdminListCurrentMachineSlotConfigsByMachine` rows as sale lines with **`availableQuantity: 0`** (and appropriate `unavailableReason` / `isAvailable: false`) so the kiosk still sees the planogram surface.

Algorithm sketch:

1. `items := ListSlotInventoryView(ctx, machineID)`
2. Build set of matched `(cabinet_code, slot_code)` from `items`
3. For each current config row not matched, append synthetic line: `slotIndex` from `slot_index` or hash order, `slotCode`, `cabinetCode`, product from config, stock 0

## Availability and `unavailableReason`

For each candidate line (merged):

| Rule | `isAvailable` | `unavailableReason` |
|------|---------------|---------------------|
| No `product_id` | false | `planogram_inactive` or `slot_disabled` |
| Product exists and `products.active = false` | false | `product_inactive` |
| `price_minor <= 0` | false | `price_missing` |
| Slot/config inactive: `effective_to` in past (if present on config) | false | `planogram_inactive` |
| `current_stock <= 0` | false | `out_of_stock` |
| Else | true | null |

Map inventory **`Status`** string (`out_of_stock` from [`slotStatus`](../../internal/app/inventoryadmin/slot_inventory_view.go)) consistently.

**`include_unavailable=false`:** filter out `!isAvailable` **after** computing reasons.

## Response JSON shape (camelCase in HTTP)

Align field names with user spec:

- Top-level: `machineId`, `organizationId`, `siteId`, `configVersion`, `currency`, `generatedAt` (UTC RFC3339Nano), `items`, optional `unchanged`
- Item: `slotIndex`, `slotCode`, `cabinetCode`, `productId`, `sku`, `name`, `shortName`, `priceMinor`, `availableQuantity`, `maxQuantity`, `isAvailable`, `unavailableReason`, `image`, `sortOrder` (from assortment or cabinet sort + slot order)

**`shortName`:** `attrs->>'shortName'` else truncate `name` (e.g. rune-safe 32 chars).

**`sortOrder`:** use `cabinet_index * 1000 + slot_index` or assortment `SortOrder` when product in primary assortment; else derive from config iteration order.

## Implementation layout

| Layer | Suggested path |
|-------|------------------|
| Domain DTOs | `internal/app/catalogruntime/` or `internal/app/salecatalog/` |
| Service | `Service.BuildSaleCatalog(ctx, machineID, opts)` |
| HTTP | `internal/httpserver/sale_catalog_http.go` — `mountSaleCatalogRoutes(r, app)` |
| Wire | [`server.go`](../../internal/httpserver/server.go) next to bootstrap/telemetry (GET, same auth stack, **no** `writeRL`) |

Add **`SaleCatalog *salecatalog.Service`** (or method on `InventoryAdmin`) to [`HTTPApplication`](../../internal/app/api/application.go) and construct in `NewHTTPApplication`.

## OpenAPI

1. Add `DocOpSaleCatalog` (or similar) in [`swagger_operations.go`](../../internal/httpserver/swagger_operations.go): `@Router`, `@Security BearerAuth`, query params, success + 304 if documented, error examples.
2. Append `("get", "/v1/machines/{machineId}/sale-catalog")` to **`REQUIRED_OPERATIONS`** in [`tools/build_openapi.py`](../../tools/build_openapi.py).
3. `make swagger` && `make swagger-check`.

## Tests

| Case | Notes |
|------|------|
| Own machine token | JWT with `machine_ids=[machineId]` |
| Other machine token | 403 |
| Other tenant user | 403 (or 404 if using tenant middleware + hidden existence) |
| Active product + stock | `isAvailable` true when price > 0 |
| Default `include_unavailable=false` | hides OOS lines |
| `include_unavailable=true` | `unavailableReason` set |
| `include_images=false` | no `image` key or `image: null` per spec |
| `if_none_match_config_version` | 304 or 200+`unchanged` |
| No internal secrets in JSON | no `storage_key` in response |

Use **`TEST_DATABASE_URL`** integration test with fixtures (see [`machine_setup_repositories_test.go`](../../internal/modules/postgres/machine_setup_repositories_test.go)) where feasible.

## Acceptance

```bash
make sqlc   # if new queries
make swagger && make swagger-check
go test ./...
```

---

**Blocked in Plan mode:** implement Go/SQL/OpenAPI in a follow-up session with Agent mode enabled.
