-- +goose Up
-- +goose StatementBegin
-- P2.1 Bulk machine provisioning batches and fleet rollout campaigns (MQTT command ledger integration).

CREATE TABLE machine_provisioning_batches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    site_id uuid NOT NULL REFERENCES sites (id) ON DELETE RESTRICT,
    hardware_profile_id uuid REFERENCES machine_hardware_profiles (id) ON DELETE SET NULL,
    cabinet_type text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'completed', 'failed')
    ),
    machine_count int NOT NULL DEFAULT 0,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now ()
);

CREATE INDEX ix_machine_provisioning_batches_org_created ON machine_provisioning_batches (organization_id, created_at DESC);

COMMENT ON TABLE machine_provisioning_batches IS 'Admin bulk machine creation with optional activation code fan-out.';

CREATE TABLE machine_provisioning_batch_machines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    batch_id uuid NOT NULL REFERENCES machine_provisioning_batches (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    serial_number text NOT NULL DEFAULT '',
    activation_code_id uuid REFERENCES machine_activation_codes (id) ON DELETE SET NULL,
    row_no int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT ux_prov_batch_machine UNIQUE (batch_id, machine_id)
);

CREATE INDEX ix_prov_batch_machines_batch ON machine_provisioning_batch_machines (batch_id);

COMMENT ON TABLE machine_provisioning_batch_machines IS 'Machines created in a provisioning batch; activation_code_id links hashed rows.';

CREATE TABLE rollout_campaigns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    rollout_type text NOT NULL CHECK (
        rollout_type IN (
            'config_version',
            'catalog_version',
            'media_version',
            'planogram_version'
        )
    ),
    target_version text NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (
        status IN (
            'draft',
            'pending',
            'running',
            'paused',
            'completed',
            'cancelled',
            'rolled_back'
        )
    ),
    strategy jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    started_at timestamptz,
    completed_at timestamptz,
    cancelled_at timestamptz
);

CREATE INDEX ix_rollout_campaigns_org_created ON rollout_campaigns (organization_id, created_at DESC);

CREATE INDEX ix_rollout_campaigns_org_status ON rollout_campaigns (organization_id, status);

COMMENT ON TABLE rollout_campaigns IS 'Fleet rollout driving MQTT command ledger (target_version + strategy JSON filters / canary).';

CREATE TABLE rollout_targets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    campaign_id uuid NOT NULL REFERENCES rollout_campaigns (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'pending' CHECK (
        status IN (
            'pending',
            'dispatched',
            'acknowledged',
            'succeeded',
            'failed',
            'skipped',
            'rolled_back'
        )
    ),
    err_message text,
    command_id uuid REFERENCES command_ledger (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now (),
    updated_at timestamptz NOT NULL DEFAULT now (),
    CONSTRAINT ux_rollout_campaign_machine UNIQUE (campaign_id, machine_id)
);

CREATE INDEX ix_rollout_targets_campaign ON rollout_targets (campaign_id);

CREATE INDEX ix_rollout_targets_machine ON rollout_targets (machine_id);

COMMENT ON TABLE rollout_targets IS 'Per-machine rollout state; command_id joins command_ledger / machine_command_attempts for ACK-driven status.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS rollout_targets;

DROP TABLE IF EXISTS rollout_campaigns;

DROP TABLE IF EXISTS machine_provisioning_batch_machines;

DROP TABLE IF EXISTS machine_provisioning_batches;

-- +goose StatementEnd
