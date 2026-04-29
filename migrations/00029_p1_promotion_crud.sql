-- +goose Up
-- +goose StatementBegin

ALTER TABLE promotions
    ADD COLUMN IF NOT EXISTS lifecycle_status text NOT NULL DEFAULT 'draft'
        CONSTRAINT chk_promotions_lifecycle CHECK (
            lifecycle_status IN ('draft', 'active', 'paused', 'deactivated')
        );

ALTER TABLE promotions
    ADD COLUMN IF NOT EXISTS priority int NOT NULL DEFAULT 0;

ALTER TABLE promotions
    ADD COLUMN IF NOT EXISTS stackable boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN promotions.lifecycle_status IS 'Operational state for redemption; distinct from approval_status governance.';
COMMENT ON COLUMN promotions.priority IS 'Higher applies first when stackable; tie-break with starts_at and id for deterministic ordering.';
COMMENT ON COLUMN promotions.stackable IS 'When false, at most one promotion applies per product line (best discount wins); when true, promotions apply in priority order.';

ALTER TABLE promotion_targets DROP CONSTRAINT IF EXISTS ck_promotion_targets_one_target;

ALTER TABLE promotion_targets
    ADD COLUMN IF NOT EXISTS tag_id uuid REFERENCES tags (id) ON DELETE CASCADE;

ALTER TABLE promotion_targets
    ADD CONSTRAINT chk_promotion_targets_one_target CHECK (
        (target_type = 'product' AND product_id IS NOT NULL AND category_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL AND tag_id IS NULL)
        OR (target_type = 'category' AND category_id IS NOT NULL AND product_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL AND tag_id IS NULL)
        OR (target_type = 'machine' AND machine_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL AND tag_id IS NULL)
        OR (target_type = 'site' AND site_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND machine_id IS NULL AND organization_target_id IS NULL AND tag_id IS NULL)
        OR (target_type = 'organization' AND organization_target_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND tag_id IS NULL)
        OR (target_type = 'tag' AND tag_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
    );

CREATE INDEX IF NOT EXISTS ix_promotion_targets_tag_id ON promotion_targets (tag_id) WHERE tag_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ix_promotion_targets_tag_id;

ALTER TABLE promotion_targets DROP CONSTRAINT IF EXISTS chk_promotion_targets_one_target;

DELETE FROM promotion_targets WHERE target_type = 'tag';

ALTER TABLE promotion_targets DROP COLUMN IF EXISTS tag_id;

ALTER TABLE promotion_targets
    ADD CONSTRAINT chk_promotion_targets_one_target CHECK (
        (target_type = 'product' AND product_id IS NOT NULL AND category_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'category' AND category_id IS NOT NULL AND product_id IS NULL AND machine_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'machine' AND machine_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND site_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'site' AND site_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND machine_id IS NULL AND organization_target_id IS NULL)
        OR (target_type = 'organization' AND organization_target_id IS NOT NULL AND product_id IS NULL AND category_id IS NULL AND machine_id IS NULL AND site_id IS NULL)
    );

ALTER TABLE promotions DROP COLUMN IF EXISTS stackable;
ALTER TABLE promotions DROP COLUMN IF EXISTS priority;
ALTER TABLE promotions DROP COLUMN IF EXISTS lifecycle_status;

-- +goose StatementEnd
