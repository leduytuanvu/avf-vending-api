-- name: GetCommandLedgerByMachineSequence :one
SELECT
    id,
    machine_id,
    sequence,
    command_type,
    payload,
    correlation_id,
    idempotency_key,
    created_at,
    protocol_type,
    deadline_at,
    timeout_at,
    attempt_count,
    last_attempt_at,
    route_key,
    source_system,
    source_event_id,
    operator_session_id
FROM command_ledger
WHERE
    machine_id = $1
    AND sequence = $2;

-- name: GetLatestMachineCommandAttemptByCommandID :one
SELECT
    id,
    command_id,
    machine_id,
    transport_session_id,
    attempt_no,
    sent_at,
    ack_deadline_at,
    acked_at,
    result_received_at,
    status,
    timeout_reason,
    protocol_pack_no,
    sequence_no,
    correlation_id,
    request_payload_json,
    raw_request,
    raw_response,
    latency_ms
FROM machine_command_attempts
WHERE command_id = $1
ORDER BY attempt_no DESC
LIMIT 1;

-- name: GetLatestOpenMachineCommandAttemptForCommand :one
SELECT
    id,
    command_id,
    machine_id,
    transport_session_id,
    attempt_no,
    sent_at,
    ack_deadline_at,
    acked_at,
    result_received_at,
    status,
    timeout_reason,
    protocol_pack_no,
    sequence_no,
    correlation_id,
    request_payload_json,
    raw_request,
    raw_response,
    latency_ms
FROM machine_command_attempts
WHERE
    command_id = $1
    AND status IN ('pending', 'sent')
ORDER BY attempt_no DESC
LIMIT 1;

-- name: InsertMachineCommandAttempt :one
INSERT INTO machine_command_attempts (
    command_id,
    machine_id,
    attempt_no,
    sent_at,
    status,
    correlation_id,
    request_payload_json
)
VALUES (
    $1,
    $2,
    (
        SELECT COALESCE(MAX(attempt_no), 0) + 1
        FROM machine_command_attempts mc
        WHERE
            mc.command_id = $1
    ),
    now(),
    'pending',
    $3,
    $4
)
RETURNING
    id,
    command_id,
    machine_id,
    transport_session_id,
    attempt_no,
    sent_at,
    ack_deadline_at,
    acked_at,
    result_received_at,
    status,
    timeout_reason,
    protocol_pack_no,
    sequence_no,
    correlation_id,
    request_payload_json,
    raw_request,
    raw_response,
    latency_ms;

-- name: UpdateCommandLedgerMQTTDispatchMeta :exec
UPDATE command_ledger
SET
    protocol_type = COALESCE(protocol_type, 'mqtt'),
    timeout_at = $2,
    attempt_count = attempt_count + 1,
    last_attempt_at = now()
WHERE
    id = $1;

-- name: UpdateMachineCommandAttemptAfterDeviceReceipt :exec
UPDATE machine_command_attempts
SET
    status = $2,
    result_received_at = now(),
    acked_at = CASE
        WHEN $2 = 'completed' THEN now()
        ELSE acked_at
    END
WHERE
    id = $1
    AND status IN ('pending', 'sent');

-- name: UpdateMachineCommandAttemptSent :exec
UPDATE machine_command_attempts
SET
    status = 'sent',
    ack_deadline_at = $2
WHERE
    id = $1
    AND status = 'pending';

-- name: UpdateMachineCommandAttemptPublishFailed :exec
UPDATE machine_command_attempts
SET
    status = 'failed',
    result_received_at = now(),
    timeout_reason = $2
WHERE
    id = $1
    AND status = 'pending';

-- name: ApplyMachineCommandAckTimeouts :exec
UPDATE machine_command_attempts
SET
    status = 'ack_timeout',
    result_received_at = now(),
    timeout_reason = 'ack_deadline_exceeded'
WHERE
    status = 'sent'
    AND ack_deadline_at IS NOT NULL
    AND ack_deadline_at < $1;

-- name: ListDeviceCommandReceiptsByMachine :many
SELECT
    id,
    machine_id,
    sequence,
    status,
    correlation_id,
    payload,
    dedupe_key,
    received_at,
    command_attempt_id
FROM device_command_receipts
WHERE
    machine_id = $1
ORDER BY received_at DESC
LIMIT $2;
