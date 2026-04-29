-- +goose Up
-- +goose StatementBegin

-- P1.4: machine tag targeting for fleet rollout + app_version rollout type.

CREATE TABLE machine_tag_assignments (
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now (),
    PRIMARY KEY (machine_id, tag_id)
);

CREATE INDEX ix_machine_tag_assignments_org_tag ON machine_tag_assignments (organization_id, tag_id);

CREATE INDEX ix_machine_tag_assignments_org_machine ON machine_tag_assignments (organization_id, machine_id);

COMMENT ON TABLE machine_tag_assignments IS 'Tenant-scoped machine labels (catalog tags) for fleet targeting (rollouts, filters).';

ALTER TABLE rollout_campaigns
DROP CONSTRAINT rollout_campaigns_rollout_type_check;

ALTER TABLE rollout_campaigns
ADD CONSTRAINT rollout_campaigns_rollout_type_check CHECK (
    rollout_type IN (
        'config_version',
        'catalog_version',
        'media_version',
        'planogram_version',
        'app_version'
    )
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE rollout_campaigns
DROP CONSTRAINT rollout_campaigns_rollout_type_check;

ALTER TABLE rollout_campaigns
ADD CONSTRAINT rollout_campaigns_rollout_type_check CHECK (
    rollout_type IN (
        'config_version',
        'catalog_version',
        'media_version',
        'planogram_version'
    )
);

DROP INDEX IF EXISTS ix_machine_tag_assignments_org_machine;

DROP INDEX IF EXISTS ix_machine_tag_assignments_org_tag;

DROP TABLE IF EXISTS machine_tag_assignments;

-- +goose StatementEnd
