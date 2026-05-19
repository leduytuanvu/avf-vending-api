-- +goose Up

-- Phase 1: offline-capable media metadata — variants table, asset filename/canonical key, product_media primary vs gallery role,
-- partial index for audits (active products missing primary_image_id). No blocking CHECK on products.active (draft SKUs default active=true).

ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS original_filename text,
    ADD COLUMN IF NOT EXISTS object_key text;

COMMENT ON COLUMN media_assets.kind IS 'Asset purpose; product_image is the primary vending product image pipeline.';

COMMENT ON COLUMN media_assets.object_key IS 'Canonical object-store key for the primary upload (typically mirrors original_object_key); use media_variants for per-rendition keys and hashes.';

COMMENT ON COLUMN media_assets.object_version IS 'Logical asset version for cache busting / offline sync (increment on substantive metadata or derivative updates).';

UPDATE media_assets
SET
    object_key = NULLIF(TRIM(original_object_key), '')
WHERE
    object_key IS NULL
    AND original_object_key IS NOT NULL;

CREATE TABLE media_variants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    media_asset_id uuid NOT NULL REFERENCES media_assets (id) ON DELETE CASCADE,
    variant text NOT NULL CONSTRAINT chk_media_variants_variant CHECK (
        variant IN ('original', 'thumb', 'display', 'fallback')
    ),
    object_key text NOT NULL,
    mime_type text,
    width int CHECK (width IS NULL OR width >= 0),
    height int CHECK (height IS NULL OR height >= 0),
    size_bytes bigint CHECK (size_bytes IS NULL OR size_bytes >= 0),
    sha256 text,
    version int NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_media_variants_asset_variant UNIQUE (media_asset_id, variant)
);

CREATE INDEX ix_media_variants_media_asset_id ON media_variants (media_asset_id);

CREATE INDEX ix_media_variants_sha256 ON media_variants (sha256)
WHERE
    sha256 IS NOT NULL;

INSERT INTO media_variants (
    media_asset_id,
    variant,
    object_key,
    mime_type,
    width,
    height,
    size_bytes,
    sha256,
    version,
    created_at,
    updated_at
)
SELECT
    ma.id,
    'original',
    ma.original_object_key,
    ma.mime_type,
    ma.width,
    ma.height,
    ma.size_bytes,
    ma.sha256,
    ma.object_version,
    ma.created_at,
    ma.updated_at
FROM
    media_assets ma
ON CONFLICT (media_asset_id, variant)
    DO NOTHING;

INSERT INTO media_variants (
    media_asset_id,
    variant,
    object_key,
    mime_type,
    width,
    height,
    size_bytes,
    sha256,
    version,
    created_at,
    updated_at
)
SELECT
    ma.id,
    'thumb',
    ma.thumb_object_key,
    ma.mime_type,
    ma.width,
    ma.height,
    ma.size_bytes,
    NULL::text,
    ma.object_version,
    ma.created_at,
    ma.updated_at
FROM
    media_assets ma
ON CONFLICT (media_asset_id, variant)
    DO NOTHING;

INSERT INTO media_variants (
    media_asset_id,
    variant,
    object_key,
    mime_type,
    width,
    height,
    size_bytes,
    sha256,
    version,
    created_at,
    updated_at
)
SELECT
    ma.id,
    'display',
    ma.display_object_key,
    ma.mime_type,
    ma.width,
    ma.height,
    ma.size_bytes,
    NULL::text,
    ma.object_version,
    ma.created_at,
    ma.updated_at
FROM
    media_assets ma
ON CONFLICT (media_asset_id, variant)
    DO NOTHING;

ALTER TABLE product_media
    ADD COLUMN IF NOT EXISTS media_role text NOT NULL DEFAULT 'gallery';

ALTER TABLE product_media DROP CONSTRAINT IF EXISTS chk_product_media_media_role;

ALTER TABLE product_media
    ADD CONSTRAINT chk_product_media_media_role CHECK (
        media_role IN ('primary', 'gallery')
    );

UPDATE product_media pm
SET
    media_role = 'primary'
FROM
    products p
WHERE
    p.id = pm.product_id
    AND p.primary_image_id IS NOT NULL
    AND p.primary_image_id = pm.id;

CREATE UNIQUE INDEX ux_product_media_one_primary_per_product ON product_media (product_id)
WHERE
    media_role = 'primary';

CREATE INDEX ix_product_media_product_role ON product_media (product_id, media_role);

CREATE INDEX ix_products_active_missing_primary_image ON products (id)
WHERE
    active
    AND primary_image_id IS NULL;

COMMENT ON TABLE media_variants IS 'Per-rendition object keys and optional per-variant sha256/version for kiosk offline caches.';

COMMENT ON COLUMN product_media.media_role IS 'primary: matches products.primary_image_id for this projection row; gallery: additional images.';

-- +goose Down

DROP INDEX IF EXISTS ix_products_active_missing_primary_image;

DROP INDEX IF EXISTS ux_product_media_one_primary_per_product;

DROP INDEX IF EXISTS ix_product_media_product_role;

ALTER TABLE product_media DROP CONSTRAINT IF EXISTS chk_product_media_media_role;

ALTER TABLE product_media DROP COLUMN IF EXISTS media_role;

DROP TABLE IF EXISTS media_variants;

ALTER TABLE media_assets DROP COLUMN IF EXISTS original_filename;

ALTER TABLE media_assets DROP COLUMN IF EXISTS object_key;
