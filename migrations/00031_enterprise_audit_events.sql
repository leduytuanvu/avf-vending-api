-- P1.4 enterprise audit trail (audit_events). Distinct from legacy audit_logs (command/workflow rows).

CREATE TABLE audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    actor_type text NOT NULL CONSTRAINT chk_audit_events_actor_type CHECK (
        actor_type IN ('user', 'machine', 'system', 'webhook')
    ),
    actor_id text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    request_id text,
    trace_id text,
    ip_address text,
    user_agent text,
    before_json jsonb,
    after_json jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_audit_events_org_created ON audit_events (organization_id, created_at DESC);

CREATE INDEX ix_audit_events_org_action ON audit_events (organization_id, action);

CREATE INDEX ix_audit_events_org_actor ON audit_events (
    organization_id,
    actor_type,
    actor_id
)
WHERE
    actor_id IS NOT NULL;

CREATE INDEX ix_audit_events_org_resource ON audit_events (organization_id, resource_type, resource_id)
WHERE
    resource_id IS NOT NULL;
