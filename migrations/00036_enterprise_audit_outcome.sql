-- P0.9 enterprise audit: outcome (success/failure) and service actor_type.

-- +goose Up
ALTER TABLE audit_events
ADD COLUMN outcome text NOT NULL DEFAULT 'success' CONSTRAINT chk_audit_events_outcome CHECK (
    outcome IN ('success', 'failure')
);

ALTER TABLE audit_events DROP CONSTRAINT chk_audit_events_actor_type;

ALTER TABLE audit_events ADD CONSTRAINT chk_audit_events_actor_type CHECK (
    actor_type IN ('user', 'machine', 'system', 'webhook', 'service')
);

CREATE INDEX ix_audit_events_org_outcome ON audit_events (organization_id, outcome, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS ix_audit_events_org_outcome;

UPDATE audit_events
SET actor_type = 'system'
WHERE actor_type = 'service';

ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS chk_audit_events_actor_type;

ALTER TABLE audit_events ADD CONSTRAINT chk_audit_events_actor_type CHECK (
    actor_type IN ('user', 'machine', 'system', 'webhook')
);

ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS chk_audit_events_outcome;

ALTER TABLE audit_events DROP COLUMN IF EXISTS outcome;
