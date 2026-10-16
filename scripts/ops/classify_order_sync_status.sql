-- Classify stuck / ambiguous order rows for sync debugging (AVF000109 Oct 2026 cluster).
-- Usage:
--   psql "$DATABASE_URL" -v machine_code='AVF000109' -v id_prefix='01a13f' \
--     -f scripts/ops/classify_order_sync_status.sql
\set ON_ERROR_STOP on

\if :{?machine_code}
\else
\set machine_code 'AVF000109'
\endif

\if :{?id_prefix}
\else
\set id_prefix '01a13f'
\endif

\echo '=== Classification (target terminal vs unknown vs cancelled QR) ==='
WITH base AS (
    SELECT o.id,
           o.status,
           o.total_minor,
           o.created_at,
           o.updated_at,
           m.code AS machine_code,
           p.provider,
           p.state AS payment_state,
           vs_agg.vend_states,
           vs_agg.open_vend_lines
    FROM orders o
    JOIN machines m ON m.id = o.machine_id
    LEFT JOIN LATERAL (
        SELECT p2.provider, p2.state
        FROM payments p2
        WHERE p2.order_id = o.id
        ORDER BY p2.created_at DESC
        LIMIT 1
    ) p ON TRUE
    LEFT JOIN LATERAL (
        SELECT string_agg(DISTINCT vs.state, ',' ORDER BY vs.state) AS vend_states,
               count(*) FILTER (
                   WHERE vs.state IN ('pending', 'in_progress')
               ) AS open_vend_lines
        FROM vend_sessions vs
        WHERE vs.order_id = o.id
    ) vs_agg ON TRUE
    WHERE m.code = :'machine_code'
      AND o.id::text LIKE (:'id_prefix' || '%')
)
SELECT id,
       status,
       machine_code,
       total_minor,
       created_at,
       provider,
       payment_state,
       vend_states,
       open_vend_lines,
       CASE
           WHEN status IN ('completed', 'partially_completed', 'failed', 'cancelled', 'unknown') THEN 'terminal_ok'
           WHEN status IN ('created', 'quoted')
                AND (payment_state IS NULL OR payment_state NOT IN ('captured', 'partially_refunded'))
               THEN 'likely_qr_abandon_or_orphan_created'
           WHEN status IN ('paid', 'vending') AND open_vend_lines > 0 THEN 'stale_vending_or_finalize_gap'
           WHEN status IN ('paid', 'vending') AND open_vend_lines = 0 THEN 'rollup_stuck_or_missing_finalize'
           WHEN status IN ('created', 'quoted') AND payment_state IN ('captured', 'partially_refunded')
               THEN 'paid_but_order_status_lag'
           ELSE 'review_manual'
       END AS sync_class,
       CASE
           WHEN status IN ('completed', 'partially_completed', 'failed', 'cancelled', 'unknown') THEN NULL
           WHEN status IN ('created', 'quoted')
                AND (payment_state IS NULL OR payment_state NOT IN ('captured', 'partially_refunded'))
               THEN 'cancelled_if_expire_policy'
           WHEN status IN ('paid', 'vending', 'created', 'quoted') THEN 'unknown_if_machine_lost_session'
           ELSE 'review_manual'
       END AS target_status_hint
FROM base
ORDER BY created_at DESC;
