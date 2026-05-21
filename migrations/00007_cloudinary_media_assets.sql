-- +goose Up
-- Cloudinary-backed product image metadata (server-side upload; no object storage required).

ALTER TABLE media_assets
    DROP CONSTRAINT IF EXISTS chk_media_assets_source_type;

ALTER TABLE media_assets
    ADD CONSTRAINT chk_media_assets_source_type CHECK (
        source_type IN ('upload', 'external', 'import', 'cloudinary')
    );

ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS storage_provider text NOT NULL DEFAULT 's3',
    ADD COLUMN IF NOT EXISTS provider_public_id text,
    ADD COLUMN IF NOT EXISTS provider_asset_id text;

ALTER TABLE media_assets
    DROP CONSTRAINT IF EXISTS chk_media_assets_storage_provider;

ALTER TABLE media_assets
    ADD CONSTRAINT chk_media_assets_storage_provider CHECK (
        storage_provider IN ('s3', 'cloudinary', 'external')
    );

UPDATE media_assets
SET
    storage_provider = 'external'
WHERE
    source_type = 'external'
    AND storage_provider = 's3';

CREATE UNIQUE INDEX IF NOT EXISTS ux_media_assets_storage_provider_public_id ON media_assets (storage_provider, provider_public_id)
WHERE
    provider_public_id IS NOT NULL
    AND btrim(provider_public_id) <> ''
    AND status NOT IN ('deleted', 'archived');

COMMENT ON COLUMN media_assets.storage_provider IS 'Backing store: s3 (object storage), cloudinary, or external (HTTPS URL only).';
COMMENT ON COLUMN media_assets.provider_public_id IS 'Provider-native stable id (e.g. Cloudinary public_id).';
COMMENT ON COLUMN media_assets.provider_asset_id IS 'Provider asset id when distinct from public_id (e.g. Cloudinary asset_id).';

-- +goose Down
DROP INDEX IF EXISTS ux_media_assets_storage_provider_public_id;

ALTER TABLE media_assets
    DROP CONSTRAINT IF EXISTS chk_media_assets_storage_provider;

ALTER TABLE media_assets
    DROP COLUMN IF EXISTS provider_asset_id,
    DROP COLUMN IF EXISTS provider_public_id,
    DROP COLUMN IF EXISTS storage_provider;

ALTER TABLE media_assets
    DROP CONSTRAINT IF EXISTS chk_media_assets_source_type;

ALTER TABLE media_assets
    ADD CONSTRAINT chk_media_assets_source_type CHECK (
        source_type IN ('upload', 'external', 'import')
    );
