# Machine activation / provisioning — implementation handoff

**Implementation status:** **Shipped** — `internal/httpserver/activation_http.go`, `internal/app/activation`, `mountPublicActivationClaim` / `mountAdminActivationRoutes` in `internal/httpserver/server.go`.

**P0.4 activation claims:** **`migrations/00052_machine_activation_claims.sql`** adds **`machine_activation_claims`** (per-attempt fingerprint hash, optional IP / user-agent, **`result`** ∈ succeeded | failed | rejected). **`max_uses`** is enforced against **COUNT(succeeded)** rows inside the same transaction as **`FOR UPDATE`** on **`machine_activation_codes`** (serialized concurrent claims). Partial unique index **`(activation_code_id, fingerprint_hash) WHERE result = 'succeeded'`** gives safe idempotent replay for the same device fingerprint. **`machine_activation_codes.uses`** / **`claimed_fingerprint_hash`** remain for backward compatibility and are refreshed via **`RefreshMachineActivationCodeAggregate`**. Rejected / failed attempts write **`machine_activation_claims`** rows and **`machine.activation.rejected`** audit when **`EnterpriseRecorder`** is wired.

This document retains the **design checklist**, migration reference, and acceptance commands for reviewers.

## Routes (Chi)

| Method | Path | Auth |
|--------|------|------|
| POST | `/v1/admin/machines/{machineId}/activation-codes` | Bearer, `org_admin` or `platform_admin` |
| GET | `/v1/admin/machines/{machineId}/activation-codes` | same |
| DELETE | `/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}` | same |
| POST | `/v1/setup/activation-codes/claim` | **Public** (no Bearer); rate-limit POST |

Mount **claim** on `/v1` **outside** the `v1Auth` group (same pattern as commerce webhook). Wrap with `SensitiveWriteRateLimitIfEnabled` (existing `writeRL`).

Keep `GET /v1/setup/machines/{machineId}/bootstrap` inside the authenticated group.

**Tenant guard (admin):** reuse [`parseAdminFleetOrganizationScope`](../../internal/httpserver/admin_fleet_http.go) + [`AdminMachines.GetMachine(ctx, orgID, machineID)`](../../internal/app/fleetadmin/service.go). On `pgx.ErrNoRows` → **404** `machine_not_found` (no cross-tenant leak).

**Machine JWT vs admin:** add [`IsMachineRuntimePrincipal`](../../internal/platform/auth/principal.go) (`len(machine_ids) > 0` and not `platform_admin`/`org_admin`) and [`RequireDenyMachineRuntimePrincipal`](../../internal/platform/auth/middleware.go) on `/v1/admin` (after `RequireAnyRole`). Ensures claimed machine tokens cannot hit admin APIs.

## Migration `migrations/00021_machine_activation_codes.sql`

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE activation_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    code_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    max_uses int NOT NULL DEFAULT 1 CHECK (max_uses >= 1),
    used_count int NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'exhausted')),
    notes text NOT NULL DEFAULT '',
    created_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    last_claimed_at timestamptz,
    CONSTRAINT fk_activation_codes_org_machine FOREIGN KEY (organization_id, machine_id) REFERENCES machines (organization_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX ux_activation_codes_code_hash ON activation_codes (code_hash);
CREATE INDEX ix_activation_codes_machine_created ON activation_codes (machine_id, created_at DESC);
CREATE INDEX ix_activation_codes_org ON activation_codes (organization_id);

CREATE TABLE activation_claims (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    activation_code_id uuid NOT NULL REFERENCES activation_codes (id) ON DELETE CASCADE,
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    fingerprint_hash bytea NOT NULL,
    fingerprint_json jsonb NOT NULL,
    device_binding_id uuid NOT NULL,
    claimed_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ux_activation_claims_code_fingerprint UNIQUE (activation_code_id, fingerprint_hash)
);

CREATE INDEX ix_activation_claims_machine ON activation_claims (machine_id);
CREATE INDEX ix_activation_claims_code ON activation_claims (activation_code_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS activation_claims;
DROP TABLE IF EXISTS activation_codes;
-- +goose StatementEnd
```

Mirror the same DDL at the end of [`db/schema/01_platform.sql`](../../db/schema/01_platform.sql) (sqlc source of truth comment).

## `db/queries/activation.sql` (sqlc)

Implement at minimum:

- `InsertActivationCode` — insert row; return full row.
- `GetActivationCodeByHashForUpdate` — `SELECT * FROM activation_codes WHERE code_hash = $1 FOR UPDATE`.
- `ListActivationCodesByMachine` — filter `machine_id = $1` order by `created_at DESC` limit/offset.
- `CountActivationCodesByMachine` — for `meta.total`.
- `GetActivationCodeByIDAndMachine` — `id` + `machine_id` (for revoke).
- `RevokeActivationCode` — set `status = 'revoked'`, `revoked_at = now()` where id + machine_id and `status = 'active'`.
- `InsertActivationClaim` — insert claim row.
- `GetActivationClaimByCodeAndFingerprint` — idempotent replay lookup.
- `IncrementActivationCodeUsage` — `used_count = used_count + 1`, `last_claimed_at = now()`, set `status = 'exhausted'` when `used_count + 1 >= max_uses`.

Run `make sqlc` and commit `internal/gen/db/*`.

## Config ([`internal/config/config.go`](../../internal/config/config.go))

Add `Activation ActivationConfig`:

- `CodePepper []byte` — env `ACTIVATION_CODE_PEPPER`, else fallback to `HTTP_AUTH_JWT_SECRET` (trimmed).
- `MachineAccessTokenTTL time.Duration` — env `MACHINE_ACCESS_TOKEN_TTL` default e.g. `720h`.
- `MQTTPublicBrokerURL string` — env `MQTT_PUBLIC_BROKER_URL`, fallback `MQTT_BROKER_URL` (kiosk-facing broker string).

Pass `cfg.Activation`, `cfg.MQTT`, and bootstrap path hints into [`HTTPApplicationDeps`](../../internal/app/api/application.go).

## Code generation and hashing

- **Format:** `AVF-` + 6 + `-` + 6 from alphabet `23456789ABCDEFGHJKLMNPQRSTUVWXYZ` (crypto/rand).
- **Normalize:** trim, uppercase, remove spaces.
- **Hash:** `SHA256(pepper || "\x00" || normalized)` stored as `bytea`; compare with `subtle.ConstantTimeCompare`.
- **Never log** plaintext after create; log only `activation_code_id`, `machine_id`, `request_id`.

## Claim transaction (pseudocode)

1. Begin tx.
2. `GetActivationCodeByHashForUpdate`.
3. If missing → rollback, return generic **400** `invalid_activation_code` (same message always).
4. If `status != 'active'` or `revoked_at != null` or `now() > expires_at` → same error.
5. Fingerprint canonical JSON (fixed field order struct) → `fingerprint_json`; `fingerprint_hash = SHA256(json)`.
6. `GetActivationClaimByCodeAndFingerprint` — if found → commit, **re-mint** JWT with same `device_binding_id`, return 200 (idempotent).
7. If `used_count >= max_uses` → generic error (exhausted).
8. `InsertActivationClaim` (new `device_binding_id = uuid.New()`), `IncrementActivationCodeUsage`.
9. Commit; mint JWT.

**Wrong fingerprint after single-use exhausted:** step 6 misses; step 7 fires → generic error (do not distinguish).

## Machine JWT ([`internal/platform/auth`](../../internal/platform/auth))

Machine access JWTs are issued separately from admin session semantics. Required claims are `iss`, `aud`, `sub=machine:{machine_id}`, `organization_id`, `machine_id`, `roles=["machine"]`, `typ=machine`, `token_use=machine_access`, `token_version`, `exp`, `iat`, and `nbf`.

Runtime gRPC auth validates signature, issuer, audience, `token_use`, role, token version, revocation, and machine lifecycle state. Machine refresh tokens remain opaque random values stored only as SHA-256 hashes in `machine_runtime_refresh_tokens`; refresh rotates the opaque refresh token and bumps `machines.credential_version`, invalidating old machine access JWTs.

Config aliases:

- `AUTH_ISSUER`
- `AUTH_ADMIN_AUDIENCE`
- `AUTH_MACHINE_AUDIENCE`
- `MACHINE_ACCESS_TTL`
- `MACHINE_REFRESH_TTL`
- `MACHINE_TOKEN_CLOCK_SKEW`
- `MACHINE_AUTH_REQUIRE_AUDIENCE`

## HTTP DTOs

- Create request: `expiresInMinutes` (1–43200), `maxUses` (≥1, cap e.g. 100), `notes` optional string.
- Create response **201:** include plaintext `activationCode` **once**; `remainingUses` = `maxUses`.
- List: items **without** plaintext code; `meta.limit/offset/returned/total`.
- Claim request/response: match user spec (`deviceFingerprint` object, `mqtt`, `bootstrapUrl` relative path).

## OpenAPI

1. Add DocOp\* stubs in [`internal/httpserver/swagger_operations.go`](../../internal/httpserver/swagger_operations.go).
2. Add paths to `REQUIRED_OPERATIONS` in [`tools/build_openapi.py`](../../tools/build_openapi.py).
3. Claim route: **omit** `@Security` so generator does not attach `bearerAuth`.
4. Run `make swagger` && `make swagger-check`.

## Tests

- **Unit:** hash/normalize, code charset length, JWT round-trip.
- **Integration** (`TEST_DATABASE_URL`): create org/site/machine, create code, assert hash in DB ≠ plaintext, claim success, list without plaintext, revoke, claim fails, expired row, wrong fingerprint after use, org B cannot create for org A machine (404), machine JWT + `RequireMachineURLAccess` can GET bootstrap, machine JWT gets **403** on `/v1/admin/machines`.

## Docs

Update [`docs/api/kiosk-app-flow.md`](kiosk-app-flow.md) §1 to describe claim + bootstrap + machine token (replace “until this ships”).

Update [`docs/api/api-surface-audit.md`](api-surface-audit.md) activation row to **implemented** with paths above.

## Acceptance commands

```bash
make sqlc && make sqlc-check
make swagger && make swagger-check
go test ./...
```

