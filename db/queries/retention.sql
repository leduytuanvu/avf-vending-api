-- Enterprise retention candidate counts (match DELETE predicates in internal/modules/postgres/enterprise_retention.go).

-- name: EnterpriseRetentionCountPublishedOutbox :one
SELECT
    count(*)::bigint
FROM
    outbox_events
WHERE
    status = 'published'
    AND published_at IS NOT NULL
    AND published_at < $1;

-- name: EnterpriseRetentionCountResolvedPaymentProviderEvents :one
SELECT
    count(*)::bigint
FROM
    payment_provider_events e
    JOIN payments p ON p.id = e.payment_id
WHERE
    e.received_at < $1
    AND e.legal_hold = FALSE
    AND p.state IN ('captured', 'failed', 'expired', 'canceled', 'refunded', 'partially_refunded')
    AND p.reconciliation_status IN ('matched', 'not_required');

-- name: EnterpriseRetentionCountDeviceCommandReceipts :one
SELECT
    count(*)::bigint
FROM
    device_command_receipts r
WHERE
    r.received_at < $1
    AND NOT EXISTS (
        SELECT
            1
        FROM
            machine_command_attempts a
        WHERE
            a.id = r.command_attempt_id
            AND a.status IN ('pending', 'sent')
    );

-- name: EnterpriseRetentionCountTerminalCommandLedger :one
SELECT
    count(*)::bigint
FROM
    command_ledger c
WHERE
    c.created_at < $1
    AND NOT EXISTS (
        SELECT
            1
        FROM
            machine_command_attempts a
        WHERE
            a.command_id = c.id
            AND a.status IN ('pending', 'sent')
    );

-- name: EnterpriseRetentionCountMessagingConsumerDedupe :one
SELECT
    count(*)::bigint
FROM
    messaging_consumer_dedupe
WHERE
    processed_at < $1;

-- name: EnterpriseRetentionCountInventoryEvents :one
SELECT
    count(*)::bigint
FROM
    inventory_events
WHERE
    occurred_at < $1;

-- name: EnterpriseRetentionCountTerminalOfflineEvents :one
SELECT
    count(*)::bigint
FROM
    machine_offline_events
WHERE
    processing_status IN (
        'processed',
        'succeeded',
        'failed',
        'duplicate',
        'replayed',
        'rejected'
    )
    AND received_at < $1;

-- name: EnterpriseRetentionCountExpiredAuthRefreshTokens :one
SELECT
    count(*)::bigint
FROM
    auth_refresh_tokens t
WHERE
    t.expires_at < $1
    AND (
        t.revoked_at IS NOT NULL
        OR t.last_used_at IS NULL
        OR t.last_used_at < $1
    );

-- name: EnterpriseRetentionCountPasswordResetTokens :one
SELECT
    count(*)::bigint
FROM
    password_reset_tokens t
WHERE
    t.expires_at < $1
    OR (
        t.used_at IS NOT NULL
        AND t.used_at < $1
    );

-- name: EnterpriseRetentionCountAuditEvents :one
SELECT
    count(*)::bigint
FROM
    audit_events
WHERE
    created_at < $1
    AND legal_hold = FALSE;

-- name: EnterpriseRetentionCountAuditLogs :one
SELECT
    count(*)::bigint
FROM
    audit_logs
WHERE
    created_at < $1
    AND legal_hold = FALSE;
