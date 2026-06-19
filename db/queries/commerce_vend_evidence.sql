-- name: InsertVendHardwareEvidence :one
INSERT INTO vend_hardware_evidence (
    order_id,
    vend_session_id,
    machine_id,
    slot_index,
    vend_attempt_id,
    correlation_id,
    command_id,
    evidence_digest,
    raw,
    dedupe_key
)
VALUES (
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
ON CONFLICT (dedupe_key) DO NOTHING
RETURNING *;

-- name: GetVendHardwareEvidenceByDedupeKey :one
SELECT *
FROM vend_hardware_evidence
WHERE dedupe_key = $1;

-- name: SetVendSessionVerificationStatus :one
UPDATE vend_sessions
SET verification_status = $1
WHERE
    order_id = $2
    AND slot_index = $3
RETURNING *;
