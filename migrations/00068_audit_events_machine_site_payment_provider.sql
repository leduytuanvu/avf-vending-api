-- P0.5: audit_events scope columns + payment_provider actor for PSP webhook attribution.

ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS chk_audit_events_actor_type;

ALTER TABLE audit_events
    ADD CONSTRAINT chk_audit_events_actor_type CHECK (
        actor_type IN (
            'user',
            'machine',
            'system',
            'webhook',
            'service',
            'payment_provider'
        )
    );

ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS machine_id uuid REFERENCES machines (id) ON DELETE SET NULL;

ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS site_id uuid REFERENCES sites (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_audit_events_org_machine_created ON audit_events (organization_id, machine_id, created_at DESC)
WHERE
    machine_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_audit_events_org_site_created ON audit_events (organization_id, site_id, created_at DESC)
WHERE
    site_id IS NOT NULL;
