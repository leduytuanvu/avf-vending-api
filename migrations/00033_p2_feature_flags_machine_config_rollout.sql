-- +goose Up
-- +goose StatementBegin
-- P2.3 Feature flags and staged machine config rollouts (tenant-scoped).

CREATE TABLE feature_flags (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    flag_key text NOT NULL,
    display_name text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT false,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT ux_feature_flags_org_key UNIQUE (organization_id, flag_key)
);

CREATE INDEX ix_feature_flags_organization_id ON feature_flags (organization_id);

COMMENT ON TABLE feature_flags IS 'Tenant-scoped feature switches; targets refine scope (site/machine/profile/canary).';

CREATE TABLE feature_flag_targets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    feature_flag_id uuid NOT NULL REFERENCES feature_flags (id) ON DELETE CASCADE,
    target_type text NOT NULL CHECK (
        target_type IN ('organization', 'site', 'machine', 'hardware_profile', 'canary')
    ),
    site_id uuid REFERENCES sites (id) ON DELETE CASCADE,
    machine_id uuid REFERENCES machines (id) ON DELETE CASCADE,
    hardware_profile_id uuid REFERENCES machine_hardware_profiles (id) ON DELETE CASCADE,
    canary_percent numeric(5, 2) CHECK (
        canary_percent IS NULL
        OR (
            canary_percent >= 0
            AND canary_percent <= 100
        )
    ),
    priority int NOT NULL DEFAULT 0,
    enabled boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now ()
);

CREATE INDEX ix_feature_flag_targets_flag ON feature_flag_targets (feature_flag_id);

CREATE INDEX ix_feature_flag_targets_org ON feature_flag_targets (organization_id);

COMMENT ON TABLE feature_flag_targets IS 'Scoped overrides for feature_flags (highest priority matching row wins).';

CREATE TABLE machine_config_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    version_label text NOT NULL,
    config_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    parent_version_id uuid REFERENCES machine_config_versions (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT ux_machine_config_versions_org_label UNIQUE (organization_id, version_label)
);

CREATE INDEX ix_machine_config_versions_org_created ON machine_config_versions (organization_id, created_at DESC);

COMMENT ON TABLE machine_config_versions IS 'Logical remote-config bundles for staged rollout (distinct from machine_configs apply log).';

CREATE TABLE machine_config_rollouts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    target_version_id uuid NOT NULL REFERENCES machine_config_versions (id) ON DELETE RESTRICT,
    previous_version_id uuid REFERENCES machine_config_versions (id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'in_progress', 'completed', 'rolled_back', 'cancelled')
    ),
    canary_percent numeric(5, 2) CHECK (
        canary_percent IS NULL
        OR (
            canary_percent >= 0
            AND canary_percent <= 100
        )
    ),
    scope_type text NOT NULL CHECK (
        scope_type IN ('organization', 'site', 'machine', 'hardware_profile')
    ),
    site_id uuid REFERENCES sites (id) ON DELETE CASCADE,
    machine_id uuid REFERENCES machines (id) ON DELETE CASCADE,
    hardware_profile_id uuid REFERENCES machine_hardware_profiles (id) ON DELETE CASCADE,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT chk_mc_rollout_scope_exclusive CHECK (
        (
            scope_type = 'organization'
            AND site_id IS NULL
            AND machine_id IS NULL
            AND hardware_profile_id IS NULL
        )
        OR (
            scope_type = 'site'
            AND site_id IS NOT NULL
            AND machine_id IS NULL
            AND hardware_profile_id IS NULL
        )
        OR (
            scope_type = 'machine'
            AND machine_id IS NOT NULL
            AND site_id IS NULL
            AND hardware_profile_id IS NULL
        )
        OR (
            scope_type = 'hardware_profile'
            AND hardware_profile_id IS NOT NULL
            AND site_id IS NULL
            AND machine_id IS NULL
        )
    )
);

CREATE INDEX ix_machine_config_rollouts_org_created ON machine_config_rollouts (organization_id, created_at DESC);

CREATE INDEX ix_machine_config_rollouts_org_status ON machine_config_rollouts (organization_id, status);

COMMENT ON TABLE machine_config_rollouts IS 'Staged rollout of machine_config_versions with optional canary and rollback lineage.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS machine_config_rollouts;

DROP TABLE IF EXISTS machine_config_versions;

DROP TABLE IF EXISTS feature_flag_targets;

DROP TABLE IF EXISTS feature_flags;

-- +goose StatementEnd
