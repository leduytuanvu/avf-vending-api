-- +goose Up

CREATE TABLE machine_activation_claims (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    activation_code_id uuid NOT NULL REFERENCES machine_activation_codes (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    fingerprint_hash bytea NOT NULL,
    claimed_at timestamptz NOT NULL DEFAULT now(),
    ip_address text NOT NULL DEFAULT '',
    user_agent text NOT NULL DEFAULT '',
    result text NOT NULL CHECK (
        result IN (
            'succeeded',
            'failed',
            'rejected'
        )
    ),
    failure_reason text NOT NULL DEFAULT ''
);

CREATE INDEX ix_machine_activation_claims_code ON machine_activation_claims (
    activation_code_id,
    claimed_at DESC
);

CREATE INDEX ix_machine_activation_claims_org_machine ON machine_activation_claims (organization_id, machine_id);

CREATE UNIQUE INDEX ux_machine_activation_claim_code_fp_succeeded ON machine_activation_claims (
    activation_code_id,
    fingerprint_hash
)
WHERE
    result = 'succeeded';

COMMENT ON TABLE machine_activation_claims IS 'Audit trail and idempotency for activation code claims; successful claim counts enforce max_uses.';

-- Backfill one succeeded row per legacy activation code that had a recorded fingerprint claim.
INSERT INTO machine_activation_claims (
    activation_code_id,
    organization_id,
    machine_id,
    fingerprint_hash,
    claimed_at,
    ip_address,
    user_agent,
    result,
    failure_reason
)
SELECT
    mac.id,
    mac.organization_id,
    mac.machine_id,
    mac.claimed_fingerprint_hash,
    COALESCE(mac.updated_at, mac.created_at),
    '',
    '',
    'succeeded',
    ''
FROM
    machine_activation_codes mac
WHERE
    mac.uses > 0
    AND mac.claimed_fingerprint_hash IS NOT NULL;

-- Resync uses column from successful claims (may exceed legacy uses if inconsistent).
UPDATE machine_activation_codes mac
SET
    uses = COALESCE(s.cnt, 0),
    updated_at = now()
FROM (
    SELECT
        activation_code_id,
        COUNT(*) FILTER (
            WHERE
                result = 'succeeded'
        )::int AS cnt
    FROM
        machine_activation_claims
    GROUP BY
        activation_code_id
) s
WHERE
    mac.id = s.activation_code_id;

-- +goose Down

DROP TABLE IF EXISTS machine_activation_claims;
