-- Speed idempotency lookups for gap-tolerant offline replay (machine_contract_grpc).
-- +goose Up
-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS ix_machine_offline_events_machine_idempotency ON machine_offline_events (machine_id, idempotency_key)
WHERE
    btrim(idempotency_key) <> '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS ix_machine_offline_events_machine_idempotency;

-- +goose StatementEnd
