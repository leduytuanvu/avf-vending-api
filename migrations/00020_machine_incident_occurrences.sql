-- Machine incident occurrence ledger + alert clock for Telegram every-occurrence mode.
-- +goose Up
-- +goose StatementBegin

ALTER TABLE machine_incidents
    ADD COLUMN IF NOT EXISTS occurrence_count bigint NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS last_alerted_at timestamptz NULL;

COMMENT ON COLUMN machine_incidents.occurrence_count IS 'Count of distinct logical occurrences for this grouped incident (fingerprint/dedupe_key).';
COMMENT ON COLUMN machine_incidents.last_alerted_at IS 'When a Telegram notification intent was last queued; independent of updated_at/last_seen.';

CREATE TABLE machine_incident_occurrences (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    machine_incident_id uuid NULL REFERENCES machine_incidents (id) ON DELETE SET NULL,
    occurrence_id text NOT NULL,
    dedupe_key text NOT NULL,
    severity text NOT NULL,
    code text NOT NULL,
    title text,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_transport text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL DEFAULT now(),
    received_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_machine_incident_occurrences_occurrence_id_nonempty CHECK (btrim(occurrence_id) <> ''),
    CONSTRAINT ck_machine_incident_occurrences_dedupe_nonempty CHECK (btrim(dedupe_key) <> '')
);

CREATE UNIQUE INDEX ux_machine_incident_occurrences_machine_occurrence
    ON machine_incident_occurrences (machine_id, occurrence_id);

CREATE INDEX ix_machine_incident_occurrences_machine_received
    ON machine_incident_occurrences (machine_id, received_at DESC);

CREATE INDEX ix_machine_incident_occurrences_incident
    ON machine_incident_occurrences (machine_incident_id)
WHERE
    machine_incident_id IS NOT NULL;

COMMENT ON TABLE machine_incident_occurrences IS 'One row per logical App/backend machine incident occurrence; cross-transport dedupe by (machine_id, occurrence_id).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS machine_incident_occurrences;

ALTER TABLE machine_incidents
    DROP COLUMN IF EXISTS last_alerted_at,
    DROP COLUMN IF EXISTS occurrence_count;

-- +goose StatementEnd
