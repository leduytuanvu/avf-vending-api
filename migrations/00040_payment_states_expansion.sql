-- +goose Up
-- Expand payments.state for enterprise PSP lifecycle (webhooks + reconciliation).

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_state_check;

ALTER TABLE payments
ADD CONSTRAINT payments_state_check CHECK (
    state IN (
        'created',
        'authorized',
        'captured',
        'failed',
        'expired',
        'canceled',
        'refunded',
        'partially_refunded'
    )
);

-- +goose Down

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_state_check;

ALTER TABLE payments
ADD CONSTRAINT payments_state_check CHECK (
    state IN ('created', 'authorized', 'captured', 'failed', 'refunded')
);
