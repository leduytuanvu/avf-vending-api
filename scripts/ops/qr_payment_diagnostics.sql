-- Read-only production diagnostics for QR payment / MoMo IPN incidents.
-- Run with a read-only role; redact connection strings before sharing output.

-- 1. Confirm column types/defaults
SELECT table_name, column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_name IN ('payment_attempts', 'payments', 'payment_provider_events', 'outbox_events')
  AND (data_type IN ('json', 'jsonb') OR column_name IN ('payload', 'provider_metadata'));

-- 2. Payments for a specific order (replace UUID)
-- SELECT id, provider, state, outcome, attempt_seq, supersedes_payment_id, amount_minor, created_at
-- FROM payments WHERE order_id = '00000000-0000-0000-0000-000000000000' ORDER BY created_at;

-- 3. Attempts for that order
-- SELECT pa.* FROM payment_attempts pa
-- JOIN payments p ON p.id = pa.payment_id
-- WHERE p.order_id = '00000000-0000-0000-0000-000000000000';

-- 4. Fleet-wide stranded created payments without attempts (30 days)
SELECT p.provider, count(*) AS stranded, min(p.created_at), max(p.created_at)
FROM payments p
LEFT JOIN payment_attempts pa ON pa.payment_id = p.id
WHERE p.state = 'created'
  AND pa.id IS NULL
  AND p.created_at > now() - interval '30 days'
GROUP BY p.provider
ORDER BY stranded DESC;

-- 5. MoMo / PSP webhook ingress (30 days)
SELECT provider, validation_status, ingress_status, count(*), max(received_at)
FROM payment_provider_events
WHERE received_at > now() - interval '30 days'
GROUP BY provider, validation_status, ingress_status
ORDER BY count(*) DESC;

-- 6. Stuck idempotency rows
SELECT operation, status, count(*)
FROM machine_idempotency_keys
WHERE status = 'in_progress'
  AND last_seen_at > now() - interval '7 days'
GROUP BY operation, status
ORDER BY count(*) DESC;

-- 7. Payment state breakdown by provider (30 days)
SELECT provider, state, count(*) AS cnt, min(created_at), max(created_at)
FROM payments
WHERE created_at > now() - interval '30 days'
GROUP BY provider, state
ORDER BY provider, cnt DESC;

-- 8. Payments that reached failed within 60s of creation (mapper/query burn suspect)
SELECT p.id, p.provider, p.state, p.order_id, p.created_at,
       EXTRACT(EPOCH FROM (p.updated_at - p.created_at)) AS seconds_to_terminal
FROM payments p
WHERE p.state = 'failed'
  AND p.created_at > now() - interval '30 days'
  AND p.updated_at - p.created_at < interval '60 seconds'
ORDER BY p.created_at DESC
LIMIT 50;

-- 9. Targeted order/payment forensics (replace UUIDs)
-- SELECT p.id, p.provider, p.state, p.amount_minor, p.created_at, p.updated_at, o.status AS order_status
-- FROM payments p JOIN orders o ON o.id = p.order_id
-- WHERE p.order_id = '00000000-0000-0000-0000-000000000000';
--
-- SELECT pa.attempt_seq, pa.provider_reference, pa.created_at,
--        pa.payload->>'provider_reference' AS payload_ref
-- FROM payment_attempts pa
-- WHERE pa.payment_id = '00000000-0000-0000-0000-000000000000'
-- ORDER BY pa.attempt_seq DESC;
--
-- SELECT provider, event_type, validation_status, ingress_status, received_at,
--        payload->>'resultCode' AS result_code,
--        payload->>'orderId' AS momo_order_id
-- FROM payment_provider_events
-- WHERE payment_id = '00000000-0000-0000-0000-000000000000'
-- ORDER BY received_at;
