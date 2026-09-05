-- +goose Up
-- Per-machine payment method allowlist (cash + QR providers).

CREATE TABLE machine_payment_methods (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    method_key text NOT NULL CHECK (
        method_key IN ('cash', 'momo', 'zalopay', 'vietqr', 'vnpay', 'shopeepay')
    ),
    enabled boolean NOT NULL DEFAULT true,
    sort_order int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uniq_machine_payment_methods
    ON machine_payment_methods (machine_id, method_key);

CREATE INDEX ix_machine_payment_methods_machine
    ON machine_payment_methods (machine_id);

-- +goose Down
DROP INDEX IF EXISTS ix_machine_payment_methods_machine;
DROP INDEX IF EXISTS uniq_machine_payment_methods;
DROP TABLE IF EXISTS machine_payment_methods;
