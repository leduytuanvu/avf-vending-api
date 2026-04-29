-- name: TelemetryRetentionDeleteNonCriticalDeviceEventsBatch :execrows
DELETE FROM device_telemetry_events
WHERE
    ctid IN (
        SELECT
            d.ctid
        FROM
            device_telemetry_events AS d
        WHERE
            d.received_at < $1
            AND NOT EXISTS (
                SELECT
                    1
                FROM
                    critical_telemetry_event_status AS c
                WHERE
                    c.machine_id = d.machine_id
                    AND c.idempotency_key = d.dedupe_key
            )
        ORDER BY
            d.received_at ASC
        LIMIT $2
    );

-- name: TelemetryRetentionDeleteCriticalLinkedDeviceEventsBatch :execrows
DELETE FROM device_telemetry_events
WHERE
    ctid IN (
        SELECT
            d.ctid
        FROM
            device_telemetry_events AS d
        WHERE
            d.received_at < $1
            AND EXISTS (
                SELECT
                    1
                FROM
                    critical_telemetry_event_status AS c
                WHERE
                    c.machine_id = d.machine_id
                    AND c.idempotency_key = d.dedupe_key
            )
        ORDER BY
            d.received_at ASC
        LIMIT $2
    );

-- name: TelemetryRetentionDeleteCriticalTelemetryStatusBatch :execrows
DELETE FROM critical_telemetry_event_status
WHERE
    ctid IN (
        SELECT
            s.ctid
        FROM
            critical_telemetry_event_status AS s
        WHERE
            COALESCE(s.processed_at, s.accepted_at, s.updated_at) < $1
        ORDER BY
            COALESCE(s.processed_at, s.accepted_at, s.updated_at) ASC
        LIMIT $2
    );

-- name: TelemetryRetentionDeleteMachineCheckInsBatch :execrows
DELETE FROM machine_check_ins
WHERE
    ctid IN (
        SELECT
            m.ctid
        FROM
            machine_check_ins AS m
        WHERE
            m.occurred_at < $1
        ORDER BY
            m.occurred_at ASC
        LIMIT $2
    );

-- name: TelemetryRetentionDeleteStateTransitionsBatch :execrows
DELETE FROM machine_state_transitions
WHERE
    ctid IN (
        SELECT
            t.ctid
        FROM
            machine_state_transitions AS t
        WHERE
            t.occurred_at < $1
        ORDER BY
            t.occurred_at ASC
        LIMIT $2
    );

-- name: TelemetryRetentionDeleteIncidentsNormalSeverityBatch :execrows
DELETE FROM machine_incidents
WHERE
    ctid IN (
        SELECT
            mi.ctid
        FROM
            machine_incidents AS mi
        WHERE
            mi.opened_at < $1
            AND mi.severity NOT IN ('high', 'critical')
        ORDER BY
            mi.opened_at ASC
        LIMIT $2
    );

-- name: TelemetryRetentionDeleteIncidentsCriticalSeverityBatch :execrows
DELETE FROM machine_incidents
WHERE
    ctid IN (
        SELECT
            mi.ctid
        FROM
            machine_incidents AS mi
        WHERE
            mi.opened_at < $1
            AND mi.severity IN ('high', 'critical')
        ORDER BY
            mi.opened_at ASC
        LIMIT $2
    );

-- name: TelemetryRetentionDeleteRollupsOneMinuteBatch :execrows
DELETE FROM telemetry_rollups
WHERE
    ctid IN (
        SELECT
            r.ctid
        FROM
            telemetry_rollups AS r
        WHERE
            r.granularity = '1m'
            AND r.bucket_start < $1
        ORDER BY
            r.bucket_start ASC
        LIMIT $2
    );

-- name: TelemetryRetentionDeleteRollupsOneHourBatch :execrows
DELETE FROM telemetry_rollups
WHERE
    ctid IN (
        SELECT
            r.ctid
        FROM
            telemetry_rollups AS r
        WHERE
            r.granularity = '1h'
            AND r.bucket_start < $1
        ORDER BY
            r.bucket_start ASC
        LIMIT $2
    );

-- name: TelemetryRetentionDeleteDiagnosticManifestsBatch :execrows
DELETE FROM diagnostic_bundle_manifests
WHERE
    ctid IN (
        SELECT
            d.ctid
        FROM
            diagnostic_bundle_manifests AS d
        WHERE
            d.created_at < $1
        ORDER BY
            d.created_at ASC
        LIMIT $2
    );

-- Candidate counts for dry-run / stats (same predicates as deletes above; no LIMIT).

-- name: TelemetryRetentionCountNonCriticalDeviceEvents :one
SELECT
    count(*)::bigint
FROM
    device_telemetry_events AS d
WHERE
    d.received_at < $1
    AND NOT EXISTS (
        SELECT
            1
        FROM
            critical_telemetry_event_status AS c
        WHERE
            c.machine_id = d.machine_id
            AND c.idempotency_key = d.dedupe_key
    );

-- name: TelemetryRetentionCountCriticalLinkedDeviceEvents :one
SELECT
    count(*)::bigint
FROM
    device_telemetry_events AS d
WHERE
    d.received_at < $1
    AND EXISTS (
        SELECT
            1
        FROM
            critical_telemetry_event_status AS c
        WHERE
            c.machine_id = d.machine_id
            AND c.idempotency_key = d.dedupe_key
    );

-- name: TelemetryRetentionCountCriticalTelemetryStatusRows :one
SELECT
    count(*)::bigint
FROM
    critical_telemetry_event_status AS s
WHERE
    COALESCE(s.processed_at, s.accepted_at, s.updated_at) < $1;

-- name: TelemetryRetentionCountMachineCheckIns :one
SELECT
    count(*)::bigint
FROM
    machine_check_ins AS m
WHERE
    m.occurred_at < $1;

-- name: TelemetryRetentionCountStateTransitions :one
SELECT
    count(*)::bigint
FROM
    machine_state_transitions AS t
WHERE
    t.occurred_at < $1;

-- name: TelemetryRetentionCountIncidentsNormalSeverity :one
SELECT
    count(*)::bigint
FROM
    machine_incidents AS mi
WHERE
    mi.opened_at < $1
    AND mi.severity NOT IN ('high', 'critical');

-- name: TelemetryRetentionCountIncidentsCriticalSeverity :one
SELECT
    count(*)::bigint
FROM
    machine_incidents AS mi
WHERE
    mi.opened_at < $1
    AND mi.severity IN ('high', 'critical');

-- name: TelemetryRetentionCountRollupsOneMinute :one
SELECT
    count(*)::bigint
FROM
    telemetry_rollups AS r
WHERE
    r.granularity = '1m'
    AND r.bucket_start < $1;

-- name: TelemetryRetentionCountRollupsOneHour :one
SELECT
    count(*)::bigint
FROM
    telemetry_rollups AS r
WHERE
    r.granularity = '1h'
    AND r.bucket_start < $1;

-- name: TelemetryRetentionCountDiagnosticManifests :one
SELECT
    count(*)::bigint
FROM
    diagnostic_bundle_manifests AS d
WHERE
    d.created_at < $1;
