-- name: DeviceCertificateInsert :one
INSERT INTO machine_device_certificates (
    organization_id,
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
    $10,
    $11
)
RETURNING *;

-- name: DeviceCertificateActiveByFingerprint :one
SELECT
    id,
    organization_id,
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
    revoke_reason = $3,
    updated_at = now()
WHERE
    organization_id = $1
    AND fingerprint_sha256 = $2
    AND status IN ('registered', 'active');

-- name: DeviceCertificateSupersede :exec
UPDATE
    machine_device_certificates
SET
    status = 'superseded',
    superseded_by = $3,
    updated_at = now()
WHERE
    id = $1
    AND organization_id = $2
    AND status IN ('registered', 'active');
