-- Add partially_completed terminal order status for multi-line partial vend outcomes.
-- +goose Up
-- +goose StatementBegin

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;

ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (
    status IN (
        'created',
        'quoted',
        'paid',
        'vending',
        'completed',
        'partially_completed',
        'failed',
        'cancelled'
    )
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;

ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (
    status IN (
        'created',
        'quoted',
        'paid',
        'vending',
        'completed',
        'failed',
        'cancelled'
    )
);

-- +goose StatementEnd
