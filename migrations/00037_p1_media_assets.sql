-- P1.1 enterprise media assets (object storage metadata + variants). product_images may reference media_assets.

-- +goose Up
CREATE TABLE media_assets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    kind text NOT NULL DEFAULT 'product_image' CONSTRAINT chk_media_assets_kind CHECK (
        kind IN ('product_image')
    ),
    original_object_key text NOT NULL,
    thumb_object_key text NOT NULL,
    display_object_key text NOT NULL,
    mime_type text,
    size_bytes bigint CHECK (size_bytes IS NULL OR size_bytes >= 0),
    sha256 text,
    width int CHECK (width IS NULL OR width >= 0),
    height int CHECK (height IS NULL OR height >= 0),
    object_version int NOT NULL DEFAULT 1,
    etag text,
    status text NOT NULL DEFAULT 'pending' CONSTRAINT chk_media_assets_status CHECK (
        status IN ('pending', 'processing', 'ready', 'deleted')
    ),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_media_assets_org_created ON media_assets (organization_id, created_at DESC);

CREATE INDEX ix_media_assets_org_status ON media_assets (organization_id, status);

ALTER TABLE product_images
ADD COLUMN media_asset_id uuid REFERENCES media_assets (id) ON DELETE SET NULL;

CREATE INDEX ix_product_images_media_asset ON product_images (media_asset_id)
WHERE
    media_asset_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS ix_product_images_media_asset;

ALTER TABLE product_images DROP COLUMN IF EXISTS media_asset_id;

DROP INDEX IF EXISTS ix_media_assets_org_status;

DROP INDEX IF EXISTS ix_media_assets_org_created;

DROP TABLE IF EXISTS media_assets;
