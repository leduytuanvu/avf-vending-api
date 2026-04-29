-- name: UpsertMachineIdempotencyKey :one
INSERT INTO machine_idempotency_keys (
    organization_id,
    machine_id,
    operation,
    idempotency_key,
    request_hash,
    expires_at,
    trace_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (organization_id, machine_id, operation, idempotency_key)
DO UPDATE SET
    last_seen_at = now(),
    expires_at = excluded.expires_at,
    trace_id = excluded.trace_id
RETURNING *, (xmax = 0)::boolean AS inserted;

-- name: DeleteStaleMachineIdempotencyInProgress :exec
DELETE FROM machine_idempotency_keys
WHERE
    organization_id = $1
    AND machine_id = $2
    AND operation = $3
    AND idempotency_key = $4
    AND status = 'in_progress'
    AND last_seen_at < $5;

-- name: MarkMachineIdempotencySucceeded :one
UPDATE machine_idempotency_keys
SET
    status = 'succeeded',
    response_snapshot = $5,
    last_seen_at = now(),
    trace_id = $6
WHERE
    organization_id = $1
    AND machine_id = $2
    AND operation = $3
    AND idempotency_key = $4
RETURNING *;

-- name: MarkMachineIdempotencyFailed :exec
UPDATE machine_idempotency_keys
SET
    status = 'failed',
    last_seen_at = now(),
    trace_id = $5
WHERE
    organization_id = $1
    AND machine_id = $2
    AND operation = $3
    AND idempotency_key = $4
    AND status = 'in_progress';

-- name: MarkMachineIdempotencyConflict :exec
UPDATE machine_idempotency_keys
SET
    status = 'conflict',
    last_seen_at = now()
WHERE
    organization_id = $1
    AND machine_id = $2
    AND operation = $3
    AND idempotency_key = $4;
