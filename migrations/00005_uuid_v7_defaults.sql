-- +goose Up
-- Phase 3: forward-only UUID v7 defaults for internal resource primary keys.
-- Existing row UUID values are unchanged; only INSERT ... DEFAULT (omit id) uses v7.
-- See docs/audits/UUID_V7_STANDARDIZATION_AUDIT.md (Phase 3).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.uuid_generate_v7()
RETURNS uuid
LANGUAGE plpgsql
VOLATILE
PARALLEL SAFE
SET search_path = public, pg_temp
AS $$
DECLARE
    unix_ts_ms bytea;
    uuid_bytes bytea;
BEGIN
    -- 48-bit big-endian Unix timestamp in milliseconds (RFC 9562 UUID v7).
    unix_ts_ms := substring(
        int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint)
        FROM 3 FOR 6
    );
    uuid_bytes := unix_ts_ms || gen_random_bytes(10);
    -- Version 7 (bits 48-51 = 0111).
    uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 15) | 112);
    -- RFC 4122 variant (bits 64-65 = 10).
    uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 63) | 128);
    RETURN encode(uuid_bytes, 'hex')::uuid;
END;
$$;

COMMENT ON FUNCTION public.uuid_generate_v7() IS
    'RFC 9562 UUID v7 for internal resource PK defaults. Uses pgcrypto gen_random_bytes; no extension beyond pgcrypto required.';
-- +goose StatementEnd

-- Convert every public.id uuid column whose default is gen_random_uuid() or uuid_generate_v4().
-- +goose StatementBegin
DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT
            n.nspname AS schema_name,
            c.relname AS table_name
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_attribute a ON a.attrelid = c.oid AND NOT a.attisdropped
        JOIN pg_attrdef d ON d.adrelid = c.oid AND d.adnum = a.attnum
        WHERE n.nspname = 'public'
          AND c.relkind = 'r'
          AND a.attname = 'id'
          AND pg_get_expr(d.adbin, d.adrelid) ~ '(gen_random_uuid|uuid_generate_v4)\s*\(\s*\)'
    LOOP
        EXECUTE format(
            'ALTER TABLE %I.%I ALTER COLUMN id SET DEFAULT public.uuid_generate_v7()',
            r.schema_name,
            r.table_name
        );
    END LOOP;
END;
$$;
-- +goose StatementEnd

-- Verification (manual): after migration, new rows omitting id should be version 7.
--   INSERT INTO regions (name, code) VALUES ('v7-check', 'v7-check') RETURNING id;
--   SELECT substring(id::text FROM 15 FOR 1) AS version_nibble;  -- expect '7'
--
-- Converted tables (91 id columns, migrations 00002 + 00004):
--   admin_login_attempts, admin_mfa_factors, admin_sessions, assortment_items, assortments,
--   audit_events, auth_refresh_tokens, brands, cash_collections, cash_reconciliations,
--   categories, command_ledger, commerce_reconciliation_cases, diagnostic_bundle_manifests,
--   feature_flag_targets, feature_flags, finance_daily_close_adjustments, finance_daily_closes,
--   incidents, inventory_anomalies, inventory_count_sessions, machine_activation_claims,
--   machine_activation_codes, machine_assortment_bindings, machine_cabinets,
--   machine_command_attempts, machine_config_rollouts, machine_config_versions, machine_configs,
--   machine_credentials, machine_device_certificates, machine_hardware_profiles,
--   machine_idempotency_keys, machine_incidents, machine_lineage, machine_modules,
--   machine_offline_events, machine_operator_sessions, machine_planogram_drafts,
--   machine_planogram_slots, machine_planogram_versions, machine_price_overrides,
--   machine_provisioning_batch_machines, machine_provisioning_batches,
--   machine_reconciliation_sessions, machine_runtime_refresh_tokens, machine_sessions,
--   machine_slot_configs, machine_slot_layouts, machine_slot_state, machine_transport_sessions,
--   machines, media_assets, media_variants, order_timelines, orders, ota_artifacts,
--   ota_campaign_events, ota_campaign_targets, ota_campaigns, ota_machine_results,
--   password_reset_tokens, payment_attempts, payment_disputes, payment_provider_settlements,
--   payment_reconciliations, payments, planogram_templates, planograms, platform_auth_accounts,
--   price_book_items, price_book_targets, price_books, product_images, products,
--   promotion_rules, promotion_targets, promotions, refill_sessions, refund_requests, refunds,
--   regions, rollout_campaigns, rollout_targets, settlement_batches, sites, slots, tags,
--   technician_machine_assignments, technicians, vend_sessions
--
-- Excluded (no gen_random_uuid id default): composite/natural PKs, correlation columns,
-- provider references, idempotency key strings, opaque token columns, seed literals in 00003.

-- +goose Down
-- Restore v4-style defaults for id columns that were switched to uuid_generate_v7().
-- Safe for roll-forward-only production policy: Down is for local/dev reversal only.

-- +goose StatementBegin
DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT
            n.nspname AS schema_name,
            c.relname AS table_name
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN pg_attribute a ON a.attrelid = c.oid AND NOT a.attisdropped
        JOIN pg_attrdef d ON d.adrelid = c.oid AND d.adnum = a.attnum
        WHERE n.nspname = 'public'
          AND c.relkind = 'r'
          AND a.attname = 'id'
          AND pg_get_expr(d.adbin, d.adrelid) ~ 'uuid_generate_v7\s*\(\s*\)'
    LOOP
        EXECUTE format(
            'ALTER TABLE %I.%I ALTER COLUMN id SET DEFAULT gen_random_uuid()',
            r.schema_name,
            r.table_name
        );
    END LOOP;
END;
$$;
-- +goose StatementEnd

DROP FUNCTION IF EXISTS public.uuid_generate_v7();
