-- Allow PSP-native IPN deliveries (MoMo/ZaloPay) verified by provider SDK, not HMAC.
ALTER TABLE payment_provider_events
    DROP CONSTRAINT IF EXISTS chk_payment_provider_events_validation_status;

ALTER TABLE payment_provider_events
    ADD CONSTRAINT chk_payment_provider_events_validation_status CHECK (
        validation_status IN (
            'hmac_verified',
            'unsigned_development',
            'invalid_signature',
            'provider_native_verified'
        )
    );
