-- +goose Up
-- +goose StatementBegin
-- Operator-attributed domain rows (machine_commands are represented by command_ledger in this repo).
-- Ordering: runs after 00008 (sessions + optional ALTER); CREATE TABLE IF NOT EXISTS keeps mixed states safe.
CREATE TABLE IF NOT EXISTS refill_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_refill_sessions_machine_started ON refill_sessions (machine_id, started_at DESC);

CREATE INDEX IF NOT EXISTS ix_refill_sessions_org_started ON refill_sessions (organization_id, started_at DESC);

CREATE INDEX IF NOT EXISTS ix_refill_sessions_operator_session ON refill_sessions (operator_session_id)
WHERE
    operator_session_id IS NOT NULL;

COMMENT ON TABLE refill_sessions IS 'Field refill visit context; link operator_session_id for attribution.';

CREATE TABLE IF NOT EXISTS machine_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    applied_at timestamptz NOT NULL DEFAULT now(),
    config_revision int NOT NULL DEFAULT 1,
    config_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_machine_configs_machine_applied ON machine_configs (machine_id, applied_at DESC);

CREATE INDEX IF NOT EXISTS ix_machine_configs_org_applied ON machine_configs (organization_id, applied_at DESC);

CREATE INDEX IF NOT EXISTS ix_machine_configs_operator_session ON machine_configs (operator_session_id)
WHERE
    operator_session_id IS NOT NULL;

COMMENT ON TABLE machine_configs IS 'Machine-local config application snapshots; operator_session_id when applied by a logged-in operator.';

CREATE TABLE IF NOT EXISTS incidents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'open' CHECK (
        status IN ('open', 'acknowledged', 'in_progress', 'resolved', 'closed', 'cancelled')
    ),
    title text NOT NULL DEFAULT '',
    opened_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS ix_incidents_machine_updated ON incidents (machine_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS ix_incidents_org_opened ON incidents (organization_id, opened_at DESC);

CREATE INDEX IF NOT EXISTS ix_incidents_operator_session ON incidents (operator_session_id)
WHERE
    operator_session_id IS NOT NULL;

COMMENT ON TABLE incidents IS 'Machine-side incidents; operator_session_id for operator-opened or last operator update when recorded.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS incidents;

DROP TABLE IF EXISTS machine_configs;

DROP TABLE IF EXISTS refill_sessions;

-- +goose StatementEnd
