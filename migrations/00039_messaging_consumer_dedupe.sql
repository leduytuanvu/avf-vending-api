-- +goose Up
-- +goose StatementBegin
-- JetStream / NATS consumer idempotency (at-least-once delivery): claim once per logical message id.
CREATE TABLE IF NOT EXISTS messaging_consumer_dedupe (
    id bigserial PRIMARY KEY,
    consumer_name text NOT NULL,
    broker_subject text NOT NULL,
    broker_msg_id text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_messaging_consumer_dedupe UNIQUE (consumer_name, broker_subject, broker_msg_id)
);

CREATE INDEX IF NOT EXISTS ix_messaging_consumer_dedupe_processed ON messaging_consumer_dedupe (processed_at);

COMMENT ON TABLE messaging_consumer_dedupe IS 'P1.2: idempotent consumer claims (Nats-Msg-Id / custom dedupe keys).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS messaging_consumer_dedupe;

-- +goose StatementEnd
