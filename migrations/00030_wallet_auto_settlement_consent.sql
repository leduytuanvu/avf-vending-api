-- +goose Up
-- Allow wallet_auto_settlement consent for automatic terminal-confirmed wallet allocation.
ALTER TABLE cash_allocations
    DROP CONSTRAINT IF EXISTS cash_allocations_consent_source_check;

ALTER TABLE cash_allocations
    ADD CONSTRAINT cash_allocations_consent_source_check CHECK (
        consent_source IN (
            'explicit_confirm',
            'implicit_post_order',
            'operator',
            'unknown',
            'wallet_auto_settlement'
        )
    );

-- +goose Down
ALTER TABLE cash_allocations
    DROP CONSTRAINT IF EXISTS cash_allocations_consent_source_check;

ALTER TABLE cash_allocations
    ADD CONSTRAINT cash_allocations_consent_source_check CHECK (
        consent_source IN ('explicit_confirm', 'implicit_post_order', 'operator', 'unknown')
    );
