# Media sync (kiosk / machine runtime)

## Principles

- **Catalog and manifests over gRPC carry metadata only**: presigned or CDN **HTTPS URLs**, checksums, etags, sizes, MIME, dimensions, optional **`expires_at`** on signed URLs, **`media_version`**, and **`updated_at`**. **No image bytes** are sent on machine gRPC; `catalog.proto` documents this invariant.
- **Per-rendition manifest**: `ProductMediaRef.media_variants` lists **original**, **thumb**, and **display** rows when the runtime projection has distinct URLs or object keys. Each variant includes **`media_asset_id`**, **`checksum_sha256`**, **`etag`**, **`size_bytes`**, **`width`/`height`** when known, **`content_type`**, **`media_version`**, and **`updated_at`**. Treat the **hash + media_asset_id + rendition kind + media_version** as the durable cache key; **HTTPS URLs (especially presigned) are not source of truth**.
- **Downloads use HTTPS**: the kiosk fetches each variant URL from object storage or CDN. Treat URLs as **short-lived** when the API issues presigned GET links; refresh via **`GetCatalogSnapshot`**, **`GetMediaManifest`**, or **`GetMediaDelta`** before expiry when online.
- **`GetCatalogSnapshot` parity**: when the binary registers `MediaStore` + `MediaPresignTTL`, the server runs the same **`RefreshPresignedProductMediaURLs`** pass as **`GetMediaManifest`** so sale-catalog responses do not reuse expired presigned URLs stored from an earlier bind time.
- **Integrity**: **`checksum_sha256`** per variant reflects the bound projection (display row uses catalog/asset hash; distinct object keys for thumb/original use deterministic variant hashes derived from keys + asset SHA when available). **`media_version`** bumps on **`product_image` / `product_media`** projection changes. **`etag`** aligns with SHA-based weak tags where available.
- **Runtime SQL projection**: `RuntimeListProductImagesForProducts` returns only **`product_images.status = 'active'`** and **`product_media.status = 'active'`** (not `processing`, `failed`, or `archived`) and requires **`media_assets.status = 'ready'`** when **`media_asset_id`** is set — inactive or broken media does not appear in machine catalog rows.
- **Change detection (P1 runtime catalog)**:
  - **`catalog_version`** on `CatalogSnapshot` / HTTP JSON is **`salecatalog.RuntimeSaleCatalogFingerprint`**: a composite digest (**`runtime_sale_catalog_v6`**) over assortment (**`setupapp.CatalogFingerprint`**), slot pricebook (**`PricingFingerprint`**), published planogram slot map (**`PlanogramFingerprint`**), promotion material (**`PromotionsSnapshotFingerprint`**), **`MediaFingerprint`** (**`media_catalog_v3`**, per-line **variant storage keys + hashes + etags**, not URLs), **`InventorySnapshotFingerprint`** (per-line stock / price envelope / unavailable reasons), machine shadow **`config_version`**, **`currency`**, and the **`include_unavailable`** / **`include_images`** projection flags used to build `items`. Clients should persist this string as **`catalog_fingerprint`** in local DB caches.
  - Separately from that composite line, **`media_fingerprint`** on manifest/delta repeats the media-only **`MediaFingerprint`** for cheap image cache sync without re-downloading the full catalog protobuf.
  - **`generated_at`** is the snapshot UTC timestamp embedded in protobuf; unary responses also carry **`MachineResponseMeta.server_time`** as authoritative RPC wall-clock.
- **Promotions**: promotion-aware pricing participates in **`catalog_version`** via **`PromotionsSnapshotFingerprint`** (see pricing runtime docs).
- Rotate presigned URLs without changing underlying **storage keys** or **integrity metadata** → **`media_fingerprint`** stays stable (URLs are not hashed).

## Related surfaces

- HTTP: `GET /v1/machines/{machineId}/sale-catalog` (same projection as gRPC snapshot; JSON).
- gRPC: `avf.machine.v1.MachineCatalogService` (`GetCatalogSnapshot`, `GetCatalogDelta`, `GetMediaManifest`).
- Auth: **Machine JWT** only on gRPC catalog RPCs; tenant/machine scope is enforced from the token and optional `machine_id` field must match the principal.

## Operational notes

- Prefer **conditional requests** (`if_none_match_config_version` on snapshot) to skip body work when shadow **`config_revision`** is unchanged — but **inventory can move without bumping shadow config**, so hot-path vending SHOULD still poll **`GetCatalogSnapshot`** or **`GetCatalogDelta`** on reconnect (see kiosk flow doc below).
- **Catalog paging**: protobuf snapshots are **single message** payloads; **`GetMediaManifest`** enforces **`CAPACITY_MAX_MEDIA_MANIFEST_ENTRIES`** (default 5000 SKUs/chunk gate) — returns **`ResourceExhausted`** if the bounded manifest would exceed the cap so operators must split catalogs or widen the env within validated bounds **`[64,100000]`**. There is **no page token on `GetCatalogSnapshot`**; use **`GetCatalogDelta`** with `basis_catalog_version == $last_RuntimeSaleCatalogFingerprint` for incremental freshness (server builds **`GetCatalogDelta`** with **`include_unavailable=true`**, **`include_images=true`**).
- When enabling artifacts/object storage, ensure **thumb.webp** / **display.webp** variants exist under deterministic keys (`internal/platform/objectstore`) and URLs resolve after outage recovery.
