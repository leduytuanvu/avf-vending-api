-- name: DeviceGetAssignedOTAUpdate :one
SELECT
    r.campaign_id,
    c.artifact_id,
    COALESCE(c.artifact_version, a.semver, '')::text AS artifact_version,
    c.campaign_type,
    a.storage_key,
    a.sha256 AS sha256_hex,
    a.size_bytes,
    r.status,
    r.updated_at
FROM ota_machine_results r
INNER JOIN ota_campaigns c ON c.id = r.campaign_id
INNER JOIN ota_artifacts a ON a.id = c.artifact_id
WHERE
    r.machine_id = $1
    AND c.organization_id = $2
    AND r.wave = 'forward'
    AND r.status IN ('dispatched', 'acked', 'downloaded', 'failed')
    AND c.status IN ('running', 'completed')
ORDER BY r.updated_at DESC
LIMIT 1;

-- name: DeviceReportOTAResult :one
UPDATE ota_machine_results r
SET
    status = $4,
    last_error = $5,
    updated_at = now()
FROM ota_campaigns c
WHERE
    r.campaign_id = c.id
    AND r.machine_id = $1
    AND r.campaign_id = $2
    AND c.organization_id = $3
RETURNING
    r.id,
    r.organization_id,
    r.campaign_id,
    r.machine_id,
    r.wave,
    r.command_id,
    r.status,
    r.last_error,
    r.updated_at,
    r.created_at;

-- name: DeviceInsertDiagnosticBundleManifest :one
INSERT INTO diagnostic_bundle_manifests (
    organization_id,
    machine_id,
    request_id,
    command_id,
    storage_key,
    storage_provider,
    content_type,
    size_bytes,
    sha256_hex,
    metadata,
    status,
    expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT ON CONSTRAINT ux_diagnostic_bundle_manifests_machine_request DO UPDATE
SET
    command_id = COALESCE(EXCLUDED.command_id, diagnostic_bundle_manifests.command_id),
    storage_key = EXCLUDED.storage_key,
    storage_provider = EXCLUDED.storage_provider,
    content_type = EXCLUDED.content_type,
    size_bytes = EXCLUDED.size_bytes,
    sha256_hex = EXCLUDED.sha256_hex,
    metadata = EXCLUDED.metadata,
    status = EXCLUDED.status,
    expires_at = EXCLUDED.expires_at
RETURNING *;

-- name: DeviceListDiagnosticBundleManifests :many
SELECT
    d.id,
    d.organization_id,
    d.machine_id,
    d.request_id,
    d.command_id,
    d.storage_key,
    d.storage_provider,
    d.content_type,
    d.size_bytes,
    d.sha256_hex,
    d.metadata,
    d.status,
    d.created_at,
    d.expires_at
FROM diagnostic_bundle_manifests d
INNER JOIN machines m ON m.id = d.machine_id
WHERE
    d.organization_id = $1
    AND d.machine_id = $2
    AND m.organization_id = $1
ORDER BY d.created_at DESC
LIMIT $3 OFFSET $4;
