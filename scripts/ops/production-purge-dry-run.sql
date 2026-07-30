-- Production purge dry-run: row counts only (safe to run anytime).
-- Usage:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/ops/production-purge-dry-run.sql

\set ON_ERROR_STOP on

\echo '=== Admin account to keep ==='
SELECT id, email, status, locked_until, roles
FROM platform_auth_accounts
WHERE lower(email) = 'admin@avf.com';

\echo ''
\echo '=== Auth (non-admin accounts) ==='
SELECT count(*) AS other_auth_accounts
FROM platform_auth_accounts
WHERE lower(email) <> 'admin@avf.com';

SELECT count(*) AS admin_sessions FROM admin_sessions;
SELECT count(*) AS auth_refresh_tokens FROM auth_refresh_tokens;
SELECT count(*) AS admin_login_attempts FROM admin_login_attempts;

\echo ''
\echo '=== Fleet / sites ==='
SELECT count(*) AS regions FROM regions;
SELECT count(*) AS sites FROM sites;
SELECT count(*) AS machines FROM machines;
SELECT count(*) AS machine_hardware_profiles FROM machine_hardware_profiles;
SELECT count(*) AS machine_credentials FROM machine_credentials;
SELECT count(*) AS machine_sessions FROM machine_sessions;
SELECT count(*) AS machine_mqtt_credentials FROM machine_mqtt_credentials;
SELECT count(*) AS machine_activation_codes FROM machine_activation_codes;
SELECT count(*) AS machine_activation_claims FROM machine_activation_claims;
SELECT count(*) AS machine_device_certificates FROM machine_device_certificates;
SELECT count(*) AS machine_device_attachments FROM machine_device_attachments;
SELECT count(*) AS machine_runtime_app_sessions FROM machine_runtime_app_sessions;
SELECT count(*) AS technicians FROM technicians;
SELECT count(*) AS technician_machine_assignments FROM technician_machine_assignments;

\echo ''
\echo '=== Catalog / planograms ==='
SELECT count(*) AS categories FROM categories;
SELECT count(*) AS brands FROM brands;
SELECT count(*) AS products FROM products;
SELECT count(*) AS tags FROM tags;
SELECT count(*) AS media_assets FROM media_assets;
SELECT count(*) AS product_images FROM product_images;
SELECT count(*) AS product_media FROM product_media;
SELECT count(*) AS price_books FROM price_books;
SELECT count(*) AS price_book_items FROM price_book_items;
SELECT count(*) AS promotions FROM promotions;
SELECT count(*) AS planograms FROM planograms;
SELECT count(*) AS slots FROM slots;
SELECT count(*) AS machine_slot_state FROM machine_slot_state;
SELECT count(*) AS machine_planogram_drafts FROM machine_planogram_drafts;
SELECT count(*) AS machine_planogram_versions FROM machine_planogram_versions;
SELECT count(*) AS machine_planogram_slots FROM machine_planogram_slots;
SELECT count(*) AS machine_lane_merge_pairs FROM machine_lane_merge_pairs;

\echo ''
\echo '=== Commerce / ops ==='
SELECT count(*) AS orders FROM orders;
SELECT count(*) AS vend_sessions FROM vend_sessions;
SELECT count(*) AS checkout_quotes FROM checkout_quotes;
SELECT count(*) AS checkout_quote_lines FROM checkout_quote_lines;
SELECT count(*) AS payments FROM payments;
SELECT count(*) AS payment_attempts FROM payment_attempts;
SELECT count(*) AS refunds FROM refunds;
SELECT count(*) AS machine_offline_events FROM machine_offline_events;
SELECT count(*) AS inventory_events FROM inventory_events;
SELECT count(*) AS machine_operator_sessions FROM machine_operator_sessions;
SELECT count(*) AS refill_sessions FROM refill_sessions;

\echo ''
\echo '=== Platform noise ==='
SELECT count(*) AS outbox_events FROM outbox_events;
SELECT count(*) AS audit_events FROM audit_events;
SELECT count(*) AS audit_logs FROM audit_logs;
SELECT count(*) AS command_ledger FROM command_ledger;
SELECT count(*) AS device_telemetry_events FROM device_telemetry_events;
SELECT count(*) AS ota_campaigns FROM ota_campaigns;
SELECT count(*) AS ota_artifacts FROM ota_artifacts;

\echo ''
\echo '=== Migration seed dev (00003) — must be 0 after purge ==='
SELECT count(*) AS seed_regions
FROM regions
WHERE id = '22222222-2222-2222-2222-222222222222';
SELECT count(*) AS seed_machines
FROM machines
WHERE id = '55555555-5555-5555-5555-555555555555';
SELECT count(*) AS seed_products
FROM products
WHERE sku IN ('SKU-COLA', 'SKU-WATER');
SELECT count(*) AS seed_planograms
FROM planograms
WHERE id = 'cccccccc-cccc-cccc-cccc-000000000001';

\echo ''
\echo '=== Schema version (must stay unchanged) ==='
SELECT version FROM goose_db_version ORDER BY version DESC LIMIT 1;
