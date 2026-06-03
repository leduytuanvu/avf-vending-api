-- +goose Up
CREATE UNIQUE INDEX IF NOT EXISTS ux_payment_provider_settlements_provider_id
    ON payment_provider_settlements (provider, provider_settlement_id);

-- +goose Down
DROP INDEX IF EXISTS ux_payment_provider_settlements_provider_id;
