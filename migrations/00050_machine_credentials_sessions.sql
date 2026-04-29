-- +goose Up
CREATE TABLE machine_credentials (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    credential_version bigint NOT NULL,
    secret_hash bytea NULL,
    status text NOT NULL CHECK (
        status IN ('active', 'rotated', 'revoked', 'compromised')
    ),
    created_at timestamptz NOT NULL DEFAULT now(),
    rotated_at timestamptz NULL,
    revoked_at timestamptz NULL,
    last_used_at timestamptz NULL,
    CONSTRAINT ux_machine_credentials_machine_version UNIQUE (machine_id, credential_version)
);

CREATE INDEX ix_machine_credentials_machine_status ON machine_credentials (machine_id, status);

CREATE TABLE machine_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    credential_id uuid NOT NULL REFERENCES machine_credentials (id) ON DELETE CASCADE,
    refresh_token_hash bytea NOT NULL,
    access_token_jti text NULL,
    refresh_token_jti text NOT NULL,
    credential_version bigint NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'revoked', 'expired')),
    issued_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz NULL,
    last_used_at timestamptz NULL,
    user_agent text NULL,
    ip_address text NULL,
    CONSTRAINT ux_machine_sessions_refresh_hash UNIQUE (refresh_token_hash)
);

CREATE UNIQUE INDEX ux_machine_sessions_one_active ON machine_sessions (machine_id)
WHERE
    status = 'active'
    AND revoked_at IS NULL;

CREATE INDEX ix_machine_sessions_machine_exp ON machine_sessions (machine_id, expires_at DESC);

CREATE INDEX ix_machine_sessions_credential ON machine_sessions (credential_id);

INSERT INTO
    machine_credentials (organization_id, machine_id, credential_version, status)
SELECT
    m.organization_id,
    m.id,
    m.credential_version,
    'active'
FROM
    machines m
ON CONFLICT (machine_id, credential_version) DO NOTHING;

INSERT INTO
    machine_sessions (
        organization_id,
        machine_id,
        credential_id,
        refresh_token_hash,
        refresh_token_jti,
        credential_version,
        status,
        issued_at,
        expires_at,
        revoked_at,
        last_used_at
    )
SELECT
    rt.organization_id,
    rt.machine_id,
    mc.id,
    rt.token_hash,
    gen_random_uuid()::text,
    m.credential_version,
    CASE
        WHEN rt.revoked_at IS NOT NULL THEN 'revoked'
        WHEN rt.expires_at <= now() THEN 'expired'
        ELSE 'active'
    END,
    rt.created_at,
    rt.expires_at,
    rt.revoked_at,
    rt.last_used_at
FROM
    machine_runtime_refresh_tokens rt
    INNER JOIN machines m ON m.id = rt.machine_id
    INNER JOIN machine_credentials mc ON mc.machine_id = m.id
    AND mc.credential_version = m.credential_version
WHERE
    NOT EXISTS (
        SELECT
            1
        FROM
            machine_sessions ms
        WHERE
            ms.refresh_token_hash = rt.token_hash
    );

-- +goose Down
DROP INDEX IF EXISTS ix_machine_sessions_credential;

DROP INDEX IF EXISTS ix_machine_sessions_machine_exp;

DROP INDEX IF EXISTS ux_machine_sessions_one_active;

DROP TABLE IF EXISTS machine_sessions;

DROP INDEX IF EXISTS ix_machine_credentials_machine_status;

DROP TABLE IF EXISTS machine_credentials;
