-- +goose Up
-- ON CONFLICT target for UpsertMachineIdempotencyKey (machine_id, operation, idempotency_key)
CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_idempotency_machine_op_key ON machine_idempotency_keys (machine_id, operation, idempotency_key);

-- +goose Down
DROP INDEX IF EXISTS ux_machine_idempotency_machine_op_key;
