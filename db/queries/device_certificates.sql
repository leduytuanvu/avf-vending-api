-- name: DeviceCertificateInsert :one
INSERT INTO machine_device_certificates (
    machine_id,
    fingerprint_sha256,
    serial_number,
    subject_dn,
    issuer_dn,
    sans_json,
    not_before,
    not_after,
    status,
    metadata
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
RETURNING *;

-- name: DeviceCertificateActiveByFingerprint :one
SELECT
    id,
    machine_id,
    fingerprint_sha256,
    status,
    not_before,
    not_after
FROM
    machine_device_certificates
WHERE
    fingerprint_sha256 = $1
    AND status IN ('registered', 'active')
    AND not_before <= now()
    AND not_after > now();

-- name: DeviceCertificateRevokeByFingerprint :execrows
UPDATE
    machine_device_certificates
SET
    status = 'revoked',
    revoked_at = now(),
    revoke_reason = $1,
    updated_at = now()
WHERE
    fingerprint_sha256 = $2
    AND status IN ('registered', 'active');

-- name: DeviceCertificateSupersede :exec
UPDATE
    machine_device_certificates
SET
    status = 'superseded',
    superseded_by = $1,
    updated_at = now()
WHERE
    id = $2
    AND TRUE
    AND status IN ('registered', 'active');
