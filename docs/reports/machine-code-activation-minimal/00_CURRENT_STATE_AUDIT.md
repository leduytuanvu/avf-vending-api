# Machine-code activation — current state audit

Date: 2026-07-06

## Goal (minimal scope)

Allow admin/operator to create, list, and revoke activation codes using human-facing `machineCode` (e.g. `AVF000001`) while runtime continues to use `machineId` UUID for REST, gRPC, MQTT, JWT, DB, and ACL.

## Identity model (unchanged)

| Identifier | Role |
|------------|------|
| `machineCode` | Human/admin field on `machines.code` (e.g. `AVF000001`) |
| `machineId` | Internal UUID (`machines.id`) for DB/JWT/gRPC/MQTT/runtime |
| `androidId` | Android board fingerprint only (claim flow) |

## Admin activation routes (before change)

| Method | Route | Machine identifier |
|--------|-------|-------------------|
| POST | `/v1/admin/machines/{machineId}/activation-codes` | UUID only (`uuid.Parse`) |
| GET | `/v1/admin/machines/{machineId}/activation-codes` | UUID only |
| DELETE | `/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}` | UUID only |
| GET | `/v1/admin/activation-codes` | Catalog (no path ref) |
| POST | `/v1/admin/activation-codes` | Body: `machineId` / `machine_id` only |
| POST | `/v1/admin/activation-codes/{codeId}/revoke` | Catalog revoke |

No `/v1/admin/machine-codes/{machineCode}/activation-codes` routes existed.

## Create/list response (before change)

Create (201) returned: `activationCode`, `activationCodeId`, `machineId`, `expiresAt`, `maxUses`, `remainingUses`, `status`.

**Missing:** `machineCode`.

List items returned metadata only (no plaintext activation code, no hash) — correct for security.

## SQL / persistence

- `machine_activation_codes.machine_id` stores UUID FK to `machines.id` — correct.
- Plaintext activation codes never stored; only `code_hash`.
- `GetMachineByID` exists in `db/queries/fleet.sql`.
- **`GetMachineByCode` did not exist.**
- `ListMachineActivationCodesForMachine` selected `mac.*` without join to `machines.code`.

## Machine code helpers

- `machineruntime.NormalizeMachineCode` / `ValidMachineCode` use `^AVF[0-9]{6,}$` (fleet-wide).
- **Plan decision:** activation admin resolver uses stricter `^AVF[0-9]{6}$` (exactly 6 digits) without changing fleet CRUD validation.

## Public claim

- `POST /v1/setup/activation-codes/claim` uses activation code + device fingerprint.
- Resolves machine by UUID from activation code row; no machineCode in request.
- Board replacement via `machineruntime.attachOrReplaceDeviceTx`; session closed with `EndReason=BOARD_REPLACED`.

## Test coverage (before change)

- DB integration: claim + attachment in `internal/app/activation/service_attachment_integration_test.go`.
- No httpserver DB tests for activation admin routes.
- No machineCode path/body tests.

## Gaps to close

1. Machine ref resolver (UUID or AVF code) for activation admin only.
2. New `/v1/admin/machine-codes/{machineCode}/activation-codes` routes.
3. Extend `{machineId}` path param to accept machineCode.
4. Catalog create body: `machineCode` / `machine_code` + conflict handling.
5. SQL join for list + `GetMachineByCode`.
6. Response DTOs include `machineCode`.
7. OpenAPI, Postman (activation only), runbook, tests, reports.

## Out of scope (confirmed)

MQTT topics, EMQX ACL, JWT claims, DB FKs, proto renames, fleet-wide route refactor, Android ID as machine identity.
