-- Read-only: locate order/payment by MoMo orderId and/or transId.
\set ON_ERROR_STOP on

\echo '=== Parameters ==='
\echo 'momo_order_id=' :momo_order_id
\echo 'momo_trans_id=' :momo_trans_id
\echo 'machine_code=' :machine_code

\echo ''
\echo '=== payment_attempts by provider_reference (MoMo orderId) ==='
SELECT pa.id AS attempt_id,
       pa.payment_id,
       pa.provider_reference,
       pa.state AS attempt_state,
       pa.created_at AS attempt_at_vn,
       p.order_id,
       p.provider,
       p.state AS payment_state,
       p.amount_minor,
       p.currency,
       o.status AS order_status,
       o.created_at AS order_created_vn,
       m.code AS machine_code
FROM payment_attempts pa
INNER JOIN payments p ON p.id = pa.payment_id
INNER JOIN orders o ON o.id = p.order_id
INNER JOIN machines m ON m.id = o.machine_id
WHERE btrim(pa.provider_reference) = btrim(:'momo_order_id')
ORDER BY pa.created_at DESC;

\echo ''
\echo '=== payment_provider_events (orderId / transId in payload) ==='
SELECT ppe.id AS event_id,
       ppe.payment_id,
       ppe.provider,
       ppe.provider_ref,
       ppe.event_type,
       ppe.validation_status,
       ppe.ingress_status,
       ppe.received_at AS received_at_vn,
       ppe.payload->>'orderId' AS payload_order_id,
       ppe.payload->>'transId' AS payload_trans_id,
       ppe.payload->>'resultCode' AS result_code
FROM payment_provider_events ppe
WHERE ppe.provider ILIKE '%momo%'
  AND (
    btrim(COALESCE(ppe.provider_ref, '')) = btrim(:'momo_order_id')
    OR ppe.payload->>'orderId' = btrim(:'momo_order_id')
    OR ppe.payload->>'transId' = btrim(:'momo_trans_id')
    OR ppe.payload::text ILIKE ('%' || btrim(:'momo_trans_id') || '%')
  )
ORDER BY ppe.received_at DESC
LIMIT 20;

\echo ''
\echo '=== orders near expected time on machine (if machine_code set) ==='
SELECT o.id AS order_id,
       o.status,
       o.total_minor,
       o.currency,
       o.created_at AS created_vn,
       o.updated_at AS updated_vn,
       m.code AS machine_code
FROM orders o
INNER JOIN machines m ON m.id = o.machine_id
WHERE (:'machine_code' = '' OR lower(btrim(m.code)) = lower(btrim(:'machine_code')))
  AND o.created_at >= timestamptz '2026-10-12 15:30:00+07'
  AND o.created_at <= timestamptz '2026-10-12 16:00:00+07'
ORDER BY o.created_at DESC
LIMIT 30;

\echo ''
\echo '=== vend_sessions for matched orders ==='
WITH matched_orders AS (
    SELECT DISTINCT p.order_id
    FROM payment_attempts pa
    INNER JOIN payments p ON p.id = pa.payment_id
    WHERE btrim(pa.provider_reference) = btrim(:'momo_order_id')
)
SELECT vs.id AS vend_session_id,
       vs.order_id,
       vs.state AS vend_state,
       vs.slot_index,
       p.sku,
       p.name AS product_name,
       vs.started_at,
       vs.completed_at,
       vs.failure_reason
FROM vend_sessions vs
INNER JOIN matched_orders mo ON mo.order_id = vs.order_id
LEFT JOIN products p ON p.id = vs.product_id
ORDER BY vs.created_at;

\echo ''
\echo '=== payments summary for matched orders ==='
WITH matched_orders AS (
    SELECT DISTINCT p.order_id
    FROM payment_attempts pa
    INNER JOIN payments p ON p.id = pa.payment_id
    WHERE btrim(pa.provider_reference) = btrim(:'momo_order_id')
)
SELECT p.id AS payment_id,
       p.order_id,
       p.provider,
       p.state,
       p.amount_minor,
       p.currency,
       p.created_at,
       p.updated_at
FROM payments p
INNER JOIN matched_orders mo ON mo.order_id = p.order_id
ORDER BY p.created_at;
