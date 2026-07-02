-- Enterprise flow accountability fields on machine_activation_claims.
ALTER TABLE machine_activation_claims
    ADD COLUMN IF NOT EXISTS activated_by_account_id uuid,
    ADD COLUMN IF NOT EXISTS operator_session_id uuid REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS request_id text,
    ADD COLUMN IF NOT EXISTS correlation_id uuid,
    ADD COLUMN IF NOT EXISTS app_version text,
    ADD COLUMN IF NOT EXISTS boot_id text,
    ADD COLUMN IF NOT EXISTS device_serial text,
    ADD COLUMN IF NOT EXISTS reason text,
    ADD COLUMN IF NOT EXISTS activation_source text;

ALTER TABLE machine_activation_claims DROP CONSTRAINT IF EXISTS machine_activation_claims_activation_source_check;

ALTER TABLE machine_activation_claims
    ADD CONSTRAINT machine_activation_claims_activation_source_check CHECK (
        activation_source IS NULL
        OR activation_source IN (
            'activation_code',
            'reactivation_code',
            'technician_reattach',
            'admin_reattach',
            'system_recovery'
        )
    );

CREATE INDEX IF NOT EXISTS ix_machine_activation_claims_machine ON machine_activation_claims (machine_id, claimed_at DESC);

CREATE INDEX IF NOT EXISTS ix_machine_activation_claims_operator ON machine_activation_claims (operator_session_id)
WHERE
    operator_session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_machine_activation_claims_account ON machine_activation_claims (activated_by_account_id)
WHERE
    activated_by_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_machine_activation_claims_correlation ON machine_activation_claims (correlation_id)
WHERE
    correlation_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_machine_activation_claims_source ON machine_activation_claims (activation_source)
WHERE
    activation_source IS NOT NULL;

COMMENT ON TABLE machine_runtime_refresh_tokens IS 'Deprecated: canonical refresh sessions live in machine_sessions. Retained for schema compatibility only.';

ALTER TABLE machine_activation_claims ALTER COLUMN activation_code_id DROP NOT NULL;
