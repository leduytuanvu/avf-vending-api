-- +goose Up
-- +goose StatementBegin

-- Lifecycle and rollout metadata for OTA campaigns.
ALTER TABLE ota_campaigns DROP CONSTRAINT IF EXISTS ota_campaigns_status_check;

UPDATE ota_campaigns SET status = CASE status
    WHEN 'active' THEN 'running'
    WHEN 'draft' THEN 'draft'
    WHEN 'paused' THEN 'paused'
    WHEN 'completed' THEN 'completed'
    ELSE 'draft'
END;

ALTER TABLE ota_campaigns RENAME COLUMN strategy TO rollout_strategy;

ALTER TABLE ota_campaigns
    ADD COLUMN IF NOT EXISTS artifact_version text,
    ADD COLUMN IF NOT EXISTS campaign_type text NOT NULL DEFAULT 'app'
        CONSTRAINT chk_ota_campaigns_type CHECK (campaign_type IN ('app', 'firmware', 'config')),
    ADD COLUMN IF NOT EXISTS canary_percent int NOT NULL DEFAULT 0
        CONSTRAINT chk_ota_campaigns_canary CHECK (canary_percent >= 0 AND canary_percent <= 100),
    ADD COLUMN IF NOT EXISTS rollback_artifact_id uuid REFERENCES ota_artifacts (id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS created_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS approved_by uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS approved_at timestamptz,
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS rollout_next_offset int NOT NULL DEFAULT 0
        CONSTRAINT chk_ota_campaigns_rollout_offset CHECK (rollout_next_offset >= 0),
    ADD COLUMN IF NOT EXISTS paused_at timestamptz;

UPDATE ota_campaigns SET rollout_strategy = 'canary' WHERE rollout_strategy = 'rolling';

ALTER TABLE ota_campaigns
    ADD CONSTRAINT chk_ota_campaigns_status CHECK (
        status IN (
            'draft',
            'approved',
            'running',
            'paused',
            'completed',
            'failed',
            'cancelled',
            'rolled_back'
        )
    );

-- Legacy rows that were "running" without governance: mark running (unchanged).
COMMENT ON COLUMN ota_campaigns.rollout_strategy IS 'immediate (single wave), canary (first subset then resume), rolling (alias treated as canary for legacy rows)';
COMMENT ON COLUMN ota_campaigns.rollout_next_offset IS 'Next index into targets sorted by machine_id for deterministic rollout waves.';
COMMENT ON COLUMN ota_campaigns.canary_percent IS '0–100; first wave size = ceil(n * percent / 100) when rollout_strategy is canary; 100 behaves like immediate for canary mode.';

ALTER TABLE ota_targets RENAME TO ota_campaign_targets;

ALTER TABLE ota_campaign_targets RENAME CONSTRAINT ux_ota_targets_campaign_machine TO ux_ota_campaign_targets_campaign_machine;

CREATE INDEX IF NOT EXISTS ix_ota_campaign_targets_campaign_id ON ota_campaign_targets (campaign_id);

CREATE TABLE ota_campaign_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    campaign_id uuid NOT NULL REFERENCES ota_campaigns (id) ON DELETE CASCADE,
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    actor_id uuid REFERENCES platform_auth_accounts (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_ota_campaign_events_campaign ON ota_campaign_events (campaign_id, created_at DESC);

CREATE TABLE ota_machine_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    campaign_id uuid NOT NULL REFERENCES ota_campaigns (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    wave text NOT NULL DEFAULT 'forward' CHECK (wave IN ('forward', 'rollback')),
    command_id uuid REFERENCES command_ledger (id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'dispatched', 'acked', 'success', 'failed')
    ),
    last_error text,
    updated_at timestamptz NOT NULL DEFAULT now (),
    created_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT ux_ota_machine_results_campaign_machine_wave UNIQUE (campaign_id, machine_id, wave)
);

CREATE INDEX ix_ota_machine_results_campaign ON ota_machine_results (campaign_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS ota_machine_results;

DROP TABLE IF EXISTS ota_campaign_events;

ALTER TABLE ota_campaign_targets RENAME TO ota_targets;

ALTER TABLE ota_targets RENAME CONSTRAINT ux_ota_campaign_targets_campaign_machine TO ux_ota_targets_campaign_machine;

DROP INDEX IF EXISTS ix_ota_campaign_targets_campaign_id;

ALTER TABLE ota_campaigns DROP CONSTRAINT IF EXISTS chk_ota_campaigns_status;

ALTER TABLE ota_campaigns DROP CONSTRAINT IF EXISTS chk_ota_campaigns_type;

ALTER TABLE ota_campaigns DROP CONSTRAINT IF EXISTS chk_ota_campaigns_canary;

ALTER TABLE ota_campaigns DROP CONSTRAINT IF EXISTS chk_ota_campaigns_rollout_offset;

ALTER TABLE ota_campaigns
    DROP COLUMN IF EXISTS artifact_version,
    DROP COLUMN IF EXISTS campaign_type,
    DROP COLUMN IF EXISTS canary_percent,
    DROP COLUMN IF EXISTS rollback_artifact_id,
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS approved_at,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS rollout_next_offset,
    DROP COLUMN IF EXISTS paused_at;

ALTER TABLE ota_campaigns RENAME COLUMN rollout_strategy TO strategy;

UPDATE ota_campaigns SET status = CASE status
    WHEN 'running' THEN 'active'
    WHEN 'approved' THEN 'draft'
    WHEN 'failed' THEN 'completed'
    WHEN 'cancelled' THEN 'completed'
    WHEN 'rolled_back' THEN 'completed'
    ELSE status
END;

ALTER TABLE ota_campaigns
    ADD CONSTRAINT ota_campaigns_status_check CHECK (
        status IN ('draft', 'active', 'paused', 'completed')
    );

-- +goose StatementEnd
