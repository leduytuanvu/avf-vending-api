-- Additive simulation metadata for enterprise HIL — excludes simulated rows from revenue when filtered.
-- +goose Up
-- +goose StatementBegin
ALTER TABLE orders ADD COLUMN IF NOT EXISTS simulated boolean NOT NULL DEFAULT false;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS simulation_run_id text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS simulation_scenario text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS fake_bill boolean NOT NULL DEFAULT false;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS fake_board boolean NOT NULL DEFAULT false;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS simulation_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE vend_sessions ADD COLUMN IF NOT EXISTS simulated boolean NOT NULL DEFAULT false;
ALTER TABLE vend_sessions ADD COLUMN IF NOT EXISTS simulation_run_id text;
ALTER TABLE vend_sessions ADD COLUMN IF NOT EXISTS simulation_scenario text;
ALTER TABLE vend_sessions ADD COLUMN IF NOT EXISTS simulation_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE payments ADD COLUMN IF NOT EXISTS simulated boolean NOT NULL DEFAULT false;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS simulation_run_id text;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS simulation_scenario text;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS fake_bill boolean NOT NULL DEFAULT false;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS fake_board boolean NOT NULL DEFAULT false;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS simulation_metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS simulated boolean NOT NULL DEFAULT false;
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS simulation_run_id text;
ALTER TABLE outbox_events ADD COLUMN IF NOT EXISTS simulation_scenario text;

CREATE INDEX IF NOT EXISTS ix_orders_simulated ON orders (simulated) WHERE simulated = true;
CREATE INDEX IF NOT EXISTS ix_payments_simulated ON payments (simulated) WHERE simulated = true;
CREATE INDEX IF NOT EXISTS ix_vend_sessions_simulated ON vend_sessions (simulated) WHERE simulated = true;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS ix_vend_sessions_simulated;
DROP INDEX IF EXISTS ix_payments_simulated;
DROP INDEX IF EXISTS ix_orders_simulated;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS simulation_scenario;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS simulation_run_id;
ALTER TABLE outbox_events DROP COLUMN IF EXISTS simulated;

ALTER TABLE payments DROP COLUMN IF EXISTS simulation_metadata;
ALTER TABLE payments DROP COLUMN IF EXISTS fake_board;
ALTER TABLE payments DROP COLUMN IF EXISTS fake_bill;
ALTER TABLE payments DROP COLUMN IF EXISTS simulation_scenario;
ALTER TABLE payments DROP COLUMN IF EXISTS simulation_run_id;
ALTER TABLE payments DROP COLUMN IF EXISTS simulated;

ALTER TABLE vend_sessions DROP COLUMN IF EXISTS simulation_metadata;
ALTER TABLE vend_sessions DROP COLUMN IF EXISTS simulation_scenario;
ALTER TABLE vend_sessions DROP COLUMN IF EXISTS simulation_run_id;
ALTER TABLE vend_sessions DROP COLUMN IF EXISTS simulated;

ALTER TABLE orders DROP COLUMN IF EXISTS simulation_metadata;
ALTER TABLE orders DROP COLUMN IF EXISTS fake_board;
ALTER TABLE orders DROP COLUMN IF EXISTS fake_bill;
ALTER TABLE orders DROP COLUMN IF EXISTS simulation_scenario;
ALTER TABLE orders DROP COLUMN IF EXISTS simulation_run_id;
ALTER TABLE orders DROP COLUMN IF EXISTS simulated;
-- +goose StatementEnd
