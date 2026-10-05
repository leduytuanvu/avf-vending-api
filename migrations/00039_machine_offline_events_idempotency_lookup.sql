CREATE INDEX IF NOT EXISTS ix_machine_offline_events_machine_idempotency ON machine_offline_events (machine_id, idempotency_key)
WHERE
    btrim(idempotency_key) <> '';
