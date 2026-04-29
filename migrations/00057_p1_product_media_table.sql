-- +goose Up
-- P1.1 Materialize product_media as an authoritative table (replaces VIEW); triggers keep rows aligned with product_images + media_assets.

DROP VIEW IF EXISTS product_media;

CREATE TABLE product_media (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    media_type text NOT NULL DEFAULT 'image' CONSTRAINT chk_product_media_media_type CHECK (
        media_type IN ('image')
    ),
    source_type text NOT NULL DEFAULT 'external' CONSTRAINT chk_product_media_source_type CHECK (
        source_type IN ('upload', 'external', 'import')
    ),
    original_object_key text,
    thumb_object_key text,
    display_object_key text,
    original_url text,
    thumb_url text,
    display_url text,
    mime_type text,
    width integer,
    height integer,
    size_bytes bigint NOT NULL DEFAULT 0 CONSTRAINT chk_product_media_size_nonneg CHECK (size_bytes >= 0),
    content_hash text,
    media_version integer NOT NULL DEFAULT 1,
    sort_order integer NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'processing' CONSTRAINT chk_product_media_status CHECK (
        status IN ('processing', 'active', 'failed', 'archived')
    ),
    created_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_product_media_org_product ON product_media (organization_id, product_id);

CREATE INDEX ix_product_media_product ON product_media (product_id);

COMMENT ON TABLE product_media IS 'Denormalized catalog media projection per product_images row (id matches product_images.id); maintained by triggers.';

CREATE OR REPLACE FUNCTION refresh_product_media_row (pi_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM product_media
    WHERE
        id = pi_id;

    INSERT INTO product_media (
        id,
        organization_id,
        product_id,
        media_type,
        source_type,
        original_object_key,
        thumb_object_key,
        display_object_key,
        original_url,
        thumb_url,
        display_url,
        mime_type,
        width,
        height,
        size_bytes,
        content_hash,
        media_version,
        sort_order,
        status,
        created_by,
        created_at,
        updated_at
    )
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
        pi.thumb_cdn_url::text AS thumb_url,
        pi.cdn_url::text AS display_url,
        pi.mime_type::text AS mime_type,
        pi.width,
        pi.height,
        COALESCE(ma.size_bytes, 0::bigint) AS size_bytes,
        pi.content_hash::text AS content_hash,
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
    FROM
        product_images pi
        INNER JOIN products p ON p.id = pi.product_id
        LEFT JOIN media_assets ma ON ma.id = pi.media_asset_id
    WHERE
        pi.id = pi_id;
END;
$$;

CREATE OR REPLACE FUNCTION trg_product_images_refresh_product_media ()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF tg_op = 'DELETE' THEN
        DELETE FROM product_media
        WHERE id = old.id;

        RETURN old;
    END IF;

    PERFORM refresh_product_media_row (new.id);

    RETURN new;
END;
$$;

CREATE TRIGGER product_images_refresh_product_media_trg
AFTER INSERT OR UPDATE OR DELETE ON product_images
FOR EACH ROW
EXECUTE FUNCTION trg_product_images_refresh_product_media ();

CREATE OR REPLACE FUNCTION trg_media_assets_touch_product_media ()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    r RECORD;
BEGIN
    IF tg_op = 'DELETE' THEN
        FOR r IN
        SELECT
            id
        FROM
            product_images
        WHERE
            media_asset_id = old.id LOOP
                PERFORM refresh_product_media_row (r.id);

            END LOOP;

        RETURN old;
    END IF;

    FOR r IN
    SELECT
        id
    FROM
        product_images
    WHERE
        media_asset_id = new.id LOOP
            PERFORM refresh_product_media_row (r.id);

        END LOOP;

    RETURN new;
END;
$$;

CREATE TRIGGER media_assets_touch_product_media_trg
AFTER INSERT OR UPDATE OR DELETE ON media_assets
FOR EACH ROW
EXECUTE FUNCTION trg_media_assets_touch_product_media ();

INSERT INTO product_media (
    id,
    organization_id,
    product_id,
    media_type,
    source_type,
    original_object_key,
    thumb_object_key,
    display_object_key,
    original_url,
    thumb_url,
    display_url,
    mime_type,
    width,
    height,
    size_bytes,
    content_hash,
    media_version,
    sort_order,
    status,
    created_by,
    created_at,
    updated_at
)
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
    pi.thumb_cdn_url::text AS thumb_url,
    pi.cdn_url::text AS display_url,
    pi.mime_type::text AS mime_type,
    pi.width,
    pi.height,
    COALESCE(ma.size_bytes, 0::bigint) AS size_bytes,
    pi.content_hash::text AS content_hash,
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
FROM
    product_images pi
    INNER JOIN products p ON p.id = pi.product_id
    LEFT JOIN media_assets ma ON ma.id = pi.media_asset_id;

ALTER TABLE product_media
ADD CONSTRAINT fk_product_media_product_image_row FOREIGN KEY (product_id, id) REFERENCES product_images (product_id, id);

-- +goose Down
ALTER TABLE product_media DROP CONSTRAINT IF EXISTS fk_product_media_product_image_row;

DROP TRIGGER IF EXISTS media_assets_touch_product_media_trg ON media_assets;

DROP TRIGGER IF EXISTS product_images_refresh_product_media_trg ON product_images;

DROP FUNCTION IF EXISTS trg_media_assets_touch_product_media;

DROP FUNCTION IF EXISTS trg_product_images_refresh_product_media;

DROP FUNCTION IF EXISTS refresh_product_media_row;

DROP TABLE IF EXISTS product_media;

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
FROM
    product_images pi
    JOIN products p ON p.id = pi.product_id
    LEFT JOIN media_assets ma ON ma.id = pi.media_asset_id;
