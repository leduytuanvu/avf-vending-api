-- +goose Up
-- +goose StatementBegin

-- Current machine snapshot (projected from state stream; not raw MQTT history).
CREATE TABLE machine_current_snapshot (
    machine_id uuid PRIMARY KEY REFERENCES machines (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    site_id uuid NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    reported_fingerprint text,
    metrics_fingerprint text,
    reported_state jsonb NOT NULL DEFAULT '{}'::jsonb,
    metrics_state jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_heartbeat_at timestamptz,
    app_version text,
    firmware_version text,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_machine_current_snapshot_org ON machine_current_snapshot (organization_id);

COMMENT ON TABLE machine_current_snapshot IS 'Single current row per machine; updated by telemetry state/metrics workers — not a raw ingest log.';

-- Meaningful state changes only (worker inserts when fingerprint / semantic diff).
CREATE TABLE machine_state_transitions (
    id bigserial PRIMARY KEY,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    transition_key text NOT NULL,
    from_value jsonb,
    to_value jsonb NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_machine_state_transitions_machine_occurred ON machine_state_transitions (machine_id, occurred_at DESC);

COMMENT ON TABLE machine_state_transitions IS 'Append-only semantic transitions derived from shadow/state stream; pruned by retention job.';

-- Persisted incidents (from incident class + worker classification).
CREATE TABLE machine_incidents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    severity text NOT NULL,
    code text NOT NULL,
    title text,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text,
    opened_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_machine_incidents_machine_dedupe ON machine_incidents (machine_id, dedupe_key)
WHERE
    dedupe_key IS NOT NULL
    AND btrim(dedupe_key) <> '';

CREATE INDEX ix_machine_incidents_machine_opened ON machine_incidents (machine_id, opened_at DESC);

COMMENT ON TABLE machine_incidents IS 'Operational/security incidents promoted from telemetry; not raw high-frequency logs.';

-- Rollups only (1m and coarse); no raw heartbeat/metrics rows here.
CREATE TABLE telemetry_rollups (
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    bucket_start timestamptz NOT NULL,
    granularity text NOT NULL CHECK (granularity IN ('1m', '1h')),
    metric_key text NOT NULL,
    sample_count bigint NOT NULL DEFAULT 0,
    sum_val double precision,
    min_val double precision,
    max_val double precision,
    last_val double precision,
    extra jsonb NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (machine_id, bucket_start, granularity, metric_key)
);

CREATE INDEX ix_telemetry_rollups_machine_bucket ON telemetry_rollups (machine_id, bucket_start DESC);

COMMENT ON TABLE telemetry_rollups IS 'Aggregated telemetry; workers upsert buckets — raw MQTT metrics are not stored in Postgres.';

-- Diagnostic / cold-path manifests (bytes live in object storage).
CREATE TABLE diagnostic_bundle_manifests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    storage_key text NOT NULL,
    storage_provider text NOT NULL DEFAULT 's3',
    content_type text,
    size_bytes bigint,
    sha256_hex text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz
);

CREATE INDEX ix_diagnostic_bundle_manifests_machine_created ON diagnostic_bundle_manifests (machine_id, created_at DESC);

COMMENT ON TABLE diagnostic_bundle_manifests IS 'Metadata for cold diagnostic bundles; blobs referenced by storage_key only.';

COMMENT ON TABLE device_telemetry_events IS 'Legacy row-per-event table. High-frequency telemetry should use NATS JetStream + rollups when TELEMETRY_NATS_BRIDGE_ENABLED; avoid new reliance on this table for heartbeats/metrics at scale.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS diagnostic_bundle_manifests;
DROP TABLE IF EXISTS telemetry_rollups;
DROP TABLE IF EXISTS machine_incidents;
DROP TABLE IF EXISTS machine_state_transitions;
DROP TABLE IF EXISTS machine_current_snapshot;
-- +goose StatementEnd
