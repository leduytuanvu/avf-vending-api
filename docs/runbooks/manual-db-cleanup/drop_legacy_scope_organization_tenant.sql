-- =============================================================================
-- Operator-run destructive cleanup (NOT goose): legacy scope / org / tenant removal.
-- Pairing marker migration: migrations/00076_drop_legacy_scope_organization_tenant.sql
-- Prerequisites: backup, maintenance window, single-company app already deployed.
-- See docs/runbooks/manual-db-cleanup/README.md
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1) Views depending on columns we remove or reshape (recreate at end).
-- ---------------------------------------------------------------------------
DROP VIEW IF EXISTS public.machine_technician_assignments CASCADE;

DROP VIEW IF EXISTS public.v_machine_current_operator CASCADE;

DROP VIEW IF EXISTS public.payment_reconciliation_cases CASCADE;

DROP VIEW IF EXISTS public.product_media CASCADE;

-- ---------------------------------------------------------------------------
-- 2) Normalize legacy literal still carried by some CHECK enums.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
    SELECT
        c.table_schema,
        c.table_name
    FROM
        information_schema.columns c
        JOIN information_schema.tables t ON t.table_schema = c.table_schema
        AND t.table_name = c.table_name
    WHERE
        c.column_name = 'target_type'
        AND c.table_schema = 'public'
        AND t.table_type = 'BASE TABLE'
        LOOP
            EXECUTE format('UPDATE %I.%I SET target_type = %L WHERE target_type = %L', r.table_schema, r.table_name, 'global', 'company');
        END LOOP;
END
$$;

-- ---------------------------------------------------------------------------
-- 3) Drop CHECK constraints tied to old column names before RENAME / DROP.
-- ---------------------------------------------------------------------------
ALTER TABLE IF EXISTS public.price_books
    DROP CONSTRAINT IF EXISTS ck_price_books_scope_shape;

ALTER TABLE IF EXISTS public.machine_config_rollouts
    DROP CONSTRAINT IF EXISTS chk_mc_rollout_scope_exclusive;

-- ---------------------------------------------------------------------------
-- 4) Drop partial unique indexes that reference old price_book scope columns.
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS public.uniq_price_books_name_effective;

DROP INDEX IF EXISTS public.uniq_price_books_site_name_effective;

DROP INDEX IF EXISTS public.uniq_price_books_machine_name_effective;

DROP INDEX IF EXISTS public.ux_price_books_org_scope_org_name_effective;

DROP INDEX IF EXISTS public.ux_price_books_org_scope_site_name_effective;

DROP INDEX IF EXISTS public.ux_price_books_org_scope_machine_name_effective;

-- ---------------------------------------------------------------------------
-- 5) Semantic renames: keep data, remove forbidden NAME tokens from columns.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (
        SELECT
            1
        FROM
            information_schema.columns
        WHERE
            table_schema = 'public'
            AND table_name = 'technician_machine_assignments'
            AND column_name = 'scope') THEN
    ALTER TABLE public.technician_machine_assignments
        RENAME COLUMN scope TO assignment_domain;
END IF;
    IF EXISTS (
        SELECT
            1
        FROM
            information_schema.columns
        WHERE
            table_schema = 'public'
            AND table_name = 'promotions'
            AND column_name = 'channel_scope') THEN
    ALTER TABLE public.promotions
        RENAME COLUMN channel_scope TO promotion_channel_kind;
END IF;
    IF EXISTS (
        SELECT
            1
        FROM
            information_schema.columns
        WHERE
            table_schema = 'public'
            AND table_name = 'price_books'
            AND column_name = 'scope_type') THEN
    ALTER TABLE public.price_books
        RENAME COLUMN scope_type TO price_book_level;
END IF;
    IF EXISTS (
        SELECT
            1
        FROM
            information_schema.columns
        WHERE
            table_schema = 'public'
            AND table_name = 'machine_config_rollouts'
            AND column_name = 'scope_type') THEN
    ALTER TABLE public.machine_config_rollouts
        RENAME COLUMN scope_type TO rollout_target_level;
END IF;
    IF EXISTS (
        SELECT
            1
        FROM
            information_schema.columns
        WHERE
            table_schema = 'public'
            AND table_name = 'machine_mqtt_credentials'
            AND column_name = 'broker_scope') THEN
    ALTER TABLE public.machine_mqtt_credentials
        RENAME COLUMN broker_scope TO mqtt_broker_shard;
END IF;
END
$$;

DROP INDEX IF EXISTS public.ix_machine_mqtt_credentials_scope;

CREATE INDEX IF NOT EXISTS ix_machine_mqtt_credentials_shard ON public.machine_mqtt_credentials (mqtt_broker_shard);

DROP INDEX IF EXISTS public.ux_finance_daily_closes_scope;

CREATE UNIQUE INDEX IF NOT EXISTS ux_finance_daily_closes_site_machine ON public.finance_daily_closes (
    close_date,
    timezone,
    COALESCE(site_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(machine_id, '00000000-0000-0000-0000-000000000000'::uuid)
);

-- Client-supplied idempotency replay after dropping (scope_id, idempotency_key) uniqueness.
CREATE UNIQUE INDEX IF NOT EXISTS ux_finance_daily_closes_idempotency ON public.finance_daily_closes (idempotency_key);

-- Technician assignment uniqueness without legacy company column.
DROP INDEX IF EXISTS public.ux_tma_one_active_machine_technician;

CREATE UNIQUE INDEX IF NOT EXISTS ux_tma_one_active_machine_technician ON public.technician_machine_assignments (machine_id, technician_id)
WHERE
    status = 'active'
    AND valid_to IS NULL;

-- ---------------------------------------------------------------------------
-- 6) Recreate CHECK constraints using renamed columns.
-- ---------------------------------------------------------------------------
ALTER TABLE IF EXISTS public.price_books
    ADD CONSTRAINT ck_price_books_level_shape CHECK (
        (
            price_book_level = 'global'
            AND site_id IS NULL
            AND machine_id IS NULL
        )
        OR (
            price_book_level = 'site'
            AND site_id IS NOT NULL
            AND machine_id IS NULL
        )
        OR (
            price_book_level = 'machine'
            AND machine_id IS NOT NULL
            AND site_id IS NULL
        )
    );

ALTER TABLE IF EXISTS public.machine_config_rollouts
    ADD CONSTRAINT chk_mc_rollout_target_exclusive CHECK (
        (
            rollout_target_level = 'global'
            AND site_id IS NULL
            AND machine_id IS NULL
            AND hardware_profile_id IS NULL
        )
        OR (
            rollout_target_level = 'site'
            AND site_id IS NOT NULL
            AND machine_id IS NULL
            AND hardware_profile_id IS NULL
        )
        OR (
            rollout_target_level = 'machine'
            AND machine_id IS NOT NULL
            AND site_id IS NULL
            AND hardware_profile_id IS NULL
        )
        OR (
            rollout_target_level = 'hardware_profile'
            AND hardware_profile_id IS NOT NULL
            AND site_id IS NULL
            AND machine_id IS NULL
        )
    );

CREATE UNIQUE INDEX IF NOT EXISTS uniq_price_books_name_effective ON public.price_books (lower(name), effective_from)
WHERE
    price_book_level = 'global';

CREATE UNIQUE INDEX IF NOT EXISTS uniq_price_books_site_name_effective ON public.price_books (site_id, lower(name), effective_from)
WHERE
    price_book_level = 'site';

CREATE UNIQUE INDEX IF NOT EXISTS uniq_price_books_machine_name_effective ON public.price_books (machine_id, lower(name), effective_from)
WHERE
    price_book_level = 'machine';

-- ---------------------------------------------------------------------------
-- 7) Drop FK / UNIQUE / CHECK constraints whose definitions reference legacy columns.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
    SELECT
        n.nspname AS schema_name,
        c.relname AS table_name,
        con.conname
    FROM
        pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE
        n.nspname = 'public'
        AND pg_get_constraintdef(con.oid) ILIKE '%scope_id%'
        LOOP
            EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS %I CASCADE', r.schema_name, r.table_name, r.conname);
        END LOOP;
END
$$;

DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
    SELECT
        n.nspname AS schema_name,
        c.relname AS table_name,
        con.conname
    FROM
        pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE
        n.nspname = 'public'
        AND pg_get_constraintdef(con.oid) ILIKE '%organization_id%'
        LOOP
            EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS %I CASCADE', r.schema_name, r.table_name, r.conname);
        END LOOP;
END
$$;

DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
    SELECT
        n.nspname AS schema_name,
        c.relname AS table_name,
        con.conname
    FROM
        pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE
        n.nspname = 'public'
        AND pg_get_constraintdef(con.oid) ILIKE '%tenant_id%'
        LOOP
            EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS %I CASCADE', r.schema_name, r.table_name, r.conname);
        END LOOP;
END
$$;

-- ---------------------------------------------------------------------------
-- 8) Drop indexes whose definitions or names still mention legacy tokens.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
    SELECT
        schemaname,
        indexname
    FROM
        pg_indexes
    WHERE
        schemaname = 'public'
        AND (
            indexdef ILIKE '%scope_id%'
            OR indexdef ILIKE '%organization_id%'
            OR indexdef ILIKE '%tenant_id%'
            OR indexdef ILIKE '%scope_type%'
            OR indexdef ILIKE '%channel_scope%'
            OR indexname ILIKE '%organization%'
            OR indexname ILIKE '%tenant%'
            OR indexname ILIKE '%scope%'
            OR indexname ILIKE '%\_org\_%' ESCAPE '\'
        )
        LOOP
            EXECUTE format('DROP INDEX IF EXISTS %I.%I CASCADE', r.schemaname, r.indexname);
        END LOOP;
END
$$;

-- ---------------------------------------------------------------------------
-- 9) Drop canonical legacy id columns (tenant / org / company placeholder).
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
    SELECT DISTINCT
        c.table_schema,
        c.table_name
    FROM
        information_schema.columns c
        JOIN information_schema.tables t ON t.table_schema = c.table_schema
        AND t.table_name = c.table_name
    WHERE
        c.table_schema = 'public'
        AND t.table_type = 'BASE TABLE'
        AND c.column_name IN ('scope_id', 'organization_id', 'tenant_id')
        LOOP
            EXECUTE format('ALTER TABLE %I.%I DROP COLUMN IF EXISTS %I CASCADE', r.table_schema, r.table_name, 'scope_id');
            EXECUTE format('ALTER TABLE %I.%I DROP COLUMN IF EXISTS %I CASCADE', r.table_schema, r.table_name, 'organization_id');
            EXECUTE format('ALTER TABLE %I.%I DROP COLUMN IF EXISTS %I CASCADE', r.table_schema, r.table_name, 'tenant_id');
        END LOOP;
END
$$;

-- ---------------------------------------------------------------------------
-- 10) Catch-all: any remaining column NAMES matching legacy tokens.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
    SELECT
        table_schema,
        table_name,
        column_name
    FROM
        information_schema.columns c
        JOIN information_schema.tables t ON t.table_schema = c.table_schema
        AND t.table_name = c.table_name
    WHERE
        c.table_schema = 'public'
        AND t.table_type = 'BASE TABLE'
        AND (
            c.column_name ILIKE '%organization%'
            OR c.column_name ILIKE '%tenant%'
            OR c.column_name ILIKE '%scope%'
        )
        LOOP
            EXECUTE format('ALTER TABLE %I.%I DROP COLUMN IF EXISTS %I CASCADE', r.table_schema, r.table_name, r.column_name);
        END LOOP;
END
$$;

-- ---------------------------------------------------------------------------
-- 11) Drop legacy aggregate tables if present (fresh installs may not have them).
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS public.organizations CASCADE;

DROP TABLE IF EXISTS public.tenants CASCADE;

DROP TABLE IF EXISTS public.scopes CASCADE;

-- ---------------------------------------------------------------------------
-- 12) commerce_reconciliation_cases open-case uniqueness without scope_id.
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS public.ux_commerce_reconciliation_cases_open_identity;

CREATE UNIQUE INDEX IF NOT EXISTS ux_commerce_reconciliation_cases_open_identity ON public.commerce_reconciliation_cases (
    case_type,
    COALESCE(order_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(payment_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(vend_session_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(refund_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(provider_event_id, 0),
    correlation_key
)
WHERE
    status IN ('open', 'reviewing', 'escalated');

-- ---------------------------------------------------------------------------
-- 13) Global uniqueness for finance PSP imports without scope_id.
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS public.ux_payment_provider_settlements_org_provider_ext;

CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_provider_settlements_provider_ext ON public.payment_provider_settlements (provider, provider_settlement_id);

DROP INDEX IF EXISTS public.ux_payment_disputes_org_provider_ext;

CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_disputes_provider_ext ON public.payment_disputes (provider, provider_dispute_id);

-- ---------------------------------------------------------------------------
-- 14) Audit timeline indexes (no company column).
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS ix_audit_events_created ON public.audit_events (created_at DESC);

CREATE INDEX IF NOT EXISTS ix_audit_events_action ON public.audit_events (action);

CREATE INDEX IF NOT EXISTS ix_audit_events_actor ON public.audit_events (actor_type, actor_id)
WHERE
    actor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_audit_events_resource ON public.audit_events (resource_type, resource_id)
WHERE
    resource_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_audit_events_machine_created ON public.audit_events (machine_id, created_at DESC)
WHERE
    machine_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_audit_events_site_created ON public.audit_events (site_id, created_at DESC)
WHERE
    site_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 15) Recreate convenience views without legacy columns.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE VIEW public.v_machine_current_operator AS
SELECT
    m.id AS machine_id,
    s.id AS operator_session_id,
    s.actor_type,
    s.technician_id,
    t.display_name AS technician_display_name,
    s.user_principal,
    s.started_at AS session_started_at,
    s.status AS session_status,
    s.expires_at AS session_expires_at
FROM
    machines m
    LEFT JOIN machine_operator_sessions s ON s.machine_id = m.id
        AND s.status = 'ACTIVE'
    LEFT JOIN technicians t ON t.id = s.technician_id;

COMMENT ON VIEW public.v_machine_current_operator IS 'Convenience join for UI: one row per machine; operator_session_id NULL when nobody logged in.';

CREATE OR REPLACE VIEW public.machine_technician_assignments AS
SELECT
    id,
    machine_id,
    technician_id AS user_id,
    role,
    NULLIF(assignment_domain, '') AS assignment_domain,
    valid_from AS active_from,
    valid_to AS active_until,
    created_by,
    created_at
FROM
    technician_machine_assignments;

CREATE OR REPLACE VIEW public.payment_reconciliation_cases AS
SELECT
    crc.id,
    crc.machine_id,
    crc.order_id,
    crc.payment_id,
    crc.provider,
    crc.case_type,
    crc.severity,
    crc.status,
    crc.reason,
    crc.metadata,
    crc.correlation_key,
    crc.first_detected_at AS created_at,
    crc.last_detected_at AS updated_at,
    crc.resolved_at,
    crc.resolved_by
FROM
    commerce_reconciliation_cases crc;

CREATE OR REPLACE VIEW public.product_media AS
SELECT
    pi.id,
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
    CASE WHEN pi.status = 'archived' THEN
        'archived'
    WHEN ma.status = 'failed' THEN
        'failed'
    WHEN ma.status IN ('pending', 'processing') THEN
        'processing'
    ELSE
        'active'
    END AS status,
    ma.created_by,
    pi.created_at,
    pi.updated_at
FROM
    product_images pi
    JOIN products p ON p.id = pi.product_id
    LEFT JOIN media_assets ma ON ma.id = pi.media_asset_id;
