-- name: UpsertMachineIdempotencyKey :one
INSERT INTO machine_idempotency_keys (
    machine_id,
    operation,
    idempotency_key,
    request_hash,
    expires_at,
    trace_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
ON CONFLICT (machine_id, operation, idempotency_key)
DO UPDATE SET
    last_seen_at = now(),
    expires_at = excluded.expires_at,
    trace_id = excluded.trace_id
RETURNING *, (xmax = 0)::boolean AS inserted;

-- name: DeleteStaleMachineIdempotencyInProgress :exec
DELETE FROM machine_idempotency_keys
WHERE
    machine_id = $1
    AND operation = $2
    AND idempotency_key = $3
    AND status = 'in_progress'
    AND last_seen_at < $4;

-- name: MarkMachineIdempotencySucceeded :one
UPDATE machine_idempotency_keys
SET
    status = 'succeeded',
    response_snapshot = NULLIF(sqlc.arg('response_snapshot')::text, '')::jsonb,
    last_seen_at = now(),
    trace_id = sqlc.arg('trace_id')
WHERE
    machine_id = sqlc.arg('machine_id')
    AND operation = sqlc.arg('operation')
    AND idempotency_key = sqlc.arg('idempotency_key')
RETURNING *;

-- name: MarkMachineIdempotencyFailed :exec
UPDATE machine_idempotency_keys
SET
    status = 'failed',
    last_seen_at = now(),
    trace_id = $1
WHERE
    machine_id = $2
    AND operation = $3
    AND idempotency_key = $4
    AND status = 'in_progress';

-- name: MarkMachineIdempotencyConflict :exec
UPDATE machine_idempotency_keys
SET
    status = 'conflict',
    last_seen_at = now()
WHERE
    machine_id = $1
    AND operation = $2
    AND idempotency_key = $3;
