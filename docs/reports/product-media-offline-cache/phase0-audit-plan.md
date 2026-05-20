# Phase 0 — Product media & offline cache (audit & implementation plan)

**Scope:** Document current state vs business requirements for production-grade product media: metadata + manifest for vending sync, local cache keyed by stable identifiers (SHA-256 / versions / object keys), no image bytes over MQTT or gRPC, no blobs in PostgreSQL.  
**Non-goals (this phase):** No runtime code changes beyond this report.

---

## 1. Current schema summary

Authoritative DDL snapshot: `db/schema/01_platform.sql`.

| Table | Role |
|-------|------|
| **`products`** | Sellable SKU metadata; `primary_image_id` FK (deferrable) to `(product_images.product_id, product_images.id)`; `active`, category/brand FKs, attrs JSON. |
| **`product_images`** | Per-product image rows: `storage_key`, optional CDN URLs, dimensions, `is_primary`, `media_asset_id` → `media_assets`, `media_version`, `status` (`active` / `archived`). Unique partial index enforces one primary per product when active. |
| **`product_media`** | Denormalized projection row **1:1 with `product_images`** (`id` + `product_id` FK to composite `(product_images)`). Carries URLs, object keys, `content_hash`, `media_version`, `status`. No binary payloads—metadata only. |
| **`media_assets`** | Canonical uploaded/processed assets: `original_object_key`, `thumb_object_key`, `display_object_key`, `sha256`, `object_version`, `etag`, `status`, optional `original_url` for provenance—not intended as kiosk identity. |
| **`tags`** / **`product_tags`** | Optional product labeling; `machine_tag_assignments` links tags to machines for rollout targeting. |
| **Planogram / runtime slot model** | **`planograms`** + **`slots`** (product placement on a planogram revision); **`machine_slot_state`** (per-machine quantities/prices per planogram slot); **`machine_slot_layouts`** / **`machine_slot_configs`** (machine topology ↔ slot codes and cabinet wiring—see schema around referenced `CREATE TABLE`). |

**Inventory-adjacent:** `inventory_events`, `inventory_count_sessions`, `inventory_anomalies` exist for operational inventory workflows; sale availability for vending projection is driven primarily by machine bootstrap + sale-catalog builders (not duplicated here).

---

## 2. Existing media support

### 2.1 Database & SQL projection

- Runtime query **`RuntimeListProductImagesForProducts`** (`db/queries/runtime_catalog.sql`) joins **`product_images` INNER JOIN `product_media`** with **`LEFT JOIN media_assets`**, exposing CDN URLs, content hash, **`asset_sha256`**, object keys (`original_`, `thumb_`, `display_`), `media_version`, etag/object_version from assets when bound and **`media_assets.status = 'ready'`**.
- Admin/catalog writes maintain **`products.primary_image_id`**, image rows, and projection consistency via `internal/app/catalogadmin/writes.go` (bind from artifact URLs or object store copy, bind from `media_assets`, archive/clear primary, patch metadata).

### 2.2 Sale catalog (machine-facing)

- **`internal/app/salecatalog/service.go`** builds per-line **`ImageMeta`** when `IncludeImages`: prefers **`pickDisplayImage`** (`internal/app/salecatalog/images.go`) honoring **`is_primary`**, then falls back to first row.
- **Content hash / etag:** `productImageContentHash` prefers **`media_assets.sha256`** (normalized `sha256:` prefix), else `content_hash`, else deterministic fallback from `storage_key`.
- **`internal/app/salecatalog/fingerprint.go`** — **`MediaFingerprint(Snapshot)`** fingerprints the **media projection** using **`media_id`**, **`media_version`**, per-variant **`storage_key`**, **`checksum_sha256`**, **`etag`** (explicitly **not** presigned URLs). **`RuntimeSaleCatalogFingerprint`** folds this into **`catalog_version`** (aligned with gRPC/HTTP responses).

### 2.3 HTTP (machine JWT)

- **`internal/httpserver/sale_catalog_http.go`** — `GET /v1/machines/{machineId}/sale-catalog` returns JSON **`catalogVersion`**, per-item **`image`** block: `thumbUrl`, `displayUrl`, `contentHash`, `etag`, optional `mediaId`, `objectVersion`, `mediaVersion`, dimensions; **`deleted: true`** when image marked deleted in projection.

### 2.4 gRPC (machine JWT)

- **`proto/avf/machine/v1/catalog.proto`** — `MachineCatalogService`: **`GetSaleCatalog` / `GetCatalogSnapshot`**, **`GetCatalogDelta`**, **`AckCatalogVersion`**, **`GetMediaManifest`**. Comments state media is URL + metadata only; **`ProductMediaVariant`** carries **`checksum_sha256`**, **`etag`**, **`media_version`**, **`url`** (may be short-lived signed URL).
- **`proto/avf/machine/v1/media.proto`** — **`MachineMediaService`**: **`GetMediaManifest`**, **`GetMediaDelta`**, **`AckMediaVersion`** — manifest rows only, no bytes.
- **`proto/avf/machine/v1/bootstrap.proto`** — **`GetBootstrap`** returns **`media_fingerprint`**; **`CheckForUpdates`** compares client-supplied fingerprints and sets **`media_changed`** plus **`server_media_fingerprint`**.
- **Implementation:** `internal/grpcserver/machine_catalog_grpc.go` maps snapshots to proto including **`primary_media`** / manifest entries; presigned URL refresh paths tie into **`salecatalog.RefreshPresignedProductMediaURLs`** (referenced from catalog server code). **`internal/grpcserver/machine_grpc_services.go`** wires bootstrap **`MediaFingerprint`** from **`setupapp.MediaFingerprint`**.

### 2.5 MQTT

- **`docs/api/mqtt-contract.md`** — Commands are JSON wire (`command_type`, `payload`, …); **`payload_sha256_hex`** logged for dispatch auditing. **No image binary** on MQTT topics—consistent with requirements.

### 2.6 Admin REST / OpenAPI / Postman

- **`internal/httpserver/openapi_types.go`** — **`V1AdminProduct`** exposes **`primaryImageId`**, **`imageUrl`** / **`displayUrl`** / **`thumbUrl`** (URLs are convenience, not identity). **`V1AdminProductMutationRequest`** does **not** embed images or base64—binding is separate.
- Media upload init/complete: **`V1AdminMediaUploadInitRequest`**, **`admin_media_http.go`** + **`internal/app/mediaadmin`** (presigned PUT, finalize **`media_assets`**).
- Product ↔ media bind: **`V1AdminProductImageBindRequest`**, **`V1AdminProductMediaBindRequest`** (`media_id` from **`media_assets`**).
- Generated **`docs/swagger/swagger.json`**, Postman under **`docs/postman/`**, builders **`tools/build_openapi.py`**, **`tools/build_postman_collection.py`**.

### 2.7 Tests / E2E

- **Unit:** `internal/app/salecatalog/fingerprint_test.go`, `runtime_fingerprint_test.go` cover catalog/media fingerprint stability and sensitivity to variant keys/checksums (not URLs).
- **gRPC:** `internal/grpcserver/machine_catalog_grpc_test.go` exercises media manifest/delta patterns.
- **E2E bash:** `tests/e2e/scenarios/21_grpc_bootstrap_catalog_media.sh` (bootstrap → catalog → manifest/delta/ack); `tests/e2e/scenarios/03_catalog_media_sync_rest.sh` checks HTTP sale-catalog for optional **`displayUrl`/`thumbUrl`** (does not assert checksum/manifest parity).

---

## 3. Missing media support (gaps vs requirements)

| Requirement area | Gap |
|-----------------|-----|
| **Primary image mandatory for sellable / slotted products** | Catalog admin allows **`active=true`** and planogram assignment without enforcing **`products.primary_image_id`** / runtime-visible primary media. Sale catalog can expose lines with **`image: null`** or **`deleted`** without an **`unavailable_reason`** tied to missing media (`internal/app/salecatalog/service.go` availability vs image handling). |
| **Bootstrap “media changed” signal vs catalog truth** | **`setupapp.MediaFingerprint`** (`internal/app/setupapp/fingerprints.go`) hashes **only `product_id` + `SKU`** from assortment—**not** bindings, **`media_version`**, or **`sha256`**. **`CheckForUpdates`** (`internal/grpcserver/machine_grpc_services.go`) uses this for **`media_changed`**, while **`salecatalog.MediaFingerprint`** correctly hashes projection fields. **Risk:** devices treating **`CheckForUpdates.media_changed`** as authoritative may **miss pure media updates** until **`catalog_version`** or manifest polling detects change. |
| **HTTP sale-catalog parity with gRPC manifest** | REST JSON omits **`media_variants`** / per-variant **`storage_key`** + checksums that proto **`ProductMediaRef`** carries—clients using **only REST** cannot build cache keys purely from the HTTP payload without inferring or hitting another API. |
| **URL as fetch lane, not identity** | Proto/docs already steer clients to checksum + versions; admin DTOs still expose **`imageUrl`** as a first-class field—documentation and client guidance should treat **`media_assets.id` + `object_version` / `media_version` + `sha256`** (and variant keys) as cache identity. |
| **MQTT-triggered refresh semantics** | Contract describes generic command dispatch; there is **no dedicated “refresh catalog/media” command type** documented as standard—machines today likely rely on **gRPC `CheckForUpdates` / polling** plus operational commands. If product-by-product MQTT wakeups are required, they are **not first-class** in this repo yet. |
| **Data cleanup / legacy rows** | **`BindProductPrimaryImage`** supports artifact/URL paths where **`storage_key`** may be `artifact:{uuid}` — fingerprinting falls back when **`sha256`** absent; production discipline should favor **`media_assets`** pipeline for stable **`sha256`**. |

---

## 4. Exact files likely to change (later phases)

*(Audit-only list; no edits in phase 0.)*

**Schema / SQL / sqlc**

- `db/schema/01_platform.sql` (only if new constraints or columns—prefer minimal DB churn)
- `db/queries/runtime_catalog.sql`, `db/queries/catalog_writes.sql`, `db/queries/catalog_admin.sql`
- `internal/gen/db/*.go` (regenerated)

**Domain / services**

- `internal/app/salecatalog/service.go`, `fingerprint.go`, `images.go`
- `internal/app/setupapp/fingerprints.go` (bootstrap **`MediaFingerprint`** alignment)
- `internal/app/catalogadmin/writes.go` (+ activation / validation helpers if extracted)
- `internal/app/planogram/*`, `internal/app/machineruntime/*` (if enforcing media when assigning slots / publishing)
- `internal/app/mediaadmin/*` (upload/bind hardening)

**Transports**

- `internal/grpcserver/machine_grpc_services.go` (**`CheckForUpdates`** / bootstrap **`media_fingerprint`**)
- `internal/grpcserver/machine_catalog_grpc.go` (manifest mapping, optional proto field tweaks)
- `internal/httpserver/sale_catalog_http.go` (REST parity: variants / keys)
- `internal/httpserver/admin_catalog_mutations_http.go`, `admin_catalog_http.go`, `admin_media_http.go`
- `internal/httpserver/openapi_types.go`, `swagger_operations.go`, `openapi_spec_test.go`

**Contracts & docs**

- `proto/avf/machine/v1/bootstrap.proto`, `catalog.proto`, `media.proto` (only if wire changes are unavoidable)
- `docs/swagger/swagger.json` (generated)
- `docs/postman/*.postman_collection.json` (generated or hand-maintained)
- `docs/api/mqtt-contract.md`
- `tools/build_openapi.py`, `tools/build_postman_collection.py`

**Tests / E2E**

- `internal/grpcserver/machine_catalog_grpc_test.go`
- `internal/app/salecatalog/*_test.go`
- `internal/httpserver/*_test.go` (admin + sale-catalog JSON)
- `tests/e2e/scenarios/03_catalog_media_sync_rest.sh`, `21_grpc_bootstrap_catalog_media.sh`

---

## 5. Proposed database changes

**Philosophy:** Keep blobs in object storage; DB holds metadata only (already true).

1. **Optional integrity constraints (evaluate after data audit)**  
   - Partial exclusion or deferred constraint triggers are hard for “active ⇒ primary image”; prefer **application-level validation** first to avoid breaking legacy rows.
   - If enforced in DB: consider **`CHECK`** on **`products`** such that **`active AND …`** requires **`primary_image_id IS NOT NULL`** — **high risk** without backfill; likely **not** v1.

2. **Indexing / query performance**  
   - If manifest queries grow, add covering indexes on **`product_images(product_id, status, is_primary)`** — verify existing indexes first (`ix_product_images_product_id`, partial unique on primary).

3. **No new tables required** for baseline offline cache—**`media_assets` + `product_images` + `product_media`** already model variants and versions.

---

## 6. Proposed REST changes

1. **Machine sale-catalog (`GET …/sale-catalog`)**  
   - Extend **`image`** object to mirror gRPC **`ProductMediaVariant`** (at minimum: **`variants[]`** with **`kind`**, **`storageKey`** or opaque **`variantKey`**, **`checksumSha256`**, **`mediaVersion`**, **`objectVersion`**, **`etag`**, optional **`expiresAt`** for URLs).  
   - Keep **`thumbUrl`/`displayUrl`** as **ephemeral fetch lanes** only; document cache keys **without** URL.

2. **Admin products**  
   - Keep mutations free of embedded binary (already).  
   - On **`PATCH`** with **`active: true`**, validate **`primary_image_id`** resolved and runtime projection has **`media_assets.status = ready`** when asset-bound.  
   - Optionally add **`409 conflict`** codes: `missing_primary_image`, `media_not_ready`.

3. **OpenAPI / Postman**  
   - Regenerate via **`tools/build_openapi.py`** and Postman builders; add examples showing **`variants`** and checksum-first caching notes.

---

## 7. Proposed gRPC changes

1. **`CheckForUpdates` / `GetBootstrap.media_fingerprint`**  
   - **Replace or augment** **`setupapp.MediaFingerprint`** so **`server_media_fingerprint`** matches **`salecatalog.MediaFingerprint`** for the machine’s current sale snapshot (same **`include_unavailable`** policy as contract—document explicitly).  
   - **Backward compatibility:** bump proto comments + client docs; consider dual fields only if old clients cannot migrate (avoid unless necessary).

2. **`ProductMediaRef` / manifest**  
   - Confirm **`storage_key`** or equivalent is present on variants for all bind paths (legacy artifact rows may need synthetic keys—already hashed in fingerprint fallback).

3. **Versioning**  
   - No binary fields; maintain **`AckMediaVersion`** for audit as today.

---

## 8. Proposed MQTT changes

1. **Documentation-first:** State explicitly that **media sync is pull-based** (HTTPS via presigned URLs from manifest/catalog) and MQTT only signals **work to do** (if/when commands exist).

2. **Optional:** Introduce a **`command_type`** e.g. **`catalog_sync_hint`** or **`media_refresh_hint`** with empty/minimal **`payload`** (`reason`, `correlation_id`) — **no URLs, no bytes**. Requires dispatcher registration, ACL review, and device consumer implementation **outside** this API repo.

3. **Shadow / telemetry:** If firmware already watches **`catalog_version`** via shadow, document alignment—avoid duplicating binary payloads.

---

## 9. Proposed Postman / E2E changes

1. **Postman:** Add requests asserting extended **`image.variants`** / checksum fields on sale-catalog when implemented; admin flow: upload **`media_assets`** → bind **`media_id`** → activate product → machine manifest shows **`checksum_sha256`**.

2. **E2E `03_catalog_media_sync_rest.sh`:** Upgrade from “URL present” to **`contentHash` / `mediaVersion` / variant key** assertions when REST payload extended.

3. **E2E `21_grpc_bootstrap_catalog_media.sh`:** Assert **`CheckForUpdates.media_changed`** toggles when **only** media projection changes (after fingerprint fix).

---

## 10. Migration strategy

1. **Phase A — Correctness without breaking clients**  
   - Align **`CheckForUpdates`** media fingerprint server-side; document client migration (**`media_changed`** becomes trustworthy).  
   - Add HTTP **`variants`** alongside existing fields (additive JSON).

2. **Phase B — Enforcement**  
   - Admin validation: cannot set **`active`** without primary image / ready asset.  
   - Planogram publish / slot assignment: reject products failing media readiness (define exact gate: published planogram vs draft).

3. **Phase C — Cleanup**  
   - Backfill **`sha256`** for legacy **`product_images`** rows; migrate artifact URL binds to **`media_assets`** where feasible.

4. **Rollout:** Feature-flag aggressive enforcement if production has legacy active products without images.

---

## 11. Risk list

| Risk | Mitigation |
|------|------------|
| **`CheckForUpdates` false negatives** for media changes | Unify fingerprint with **`salecatalog.MediaFingerprint`**; add regression tests. |
| **Breaking admin workflows** if activation requires image | Communicate + backfill primaries + phased enforcement. |
| **REST-only clients** missing variant keys | Extend HTTP payload; document single-source manifest recommendation. |
| **Presigned URL expiry** causing flaky downloads | Clients use **`expires_at`** + refresh manifest; server keeps **`catalog_version`** stable across URL rotation where possible (existing tests: ignore URL rotation in fingerprint). |
| **Scope creep into multi-tenant concepts** | Keep validation machine/catalog scoped; **do not** reintroduce retired partition keys in public APIs (internal SQL names may still say `ForOrg`—out of band rename only if requested). |

---

## 12. Test plan

1. **Unit**  
   - Fingerprint: media change without SKU/catalog membership change updates **`MediaFingerprint`** and (after fix) bootstrap/check-for-updates **`server_media_fingerprint`**.  
   - Sale catalog: **`is_primary`** selection; tombstone/`deleted` behavior.

2. **Integration / gRPC**  
   - `machine_catalog_grpc_test.go`: manifest/delta matches snapshot; **`basis_matches`** paths.

3. **HTTP**  
   - Golden JSON tests for **`sale-catalog`** include **`variants`** and stable hashes.

4. **Admin**  
   - Attempt **`active=true`** without primary → **`409`**.  
   - Bind **`media_assets`** not **`ready`** → clear error.

5. **E2E**  
   - Extend bash scenarios for checksum/manifest; grpc-21 validates **`media_changed`** on image-only edit.

---

## 13. Phase 0 verification commands

- `git status --short` — captured at audit time (working tree may contain unrelated local changes).
- `go test ./... -short -count=1` — **PASS** (exit 0) on audit environment.

---

## 14. Conclusion — can implementation continue?

**Yes.** The repository already has **normalized media metadata tables**, **runtime projection SQL**, **sale-catalog + gRPC manifest/delta** without binary transport, and **fingerprinting that ignores presigned URL churn**.  

The **highest-impact follow-ups** are: (1) **align bootstrap `media_fingerprint` / `CheckForUpdates.media_changed` with `salecatalog.MediaFingerprint`**, (2) **enforce primary media for active/slotted products**, and (3) **extend HTTP sale-catalog JSON for variant-level cache keys** so REST-only vending apps match the offline-cache story.
