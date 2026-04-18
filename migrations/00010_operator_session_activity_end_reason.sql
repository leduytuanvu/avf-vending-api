-- +goose Up
-- +goose StatementBegin
-- last_activity_at: NOT NULL + DEFAULT now() uses PG11+ fast path (no full-table rewrite) when adding the column.
ALTER TABLE machine_operator_sessions
ADD COLUMN IF NOT EXISTS last_activity_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE machine_operator_sessions
ADD COLUMN IF NOT EXISTS ended_reason text;

COMMENT ON COLUMN machine_operator_sessions.last_activity_at IS 'Last client heartbeat or successful session activity; updated independently of generic updated_at when desired.';
COMMENT ON COLUMN machine_operator_sessions.ended_reason IS 'Optional free-text or stable code describing why the session ended (e.g. user_logout, timeout).';

ALTER TABLE machine_operator_auth_events
ALTER COLUMN operator_session_id DROP NOT NULL;

COMMENT ON COLUMN machine_operator_auth_events.operator_session_id IS 'NULL allowed for machine-scoped login_failure before a session row exists.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $do$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM machine_operator_auth_events
        WHERE operator_session_id IS NULL
    ) THEN
        RAISE EXCEPTION 'goose down blocked: machine_operator_auth_events contains NULL operator_session_id; delete or backfill those rows before reverting 00010_operator_session_activity_end_reason';
    END IF;
END
$do$;

ALTER TABLE machine_operator_auth_events
ALTER COLUMN operator_session_id SET NOT NULL;

ALTER TABLE machine_operator_sessions
DROP COLUMN IF EXISTS ended_reason;

ALTER TABLE machine_operator_sessions
DROP COLUMN IF EXISTS last_activity_at;

-- +goose StatementEnd
