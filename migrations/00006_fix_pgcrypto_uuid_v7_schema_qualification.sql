-- +goose Up
-- Forward fix: qualify pgcrypto calls for Supabase (pgcrypto in extensions schema).
-- Production login failed during audit_events INSERT: function gen_random_bytes(integer) does not exist (42883).
-- See docs/audits/PGCRYPTO_UUID_V7_AUDIT.md

CREATE SCHEMA IF NOT EXISTS extensions;

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.uuid_generate_v7()
RETURNS uuid
LANGUAGE plpgsql
VOLATILE
PARALLEL SAFE
SET search_path = public, extensions, pg_temp
AS $$
DECLARE
    unix_ts_ms bytea;
    uuid_bytes bytea;
BEGIN
    unix_ts_ms := substring(
        int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint)
        FROM 3 FOR 6
    );
    uuid_bytes := unix_ts_ms || extensions.gen_random_bytes(10);
    uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 15) | 112);
    uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 63) | 128);
    RETURN encode(uuid_bytes, 'hex')::uuid;
END;
$$;

COMMENT ON FUNCTION public.uuid_generate_v7() IS
    'RFC 9562 UUID v7 for internal resource PK defaults. Uses extensions.gen_random_bytes (pgcrypto).';
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
DECLARE
    v uuid;
    version_nibble text;
BEGIN
    PERFORM extensions.gen_random_bytes(16);
    v := public.uuid_generate_v7();
    IF v IS NULL THEN
        RAISE EXCEPTION 'uuid_generate_v7 returned null';
    END IF;
    version_nibble := substring(v::text FROM 15 FOR 1);
    IF version_nibble <> '7' THEN
        RAISE EXCEPTION 'uuid_generate_v7 version nibble = %, want 7', version_nibble;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Irreversible: reverting would restore broken unqualified gen_random_bytes on extensions-only pgcrypto.
SELECT 1;
