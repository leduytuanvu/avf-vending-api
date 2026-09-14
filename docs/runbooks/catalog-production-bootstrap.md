# Production Catalog Bootstrap Runbook

Rebuild the AVF production product catalog (117 SKUs) from `docs/catalog-import-final.json`.

## Prerequisites

- Production API env on app-node: `APP_ENV=production`, `DATABASE_URL`, Cloudinary (`CLOUDINARY_*`), `MEDIA_COMPANY_ID`
- Cloudinary cloud `dz4qz0tk9`, folder `avf-vending/products` (empty after wipe)
- Built CLI: `make catalog-bootstrap`

## Phase 0 — Generate manifests (workstation)

```bash
cd scripts/catalog
python enrich_legacy_manifest.py
python validate_images.py
python finalize_manifest.py
```

Outputs:

- `docs/avf_products_enriched_cloudinary_import_manifest.json`
- `docs/catalog-import-final.json`
- `.catalog-bootstrap-evidence/07-source-image-validation.json`

## Phase 1 — Preflight (production app-node)

```bash
export APP_ENV=production
# load deployments/prod/app-node/.env.app-node

bin/catalog-bootstrap \
  --manifest /path/to/catalog-import-final.json \
  --evidence-dir .catalog-bootstrap-evidence \
  --environment production \
  --preflight-only
```

## Phase 2 — Dry run

```bash
bin/catalog-bootstrap \
  --manifest /path/to/catalog-import-final.json \
  --environment production \
  --dry-run
```

## Phase 3 — Live import

```bash
export CONFIRM_CATALOG_BOOTSTRAP=CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION
scripts/catalog/run-production-catalog-bootstrap.sh
```

Or manually:

```bash
bin/catalog-bootstrap \
  --manifest /path/to/catalog-import-final.json \
  --environment production \
  --upload-images --import-taxonomy --import-products --import-prices \
  --confirm-production=CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION \
  --resume
```

## Phase 4 — Verify

```bash
bin/catalog-bootstrap \
  --manifest /path/to/catalog-import-final.json \
  --environment production \
  --verify-only
```

Acceptance: `DUPLICATE_SKU_COUNT=0`, `PRODUCTS_WITHOUT_PRIMARY_IMAGE=0`, `PRODUCTS_WITH_OLD_SOURCE_IMAGE_URL=0`.

## Resume

Re-run with `--resume` after fixing a failed SKU; checkpoint in `.catalog-bootstrap-evidence/import-state.json`.
