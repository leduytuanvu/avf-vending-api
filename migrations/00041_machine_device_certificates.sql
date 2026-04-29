-- +goose Up
-- +goose StatementBegin
-- P2.3: device TLS client certificate metadata for machine gRPC mTLS (registration/revocation/rotation; no private keys stored).
CREATE TABLE machine_device_certificates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    fingerprint_sha256 bytea NOT NULL,
    serial_number text NOT NULL,
    subject_dn text NOT NULL,
    issuer_dn text,
    sans_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    not_before timestamptz NOT NULL,
    not_after timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'active' CONSTRAINT chk_machine_device_certificates_status CHECK (
        status IN ('registered', 'active', 'revoked', 'superseded')
    ),
    superseded_by uuid REFERENCES machine_device_certificates (id) ON DELETE SET NULL,
    revoked_at timestamptz,
    revoke_reason text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_machine_device_certificates_fp UNIQUE (fingerprint_sha256)
);

CREATE INDEX ix_machine_device_certificates_machine_status ON machine_device_certificates (machine_id, status);

CREATE INDEX ix_machine_device_certificates_org_machine ON machine_device_certificates (organization_id, machine_id);

COMMENT ON TABLE machine_device_certificates IS 'P2.3: registered device client cert fingerprints for mTLS + lifecycle (revoke/supersede).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS machine_device_certificates;

-- +goose StatementEnd
