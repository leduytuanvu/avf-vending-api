# UUID v7 policy

AVF standardizes **internally generated resource identifiers** on UUID version 7 (RFC 9562). This document is the operator and developer contract after the v7 migration (Phases 0–5).

## Requirements

### New internal resource IDs → UUID v7

All **new** primary keys and resource row IDs created by this backend must be UUID v7:

| Layer | Mechanism |
|-------|-----------|
| **Go** | `id.NewUUIDV7()` / `id.NewUUIDV7String()` from [`internal/platform/id`](../../internal/platform/id/uuid_v7.go) |
| **PostgreSQL** | Column default `public.uuid_generate_v7()` (see [`migrations/00005_uuid_v7_defaults.sql`](../../migrations/00005_uuid_v7_defaults.sql)) |

Do **not** call `uuid.New()`, `uuid.NewString()`, `uuid.NewRandom()`, or v4 helpers for resource PKs in production code.

### Existing data is never rewritten

- Production rows keep their current UUID values (v4, v7, or fixed seed literals).
- Migrations must not `UPDATE` existing UUID columns to change version.
- APIs accept any valid RFC 4122 UUID on read paths; OpenAPI `format: uuid` is version-agnostic.

### Out of scope (not UUID v7)

These must **not** be converted to v7:

| Category | Examples |
|----------|----------|
| **Tokens & secrets** | JWT `jti`, refresh token raw bytes, activation codes, API keys |
| **Correlation / tracing** | `X-Request-ID`, gRPC request id, diagnostic `request_id` |
| **Idempotency & dedupe** | `Idempotency-Key`, MQTT dedupe keys, `"order-"+uuid` test prefixes |
| **External / provider IDs** | Webhook event ids, provider references, settlement ids from PSPs |
| **Opaque strings** | SKU suffixes, email local-part randomness in tests |

## Enforcement

CI and local development run:

```bash
bash scripts/checks/check-uuid-v7.sh
```

The check:

1. Scans non-test `.go` files for forbidden `uuid.New*` / v4 helpers.
2. Scans goose **Up** sections in `migrations/*.sql` for `DEFAULT gen_random_uuid()` / `DEFAULT uuid_generate_v4()`.
3. Scans `db/schema/*.sql` for the same forbidden defaults (sqlc mirror must stay aligned).

### Allowlist

Documented exceptions live in [`scripts/checks/uuid-v7-allowlist.txt`](../../scripts/checks/uuid-v7-allowlist.txt).

For one-off lines, add an inline comment on the same line or the line above:

```go
// uuid-v7-allow: JWT jti is an opaque claim, not a resource PK
Jti: uuid.NewString(),
```

```sql
-- uuid-v7-allow: Down migration restores v4 defaults for local rollback
```

New allowlist entries require review — prefer fixing the call site to use v7 or confirming the ID is truly out of scope.

## Database notes

- Baseline migrations `00002` / `00004` historically used `gen_random_uuid()`; `00005` switches forward defaults to v7 without touching existing rows.
- New tables in goose migrations must use `DEFAULT public.uuid_generate_v7()` on internal resource `id` columns.
- Function implementation: PL/pgSQL + `pgcrypto` `gen_random_bytes` (no Supabase-only extensions required).

## Go notes

- Parsing, validation, and `uuid.Nil` sentinels are unchanged.
- Tests may use `uuid.NewString()` for idempotency keys, provider ids, and other excluded categories.
- Tests that assert **generated resource IDs** should use `testfixtures.AssertResourceUUIDV7` or equivalent (v7, non-nil, parseable).

## Related docs

- Audit: [`docs/audits/UUID_V7_STANDARDIZATION_AUDIT.md`](../audits/UUID_V7_STANDARDIZATION_AUDIT.md)
- Migration safety: [`docs/runbooks/migration-safety.md`](../runbooks/migration-safety.md)
