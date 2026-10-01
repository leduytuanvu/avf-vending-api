-- Add partially_completed terminal order status for multi-line partial vend outcomes.
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
