-- +goose Up
-- +goose StatementBegin

ALTER TABLE payment_provider_events
    ADD COLUMN IF NOT EXISTS webhook_event_id text;

CREATE UNIQUE INDEX ux_payment_provider_events_provider_webhook_event
    ON payment_provider_events (provider, webhook_event_id)
    WHERE webhook_event_id IS NOT NULL AND btrim(webhook_event_id) <> '';

COMMENT ON COLUMN payment_provider_events.webhook_event_id IS 'Optional PSP delivery id; unique per provider for replay detection alongside provider_ref.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ux_payment_provider_events_provider_webhook_event;

ALTER TABLE payment_provider_events
    DROP COLUMN IF EXISTS webhook_event_id;

-- +goose StatementEnd
