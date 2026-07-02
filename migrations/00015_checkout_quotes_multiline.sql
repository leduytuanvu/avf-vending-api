-- +goose Up

CREATE TABLE checkout_quotes (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE RESTRICT,
    currency text NOT NULL,
    payment_method text NOT NULL DEFAULT 'cash',
    subtotal_minor bigint NOT NULL,
    discount_minor bigint NOT NULL DEFAULT 0,
    payable_minor bigint NOT NULL,
    state text NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'consumed', 'expired')),
    idempotency_key text,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_checkout_quotes_machine_idempotency
    ON checkout_quotes (machine_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND btrim(idempotency_key) <> '';

CREATE INDEX ix_checkout_quotes_machine_expires ON checkout_quotes (machine_id, expires_at DESC);

CREATE TABLE checkout_quote_lines (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    quote_id uuid NOT NULL REFERENCES checkout_quotes (id) ON DELETE CASCADE,
    line_sequence int NOT NULL,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    slot_config_id uuid,
    cabinet_code text NOT NULL DEFAULT '',
    slot_code text NOT NULL DEFAULT '',
    slot_index int NOT NULL,
    quantity int NOT NULL CHECK (quantity > 0),
    unit_price_minor bigint NOT NULL,
    line_subtotal_minor bigint NOT NULL,
    pricing_fingerprint text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_checkout_quote_lines_quote_seq UNIQUE (quote_id, line_sequence)
);

CREATE INDEX ix_checkout_quote_lines_quote ON checkout_quote_lines (quote_id, line_sequence ASC);

ALTER TABLE vend_sessions
    ADD COLUMN IF NOT EXISTS line_sequence int NOT NULL DEFAULT 1;

CREATE UNIQUE INDEX IF NOT EXISTS ux_vend_sessions_order_line_seq
    ON vend_sessions (order_id, line_sequence);

-- +goose Down

DROP INDEX IF EXISTS ux_vend_sessions_order_line_seq;
ALTER TABLE vend_sessions DROP COLUMN IF EXISTS line_sequence;
DROP TABLE IF EXISTS checkout_quote_lines;
DROP TABLE IF EXISTS checkout_quotes;
