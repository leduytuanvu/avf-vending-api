-- name: GetCommandLedgerByMachineSequence :one
SELECT
    id,
    machine_id,
    organization_id,
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
    operator_session_id,
    max_dispatch_attempts
FROM command_ledger
WHERE
    machine_id = $1
    AND sequence = $2;

-- name: GetCommandLedgerByID :one
SELECT
    id,
    machine_id,
    organization_id,
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
    operator_session_id,
    max_dispatch_attempts
FROM command_ledger
WHERE
    id = $1;

-- name: CountMachineCommandAttemptsByCommandID :one
SELECT
    count(*)::bigint AS n
FROM machine_command_attempts
WHERE
    command_id = $1;

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
    last_attempt_at = now(),
    route_key = CASE
        WHEN $3::text IS NOT NULL
        AND btrim($3::text) <> '' THEN $3::text
        ELSE route_key
    END
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

-- name: ApplyMachineCommandAckTimeouts :execrows
UPDATE machine_command_attempts
SET
    status = 'ack_timeout',
    result_received_at = now(),
    timeout_reason = 'ack_deadline_exceeded'
WHERE
    status = 'sent'
    AND ack_deadline_at IS NOT NULL
    AND ack_deadline_at < $1;

-- name: ApplyMachineCommandLedgerExpired :execrows
UPDATE machine_command_attempts AS mca
SET
    status = 'expired',
    result_received_at = now(),
    timeout_reason = 'ledger_timeout_exceeded'
FROM command_ledger AS cl
WHERE
    mca.command_id = cl.id
    AND mca.status = 'sent'
    AND cl.timeout_at IS NOT NULL
    AND cl.timeout_at < $1;

-- name: GetDeviceCommandReceiptIDByDedupeKey :one
SELECT
    id
FROM device_command_receipts
WHERE
    dedupe_key = $1;

-- name: GetLatestDeviceCommandReceiptByMachineSequence :one
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
    AND sequence = $2
ORDER BY
    id DESC
LIMIT 1;

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

-- name: AdminOpsListAttemptsForCommand :many
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
ORDER BY attempt_no ASC;

-- name: AdminOpsCancelOpenAttemptsForCommand :execrows
UPDATE machine_command_attempts
SET
    status = 'failed',
    result_received_at = now(),
    timeout_reason = 'admin_cancelled'
WHERE
    command_id = $1
    AND status IN ('pending', 'sent');

-- name: AdminOpsGetCommandLedgerForOrg :one
SELECT
    cl.id,
    cl.machine_id,
    cl.organization_id,
    cl.sequence,
    cl.command_type,
    cl.payload,
    cl.correlation_id,
    cl.idempotency_key,
    cl.created_at,
    cl.protocol_type,
    cl.deadline_at,
    cl.timeout_at,
    cl.attempt_count,
    cl.last_attempt_at,
    cl.route_key,
    cl.source_system,
    cl.source_event_id,
    cl.operator_session_id,
    cl.max_dispatch_attempts
FROM command_ledger AS cl
INNER JOIN machines AS m ON m.id = cl.machine_id
WHERE
    cl.id = $1
    AND m.organization_id = $2;
