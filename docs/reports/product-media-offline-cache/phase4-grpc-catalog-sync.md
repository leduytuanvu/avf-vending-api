# Phase 4 — gRPC catalog sync with media manifest + deltas

## Proto (`proto/avf/machine/v1/catalog.proto`)

- New RPC **`MachineCatalogService.SyncCatalogBundle`**: single call returns machine-scoped **`changed_products`** (`CatalogSlotItem`) and **`changed_media_assets`** (`MediaAsset`), plus optional tombstones when **`basis_product_ids`** / **`basis_media_asset_keys`** are supplied.
- **`MediaAsset`**: `id`, `product_id`, `role` (`primary`), `variant` (`original` \| `thumb` \| `display`), `mime_type`, `width`, `height`, `sha256` (hex without `sha256:` prefix), `size_bytes`, `version`, `download_url`. **No binary fields.**
- Request cursors: **`current_catalog_version`**, **`current_media_manifest_version`** (empty ⇒ treat as never synced ⇒ full payloads).
- Response cursors: **`catalog_version`**, **`media_manifest_version`**.

## Version semantics

- **`catalog_version`** on this RPC is **`CatalogSyncCatalogVersion`** (`runtime_sale_catalog_sync_v1`): assortment, pricing, planogram, promotions, inventory, config, currency, and snapshot flags — **excluding** embedded media projection.
- **`media_manifest_version`** matches **`MediaFingerprint`** / `GetMediaManifest.media_fingerprint` (same `include_unavailable` / `include_images` as the request).
- Legacy **`CatalogSnapshot.catalog_version`** from **`GetCatalogSnapshot`** remains the **composite** **`RuntimeSaleCatalogFingerprint`** (still includes `med:`). Clients must not mix the two `catalog_version` meanings across RPCs without reading comments.

## Server (`internal/grpcserver/machine_catalog_grpc.go`)

- Builds **`salecatalog.Snapshot`** for the authenticated machine (same planogram-scoped path as existing catalog RPCs).
- Refreshes presigned URLs when **`include_images`** is true and object store + TTL are configured.
- Independent deltas: catalog mismatch ⇒ fill **`changed_products`** (+ **`removed_product_ids`** from basis); media mismatch ⇒ fill **`changed_media_assets`** (+ **`removed_media_asset_ids`** from basis).
- **`changed_media_assets`** only includes **ready** projection rows (skips deleted / tombstone primaries); omits variants with no **`download_url`**.
- Enforces **`Capacity.MaxMediaManifestEntries`** against emitted **`MediaAsset`** count (parity with manifest RPC).

## Sale catalog readiness (`internal/app/salecatalog/service.go`)

- When **`IncludeImages`** is true and a line has **no** primary image row or a **tombstone** projection, the line is marked **`IsAvailable=false`** with **`missing_primary_media`** in **`UnavailableReason`** (UI/cache should treat as not sellable).

## grpcurl example

From repo root (`proto` import path):

```bash
grpcurl -plaintext \
  -import-path proto -proto avf/machine/v1/catalog.proto \
  -H "authorization: Bearer ${MACHINE_TOKEN}" \
  -d '{"current_catalog_version":"","current_media_manifest_version":"","include_unavailable":false}' \
  "${GRPC_ADDR:-127.0.0.1:9090}" \
  avf.machine.v1.MachineCatalogService/SyncCatalogBundle
```

Second sync (no payload) after persisting both version strings from the previous response:

```bash
grpcurl -plaintext \
  -import-path proto -proto avf/machine/v1/catalog.proto \
  -H "authorization: Bearer ${MACHINE_TOKEN}" \
  -d "{\"current_catalog_version\":\"<prior_catalog_version>\",\"current_media_manifest_version\":\"<prior_media_manifest_version>\",\"include_unavailable\":false}" \
  "${GRPC_ADDR:-127.0.0.1:9090}" \
  avf.machine.v1.MachineCatalogService/SyncCatalogBundle
```

With reflection enabled, omit `-import-path` / `-proto`.

## Tests

- `internal/grpcserver/machine_catalog_grpc_test.go` — first sync, second sync empty delta, media-only delta, missing-media readiness, basis removals.
- `internal/app/salecatalog/fingerprint_test.go` — `CatalogSyncCatalogVersion` stable when only media checksum changes.
- `internal/app/salecatalog/service_test.go` — `applyMissingPrimaryMediaPenalty`.

## Known limitations

- **`basis_*`** tombstones apply only when the corresponding half (catalog or media) is returning a delta (`needCat` / `needMedia`).
- Setting **`include_images=false`** avoids loading image rows in **`salecatalog`**; media deltas and **`MediaAsset`** payloads will be empty — intended for bandwidth-poor polls that only need product/planogram deltas.
