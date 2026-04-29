-- +goose Up
ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS source_type text NOT NULL DEFAULT 'upload',
    ADD COLUMN IF NOT EXISTS original_url text,
    ADD COLUMN IF NOT EXISTS created_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS failed_reason text;

ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS chk_media_assets_source_type;
ALTER TABLE media_assets
    ADD CONSTRAINT chk_media_assets_source_type CHECK (source_type IN ('upload', 'external', 'import'));

ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS chk_media_assets_status;
ALTER TABLE media_assets
    ADD CONSTRAINT chk_media_assets_status CHECK (status IN ('pending', 'processing', 'ready', 'failed', 'deleted', 'archived'));

ALTER TABLE product_images
    ADD COLUMN IF NOT EXISTS media_version int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE product_images DROP CONSTRAINT IF EXISTS chk_product_images_status;
ALTER TABLE product_images
    ADD CONSTRAINT chk_product_images_status CHECK (status IN ('active', 'archived'));

DROP INDEX IF EXISTS ux_product_images_one_primary_per_product;
CREATE UNIQUE INDEX IF NOT EXISTS ux_product_images_one_primary_per_product
    ON product_images (product_id)
    WHERE is_primary AND status = 'active';

CREATE OR REPLACE VIEW product_media AS
SELECT
    pi.id,
    p.organization_id,
    pi.product_id,
    'image'::text AS media_type,
    COALESCE(ma.source_type, 'external'::text) AS source_type,
    ma.original_object_key,
    ma.thumb_object_key,
    ma.display_object_key,
    ma.original_url,
    pi.thumb_cdn_url AS thumb_url,
    pi.cdn_url AS display_url,
    pi.mime_type,
    pi.width,
    pi.height,
    COALESCE(ma.size_bytes, 0::bigint) AS size_bytes,
    pi.content_hash,
    pi.media_version,
    pi.sort_order,
    CASE
        WHEN pi.status = 'archived' THEN 'archived'
        WHEN ma.status = 'failed' THEN 'failed'
        WHEN ma.status IN ('pending', 'processing') THEN 'processing'
        ELSE 'active'
    END AS status,
    ma.created_by,
    pi.created_at,
    pi.updated_at
FROM product_images pi
JOIN products p ON p.id = pi.product_id
LEFT JOIN media_assets ma ON ma.id = pi.media_asset_id;

-- +goose Down
DROP VIEW IF EXISTS product_media;

DROP INDEX IF EXISTS ux_product_images_one_primary_per_product;
CREATE UNIQUE INDEX IF NOT EXISTS ux_product_images_one_primary_per_product
    ON product_images (product_id)
    WHERE is_primary;

ALTER TABLE product_images DROP CONSTRAINT IF EXISTS chk_product_images_status;
ALTER TABLE product_images
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS media_version;

ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS chk_media_assets_status;
ALTER TABLE media_assets
    ADD CONSTRAINT chk_media_assets_status CHECK (status IN ('pending', 'processing', 'ready', 'deleted'));

ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS chk_media_assets_source_type;
ALTER TABLE media_assets
    DROP COLUMN IF EXISTS failed_reason,
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS original_url,
    DROP COLUMN IF EXISTS source_type;
