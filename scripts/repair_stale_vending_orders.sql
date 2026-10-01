-- Repair orders stuck in `vending` when all vend_sessions are already terminal.
-- Run manually after deploying partially_completed status support (migration 00036).
--
-- Preview stale orders (vending > 30 minutes, all lines terminal):
-- SELECT o.id, o.status, o.updated_at,
--        array_agg(vs.state ORDER BY vs.slot_index) AS line_states
-- FROM orders o
-- JOIN vend_sessions vs ON vs.order_id = o.id
-- WHERE o.status = 'vending'
--   AND o.updated_at < now() - interval '30 minutes'
-- GROUP BY o.id
-- HAVING bool_and(vs.state IN ('success', 'failed'));

WITH terminal_sessions AS (
    SELECT
        o.id AS order_id,
        bool_or(vs.state = 'success') AS any_success,
        bool_or(vs.state = 'failed') AS any_failed,
        bool_and(vs.state IN ('success', 'failed')) AS all_terminal
    FROM orders o
    JOIN vend_sessions vs ON vs.order_id = o.id
    WHERE o.status = 'vending'
    GROUP BY o.id
),
resolved AS (
    SELECT
        order_id,
        CASE
            WHEN any_success AND any_failed THEN 'partially_completed'
            WHEN any_success THEN 'completed'
            WHEN any_failed THEN 'failed'
            ELSE 'vending'
        END AS next_status
    FROM terminal_sessions
    WHERE all_terminal
)
UPDATE orders o
SET
    status = r.next_status,
    updated_at = now()
FROM resolved r
WHERE o.id = r.order_id
  AND r.next_status <> 'vending';

-- Alert query for ops dashboards:
-- SELECT count(*) AS stale_vending_count
-- FROM orders
-- WHERE status = 'vending'
--   AND updated_at < now() - interval '30 minutes';
