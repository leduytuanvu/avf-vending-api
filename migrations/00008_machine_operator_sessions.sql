-- +goose Up
-- +goose StatementBegin

-- Production notes:
-- - New tables are empty at rollout; CREATE TABLE / CREATE INDEX take short locks.
-- - Nullable FK columns on existing tables (cash_collections, command_ledger) avoid DEFAULT
--   backfills; brief ACCESS EXCLUSIVE on ALTER TABLE only for metadata (PG 11+).
-- - v_machine_current_operator is a plain VIEW (not materialized): no refresh job; cardinality
--   of at most one ACTIVE session per machine is enforced by ux_machine_operator_sessions_one_active.

-- Enum semantics (this repo uses text + CHECK, not CREATE TYPE):
-- operator_actor_type: TECHNICIAN | USER
-- operator_session_status: ACTIVE | ENDED | EXPIRED | REVOKED
-- operator_auth_method: pin | password | badge | oidc | device_cert | unknown
-- operator_auth_event_type: login_success | login_failure | logout | session_refresh | lockout | unknown
-- action_origin_type: operator_session | system | scheduled | api | remote_support

-- ---------------------------------------------------------------------------
-- machine_operator_sessions: one ACTIVE session per machine (partial unique)
-- ---------------------------------------------------------------------------
CREATE TABLE machine_operator_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    actor_type text NOT NULL CHECK (actor_type IN ('TECHNICIAN', 'USER')),
    technician_id uuid REFERENCES technicians (id) ON DELETE SET NULL,
    user_principal text,
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'ENDED', 'EXPIRED', 'REVOKED')),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    expires_at timestamptz,
    client_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_operator_session_actor_shape CHECK (
        (actor_type = 'TECHNICIAN' AND technician_id IS NOT NULL AND user_principal IS NULL)
        OR (
            actor_type = 'USER'
            AND technician_id IS NULL
            AND user_principal IS NOT NULL
            AND btrim(user_principal) <> ''
        )
    )
);

CREATE UNIQUE INDEX ux_machine_operator_sessions_one_active ON machine_operator_sessions (machine_id)
    WHERE status = 'ACTIVE';

CREATE INDEX ix_machine_operator_sessions_machine_started ON machine_operator_sessions (machine_id, started_at DESC);
CREATE INDEX ix_machine_operator_sessions_org_started ON machine_operator_sessions (organization_id, started_at DESC);
CREATE INDEX ix_machine_operator_sessions_technician ON machine_operator_sessions (technician_id, started_at DESC)
    WHERE technician_id IS NOT NULL;
CREATE INDEX ix_machine_operator_sessions_user_principal ON machine_operator_sessions (organization_id, user_principal, started_at DESC)
    WHERE actor_type = 'USER' AND user_principal IS NOT NULL;
CREATE INDEX ix_machine_operator_sessions_org_machine_started ON machine_operator_sessions (organization_id, machine_id, started_at DESC);
CREATE INDEX ix_machine_operator_sessions_org_active_started ON machine_operator_sessions (organization_id, started_at DESC)
    WHERE status = 'ACTIVE';

COMMENT ON TABLE machine_operator_sessions IS 'Machine-side operator login context; machine identity stays on machines, technician identity on technicians, USER uses opaque user_principal (IdP sub / admin id).';
COMMENT ON COLUMN machine_operator_sessions.user_principal IS 'Non-technician operator identity when actor_type=USER; never store technician PII here.';
COMMENT ON COLUMN machine_operator_sessions.client_metadata IS 'Device/session hints (app version, locale); avoid secrets.';

-- ---------------------------------------------------------------------------
-- machine_operator_auth_events: immutable login/logout trace (INSERT-only in app)
-- ---------------------------------------------------------------------------
CREATE TABLE machine_operator_auth_events (
    id bigserial PRIMARY KEY,
    operator_session_id uuid NOT NULL REFERENCES machine_operator_sessions (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (
        event_type IN ('login_success', 'login_failure', 'logout', 'session_refresh', 'lockout', 'unknown')
    ),
    auth_method text NOT NULL CHECK (
        auth_method IN ('pin', 'password', 'badge', 'oidc', 'device_cert', 'unknown')
    ),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    correlation_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_machine_operator_auth_events_machine_time ON machine_operator_auth_events (machine_id, occurred_at DESC);
CREATE INDEX ix_machine_operator_auth_events_session_time ON machine_operator_auth_events (operator_session_id, occurred_at DESC)
    WHERE operator_session_id IS NOT NULL;
CREATE INDEX ix_machine_operator_auth_events_correlation ON machine_operator_auth_events (correlation_id, occurred_at DESC)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE machine_operator_auth_events IS 'Append-only auth audit for operator sessions; do not UPDATE rows.';

-- ---------------------------------------------------------------------------
-- machine_action_attributions: generic resource → session linkage
-- ---------------------------------------------------------------------------
CREATE TABLE machine_action_attributions (
    id bigserial PRIMARY KEY,
    operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    action_origin_type text NOT NULL CHECK (
        action_origin_type IN ('operator_session', 'system', 'scheduled', 'api', 'remote_support')
    ),
    resource_type text NOT NULL,
    resource_id text NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX ix_machine_action_attributions_resource_time ON machine_action_attributions (resource_type, resource_id, occurred_at DESC);
CREATE INDEX ix_machine_action_attributions_machine_resource_time ON machine_action_attributions (machine_id, resource_type, resource_id, occurred_at DESC);
CREATE INDEX ix_machine_action_attributions_session_time ON machine_action_attributions (operator_session_id, occurred_at DESC)
    WHERE operator_session_id IS NOT NULL;
CREATE INDEX ix_machine_action_attributions_machine_time ON machine_action_attributions (machine_id, occurred_at DESC);

COMMENT ON TABLE machine_action_attributions IS 'Links domain actions to operator_session_id when known; resource_type/resource_id are polymorphic (e.g. command_ledger uuid as text).';
COMMENT ON COLUMN machine_action_attributions.operator_session_id IS 'NULL allowed for unattended/system/scheduled actions.';

-- ---------------------------------------------------------------------------
-- FK: operator_session_id on operational tables (additive)
-- ---------------------------------------------------------------------------
ALTER TABLE cash_collections
    ADD COLUMN IF NOT EXISTS operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_cash_collections_operator_session ON cash_collections (operator_session_id)
    WHERE operator_session_id IS NOT NULL;

ALTER TABLE command_ledger
    ADD COLUMN IF NOT EXISTS operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_command_ledger_operator_session ON command_ledger (operator_session_id)
    WHERE operator_session_id IS NOT NULL;

COMMENT ON COLUMN cash_collections.operator_session_id IS 'Operator session active during physical collection when recorded.';
COMMENT ON COLUMN command_ledger.operator_session_id IS 'This repo uses command_ledger as machine command rows (no separate machine_commands table).';

-- Optional tables: add column only if table exists (idempotent for mixed migration states)
DO $do$
BEGIN
    IF to_regclass('public.refill_sessions') IS NOT NULL
        AND NOT EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
                AND table_name = 'refill_sessions'
                AND column_name = 'operator_session_id'
        ) THEN
        EXECUTE 'ALTER TABLE refill_sessions ADD COLUMN IF NOT EXISTS operator_session_id uuid REFERENCES machine_operator_sessions(id) ON DELETE SET NULL';
    END IF;
    IF to_regclass('public.machine_configs') IS NOT NULL
        AND NOT EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
                AND table_name = 'machine_configs'
                AND column_name = 'operator_session_id'
        ) THEN
        EXECUTE 'ALTER TABLE machine_configs ADD COLUMN IF NOT EXISTS operator_session_id uuid REFERENCES machine_operator_sessions(id) ON DELETE SET NULL';
    END IF;
    IF to_regclass('public.incidents') IS NOT NULL
        AND NOT EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
                AND table_name = 'incidents'
                AND column_name = 'operator_session_id'
        ) THEN
        EXECUTE 'ALTER TABLE incidents ADD COLUMN IF NOT EXISTS operator_session_id uuid REFERENCES machine_operator_sessions(id) ON DELETE SET NULL';
    END IF;
    IF to_regclass('public.machine_commands') IS NOT NULL
        AND NOT EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
                AND table_name = 'machine_commands'
                AND column_name = 'operator_session_id'
        ) THEN
        EXECUTE 'ALTER TABLE machine_commands ADD COLUMN IF NOT EXISTS operator_session_id uuid REFERENCES machine_operator_sessions(id) ON DELETE SET NULL';
    END IF;
END
$do$;

-- ---------------------------------------------------------------------------
-- Helper view: current ACTIVE operator per machine
-- Join is on (machine_id, status = ACTIVE); ux_machine_operator_sessions_one_active guarantees
-- at most one matching session row per machine (plain VIEW — no MV maintenance).
-- ---------------------------------------------------------------------------
CREATE VIEW v_machine_current_operator AS
SELECT
    m.id AS machine_id,
    m.organization_id,
    s.id AS operator_session_id,
    s.actor_type,
    s.technician_id,
    t.display_name AS technician_display_name,
    s.user_principal,
    s.started_at AS session_started_at,
    s.status AS session_status,
    s.expires_at AS session_expires_at
FROM machines m
LEFT JOIN machine_operator_sessions s ON s.machine_id = m.id
    AND s.status = 'ACTIVE'
LEFT JOIN technicians t ON t.id = s.technician_id;

COMMENT ON VIEW v_machine_current_operator IS 'Convenience join for UI: one row per machine; operator_session_id NULL when nobody logged in. At most one ACTIVE session per machine is enforced by index ux_machine_operator_sessions_one_active.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP VIEW IF EXISTS v_machine_current_operator;

DO $do$
BEGIN
    IF to_regclass('public.machine_commands') IS NOT NULL
        AND EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
                AND table_name = 'machine_commands'
                AND column_name = 'operator_session_id'
        ) THEN
        EXECUTE 'ALTER TABLE machine_commands DROP COLUMN IF EXISTS operator_session_id';
    END IF;
    IF to_regclass('public.incidents') IS NOT NULL
        AND EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
                AND table_name = 'incidents'
                AND column_name = 'operator_session_id'
        ) THEN
        EXECUTE 'ALTER TABLE incidents DROP COLUMN IF EXISTS operator_session_id';
    END IF;
    IF to_regclass('public.machine_configs') IS NOT NULL
        AND EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
                AND table_name = 'machine_configs'
                AND column_name = 'operator_session_id'
        ) THEN
        EXECUTE 'ALTER TABLE machine_configs DROP COLUMN IF EXISTS operator_session_id';
    END IF;
    IF to_regclass('public.refill_sessions') IS NOT NULL
        AND EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
                AND table_name = 'refill_sessions'
                AND column_name = 'operator_session_id'
        ) THEN
        EXECUTE 'ALTER TABLE refill_sessions DROP COLUMN IF EXISTS operator_session_id';
    END IF;
END
$do$;

DROP INDEX IF EXISTS ix_command_ledger_operator_session;

ALTER TABLE command_ledger DROP COLUMN IF EXISTS operator_session_id;

DROP INDEX IF EXISTS ix_cash_collections_operator_session;

ALTER TABLE cash_collections DROP COLUMN IF EXISTS operator_session_id;

DROP TABLE IF EXISTS machine_action_attributions;

DROP TABLE IF EXISTS machine_operator_auth_events;

DROP TABLE IF EXISTS machine_operator_sessions;

-- +goose StatementEnd
