-- name: GetMachineSyncCursor :one
SELECT
    *
FROM
    machine_sync_cursors
WHERE
    organization_id = $1
    AND machine_id = $2
    AND stream_name = $3;

-- name: UpsertMachineSyncCursor :one
INSERT INTO
    machine_sync_cursors (
        organization_id,
        machine_id,
        stream_name,
        last_sequence,
        last_synced_at
    )
VALUES
    ($1, $2, $3, $4, now())
ON CONFLICT (organization_id, machine_id, stream_name) DO UPDATE
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
    organization_id = $1
    AND machine_id = $2
    AND client_event_id = $3
    AND btrim(client_event_id) <> '';

-- name: InsertMachineOfflineEvent :one
INSERT INTO
    machine_offline_events (
        organization_id,
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
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (organization_id, machine_id, offline_sequence)
DO UPDATE SET
    received_at = now()
RETURNING
    id,
    organization_id,
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
    processing_status = $4,
    processing_error = $5
WHERE
    organization_id = $1
    AND machine_id = $2
    AND offline_sequence = $3;
