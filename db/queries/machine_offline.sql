-- name: GetMachineSyncCursor :one
SELECT
    *
FROM
    machine_sync_cursors
WHERE
    machine_id = $1
    AND stream_name = $2;

-- name: UpsertMachineSyncCursor :one
INSERT INTO
    machine_sync_cursors (
    machine_id,
    stream_name,
    last_sequence,
    last_synced_at
)
VALUES
    (
    $1,
    $2,
    $3,
    now()
)
ON CONFLICT (machine_id, stream_name) DO UPDATE
SET
    last_sequence = GREATEST(machine_sync_cursors.last_sequence, EXCLUDED.last_sequence),
    last_synced_at = now(),
    updated_at = now()
RETURNING
    *;

-- name: GetMachineOfflineEventByClientEventID :one
SELECT
    *
FROM
    machine_offline_events
WHERE
    machine_id = $1
    AND client_event_id = $2
    AND btrim(client_event_id) <> '';

-- name: InsertMachineOfflineEvent :one
INSERT INTO
    machine_offline_events (
    machine_id,
    offline_sequence,
    event_type,
    event_id,
    client_event_id,
    occurred_at,
    payload,
    processing_status,
    processing_error,
    idempotency_key
)
VALUES
    (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10
)
ON CONFLICT (machine_id, offline_sequence)
DO UPDATE SET
    received_at = now()
RETURNING
    id,
    machine_id,
    offline_sequence,
    event_type,
    event_id,
    client_event_id,
    occurred_at,
    received_at,
    payload,
    processing_status,
    processing_error,
    idempotency_key,
    (xmax = 0)::boolean AS inserted;

-- name: UpdateMachineOfflineEventStatus :exec
UPDATE machine_offline_events
SET
    processing_status = $1,
    processing_error = $2
WHERE
    machine_id = $3
    AND offline_sequence = $4;
