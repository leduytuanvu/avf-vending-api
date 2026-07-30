-- Selective production purge: wipe all business/test data, keep admin@avf.com only.
-- Does NOT drop schema or run goose down.
--
-- Required session setting (abort guard):
--   SET avf.confirm_production_purge='I_UNDERSTAND_THIS_WIPES_PRODUCTION';
--
-- Usage:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
--     -c "SET avf.confirm_production_purge='I_UNDERSTAND_THIS_WIPES_PRODUCTION';" \
--     -f scripts/ops/production-purge-keep-admin.sql

\set ON_ERROR_STOP on

BEGIN;

-- ---------------------------------------------------------------------------
-- Guard rails
-- ---------------------------------------------------------------------------
DO $$
BEGIN
  IF current_setting('avf.confirm_production_purge', true) IS DISTINCT FROM 'I_UNDERSTAND_THIS_WIPES_PRODUCTION' THEN
    RAISE EXCEPTION 'Refusing purge: set avf.confirm_production_purge=I_UNDERSTAND_THIS_WIPES_PRODUCTION first';
  END IF;
END $$;

DO $$
DECLARE
  keep_id uuid;
BEGIN
  SELECT id
  INTO keep_id
  FROM platform_auth_accounts
  WHERE lower(email) = 'admin@avf.com';

  IF keep_id IS NULL THEN
    RAISE EXCEPTION 'admin@avf.com not found — aborting purge';
  END IF;

  PERFORM set_config('avf.keep_admin_id', keep_id::text, true);
  RAISE NOTICE 'Keeping admin account id=%', keep_id;
END $$;

-- ---------------------------------------------------------------------------
-- Auth cleanup (admin account row preserved; sessions/tokens cleared)
-- ---------------------------------------------------------------------------
DELETE FROM admin_sessions;
DELETE FROM auth_refresh_tokens;
DELETE FROM password_reset_tokens;
DELETE FROM admin_mfa_factors;
DELETE FROM admin_login_attempts;
DELETE FROM platform_auth_accounts
WHERE id <> current_setting('avf.keep_admin_id')::uuid;

-- ---------------------------------------------------------------------------
-- Truncate all public tables except schema version + kept admin account
-- ---------------------------------------------------------------------------
DO $$
DECLARE
  preserve constant text[] := ARRAY[
    'goose_db_version',
    'platform_auth_accounts'
  ];
  truncate_targets text;
BEGIN
  SELECT string_agg(format('%I.%I', schemaname, tablename), ', ' ORDER BY tablename)
  INTO truncate_targets
  FROM pg_tables
  WHERE schemaname = 'public'
    AND NOT (tablename = ANY (preserve));

  IF truncate_targets IS NULL OR btrim(truncate_targets) = '' THEN
    RAISE EXCEPTION 'No tables selected for truncate — aborting';
  END IF;

  RAISE NOTICE 'Truncating tables: %', truncate_targets;
  EXECUTE format('TRUNCATE TABLE %s RESTART IDENTITY CASCADE', truncate_targets);
END $$;

-- ---------------------------------------------------------------------------
-- 00003_seed_dev.sql Down rows (belt-and-suspenders after truncate)
-- ---------------------------------------------------------------------------
DELETE FROM ota_campaign_targets WHERE campaign_id = 'eeeeeeee-eeee-eeee-eeee-000000000001';
DELETE FROM ota_campaigns WHERE id = 'eeeeeeee-eeee-eeee-eeee-000000000001';
DELETE FROM ota_artifacts WHERE id = 'dddddddd-dddd-dddd-dddd-000000000001';
DELETE FROM machine_shadow WHERE machine_id = '55555555-5555-5555-5555-555555555555';
DELETE FROM machine_slot_state WHERE machine_id = '55555555-5555-5555-5555-555555555555';
DELETE FROM slots WHERE planogram_id = 'cccccccc-cccc-cccc-cccc-000000000001';
DELETE FROM planograms WHERE id = 'cccccccc-cccc-cccc-cccc-000000000001';
DELETE FROM product_media WHERE id IN (
  'dddddddd-dddd-dddd-dddd-000000000201',
  'dddddddd-dddd-dddd-dddd-000000000202'
);
DELETE FROM product_images WHERE id IN (
  'dddddddd-dddd-dddd-dddd-000000000201',
  'dddddddd-dddd-dddd-dddd-000000000202'
);
DELETE FROM media_assets WHERE id IN (
  'dddddddd-dddd-dddd-dddd-000000000101',
  'dddddddd-dddd-dddd-dddd-000000000102'
);
DELETE FROM price_book_items WHERE price_book_id = 'bbbbbbbb-bbbb-bbbb-bbbb-000000000001';
DELETE FROM price_books WHERE id = 'bbbbbbbb-bbbb-bbbb-bbbb-000000000001';
DELETE FROM products WHERE sku IN ('SKU-COLA', 'SKU-WATER');
DELETE FROM technician_machine_assignments WHERE id = '77777777-7777-7777-7777-777777777777';
DELETE FROM technicians WHERE id = '66666666-6666-6666-6666-666666666666';
DELETE FROM machines WHERE id = '55555555-5555-5555-5555-555555555555';
DELETE FROM machine_hardware_profiles WHERE id = '44444444-4444-4444-4444-444444444444';
DELETE FROM sites WHERE id = '33333333-3333-3333-3333-333333333333';
DELETE FROM regions WHERE id = '22222222-2222-2222-2222-222222222222';

COMMIT;

-- Post-purge verification (outside transaction for visibility)
\echo ''
\echo '=== Post-purge verification ==='
SELECT count(*) AS machines FROM machines;
SELECT count(*) AS products FROM products;
SELECT count(*) AS platform_auth_accounts FROM platform_auth_accounts;
SELECT email, status, roles FROM platform_auth_accounts;
SELECT version FROM goose_db_version ORDER BY version DESC LIMIT 1;
