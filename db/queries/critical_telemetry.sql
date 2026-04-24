-- name: UpsertCriticalTelemetryEventStatus :exec
INSERT INTO critical_telemetry_event_status (
    machine_id,
    idempotency_key,
    status,
    event_type,
    accepted_at,
    processed_at
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (machine_id, idempotency_key) DO UPDATE
SET
    status = EXCLUDED.status,
    event_type = COALESCE(critical_telemetry_event_status.event_type, EXCLUDED.event_type),
    accepted_at = COALESCE(critical_telemetry_event_status.accepted_at, EXCLUDED.accepted_at),
    processed_at = COALESCE(EXCLUDED.processed_at, critical_telemetry_event_status.processed_at),
    updated_at = now();

-- name: GetCriticalTelemetryEventStatus :one
SELECT
    machine_id,
    idempotency_key,
    status,
    event_type,
    accepted_at,
    processed_at,
    updated_at
FROM
    critical_telemetry_event_status
WHERE
    machine_id = $1
    AND idempotency_key = $2;
