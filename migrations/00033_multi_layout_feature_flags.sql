-- Seed multi-layout rollout feature flags (disabled by default for fleet canary).
-- +goose Up
-- +goose StatementBegin

INSERT INTO feature_flags (flag_key, display_name, description, enabled, metadata)
VALUES
    (
        'multi_layout_enabled',
        'Multi-layout machine library',
        'Enable named layouts per machine on device and web.',
        false,
        '{}'::jsonb
    ),
    (
        'snapshot_history_ingest_enabled',
        'Layout snapshot history ingest',
        'Accept immutable ReportLayoutSnapshot history on API.',
        true,
        '{}'::jsonb
    ),
    (
        'periodic_snapshot_capture_enabled',
        'Periodic layout snapshot capture',
        'Capture active layout snapshots every 30 minutes on device.',
        false,
        '{}'::jsonb
    ),
    (
        'hardware_reconcile_on_activate_enabled',
        'TCN reconcile on layout activation',
        'Run ALL SINGLE + merge pipeline before materializing active layout.',
        false,
        '{}'::jsonb
    ),
    (
        'immediate_planogram_publish_enabled',
        'Immediate planogram publish',
        'Legacy HTTP publish on every planogram edit; disable for local-first rollout.',
        true,
        '{}'::jsonb
    )
ON CONFLICT (flag_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM feature_flags
WHERE flag_key IN (
    'multi_layout_enabled',
    'snapshot_history_ingest_enabled',
    'periodic_snapshot_capture_enabled',
    'hardware_reconcile_on_activate_enabled',
    'immediate_planogram_publish_enabled'
);
-- +goose StatementEnd
