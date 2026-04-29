-- +goose Up
-- +goose StatementBegin

ALTER TABLE payment_provider_events
    ADD COLUMN IF NOT EXISTS validation_status text NOT NULL DEFAULT 'hmac_verified'
        CONSTRAINT chk_payment_provider_events_validation_status CHECK (
            validation_status IN ('hmac_verified', 'unsigned_development')
        );

ALTER TABLE payment_provider_events
    ADD COLUMN IF NOT EXISTS provider_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN payment_provider_events.validation_status IS 'How the HTTP layer authenticated this delivery before persistence.';
COMMENT ON COLUMN payment_provider_events.provider_metadata IS 'Optional PSP-specific metadata (non-secret JSON).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE payment_provider_events DROP COLUMN IF EXISTS provider_metadata;
ALTER TABLE payment_provider_events DROP COLUMN IF EXISTS validation_status;

-- +goose StatementEnd
