-- +goose Up
-- +goose StatementBegin
-- Enterprise audit_events historically carried a NOT NULL company-scope column (`scope_id`, or in some
-- deployments `organization_id`). Runtime INSERTs omit these columns entirely (sqlc matches canonical
-- schema without them). Legacy NOT NULL therefore surfaces as SQLSTATE 23502 during auth login audit writes.
--
-- This migration is intentionally non-destructive for CI migration safety: relax NOT NULL and drop
-- indexes that only existed for old scope-scoped timelines. Final DROP COLUMN belongs in manual cleanup
-- after verification — see docs/runbooks/manual-db-cleanup/README.md.

DROP INDEX IF EXISTS ix_audit_events_org_created;

DROP INDEX IF EXISTS ix_audit_events_org_action;

DROP INDEX IF EXISTS ix_audit_events_org_actor;

DROP INDEX IF EXISTS ix_audit_events_org_resource;

DROP INDEX IF EXISTS ix_audit_events_org_outcome;

DROP INDEX IF EXISTS ix_audit_events_company_occurred_action;

DROP INDEX IF EXISTS ix_audit_events_org_machine_created;

DROP INDEX IF EXISTS ix_audit_events_org_site_created;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'audit_events'
          AND column_name = 'scope_id'
    ) THEN
        ALTER TABLE public.audit_events
            ALTER COLUMN scope_id DROP NOT NULL;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'audit_events'
          AND column_name = 'organization_id'
    ) THEN
        ALTER TABLE public.audit_events
            ALTER COLUMN organization_id DROP NOT NULL;
    END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    RAISE EXCEPTION '00075_audit_events_relax_company_scope is intentionally irreversible; restore from backup or forward-fix';
END $$;

-- +goose StatementEnd
