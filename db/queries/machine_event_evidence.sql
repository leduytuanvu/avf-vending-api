-- name: InsertMachineEventEvidence :one
INSERT INTO
    machine_event_evidence (
        machine_id,
        event_id,
        event_type,
        schema_version,
        category,
        severity,
        source,
        stream_id,
        client_sequence,
        boot_id,
        occurred_at,
        monotonic_elapsed_ms,
        order_id,
        payment_id,
        vend_attempt_id,
        correlation_id,
        operator_session_id,
        request_id,
        cause,
        recovery_action,
        payload,
        payload_fingerprint,
        processing_status
    )
VALUES
    (
        $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
        $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
        $21, $22, $23
    )
ON CONFLICT (machine_id, event_id) DO UPDATE SET
    received_at = machine_event_evidence.received_at
RETURNING
    id,
    machine_id,
    event_id,
    event_type,
    payload,
    payload_fingerprint,
    processing_status,
    occurred_at,
    received_at,
    (xmax = 0)::boolean AS inserted;

-- name: GetMachineEventEvidence :one
SELECT
    *
FROM
    machine_event_evidence
WHERE
    machine_id = $1
    AND event_id = $2;
