# Machine-Code Activation — Current State Audit

Date: 2026-07-06  
Repository: `avf-vending-api`  
Production baseline SHA: `22e56f0f` (merge PR #423, deploy tag `v20260705-22e56f0`)

## Executive summary

Machine-code activation admin routes are **implemented and deployed**. Admin can create/list/revoke activation codes using `machineCode` (e.g. `AVF000001`) or UUID. Runtime REST/gRPC/MQTT remains UUID-based. Remaining gaps for this verification effort: **`MachineIdentityRef` struct** (tuple return today), **incomplete HTTP test matrix**, and **production full-surface verification not yet executed** for this release.

---

## Activation-code routes (current)

| Method | Path | Handler | Auth |
|--------|------|---------|------|
| POST | `/v1/setup/activation-codes/claim` | `postActivationClaim` | Public |
| GET | `/v1/admin/machines/{machineId}/activation-codes` | `getAdminListActivationCodes` | `PermFleetRead` |
| POST | `/v1/admin/machines/{machineId}/activation-codes` | `postAdminCreateActivationCode` | `PermSetupWrite` |
| DELETE | `/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}` | `deleteAdminActivationCode` | `PermSetupWrite` |
| GET | `/v1/admin/machine-codes/{machineCode}/activation-codes` | `getAdminListActivationCodes` | `PermFleetRead` |
| POST | `/v1/admin/machine-codes/{machineCode}/activation-codes` | `postAdminCreateActivationCode` | `PermSetupWrite` |
| DELETE | `/v1/admin/machine-codes/{machineCode}/activation-codes/{activationCodeId}` | `deleteAdminActivationCode` | `PermSetupWrite` |
| GET | `/v1/admin/activation-codes` | `getAdminOrgListActivationCodes` | `PermFleetRead` |
| POST | `/v1/admin/activation-codes` | `postAdminOrgCreateActivationCode` | `PermSetupWrite` |
| POST | `/v1/admin/activation-codes/{codeId}/revoke` | `postAdminOrgRevokeActivationCode` | `PermSetupWrite` |

Mounted in `internal/httpserver/server.go` under `/v1/admin`.

---

## UUID-only parsing locations

| Location | Behavior |
|----------|----------|
| `{machineId}` path param | **Not UUID-only** — resolved via `ResolveMachineRef` (UUID or `AVF000001`) |
| Catalog body | Accepts `machineId`, `machine_id`, `machineCode`, `machine_code` |
| JWT `machine_id` claim | **UUID only** (unchanged) |
| MQTT topics | **UUID only** (unchanged) |
| gRPC machine-scoped calls | **UUID only** (unchanged) |
| `machine_activation_codes.machine_id` | **UUID FK** (unchanged) |

Resolver: `internal/app/activation/machineref.go` — strict regex `^AVF[0-9]{6}$` for activation admin (fleet CRUD uses `^AVF[0-9]{6,}$` via `machineruntime.ValidMachineCode`).

---

## Database: `machines.code`

| Question | Answer |
|----------|--------|
| Column exists? | **Yes** — `machines.code` in `db/schema/01_platform.sql` |
| Unique? | **Yes** — `uniq_machines_code_lower ON machines (lower(code)) WHERE btrim(code) <> ''` |
| `GetMachineByCode` exists? | **Yes** — `db/queries/fleet.sql` |
| List joins `machine_code`? | **Yes** — `ListMachineActivationCodesForMachine`, `ListMachineActivationCodesPaged` join `machines m` |

---

## Response contract

**Create** (`writeAdminActivationCreateResponse`): `activationCode`, `activationCodeId`, `machineId`, `machineCode`, `expiresAt`, `maxUses`, `remainingUses`, `status`.

**List** (`writeAdminActivationListItem`): metadata only — no plaintext code, no `codeHash`, no tokens/MQTT password.

**Claim** (`postActivationClaim`): returns `machineId`, optional `machineCode`, `deviceAttachmentId` when attachment created, token/MQTT fields per existing contract.

---

## `MachineIdentityRef` struct

**Not present.** Resolver returns `(uuid.UUID, string, error)`. Planned refactor in Phase 2.

---

## Board replacement

Existing tests in `internal/app/activation/service_attachment_integration_test.go`:
- Same fingerprint reuses active attachment
- Different fingerprint replaces attachment
- `machines.code` unchanged after board replacement

Claim returns `deviceAttachmentId` in HTTP response when supported (`activation_http.go` lines 306–308).

---

## Surface inventory counts (source inspection)

From `tools/enterprise_flow/inventory_*.py` (report `20260703T013119Z`):

| Surface | Count | Notes |
|---------|-------|-------|
| REST OpenAPI operations | **347** | Committed swagger may differ; Chi-mounted **266** |
| REST Chi-only (missing OpenAPI) | **16+** | Planogram v2, ops-overview, device-attachments, etc. |
| gRPC services | **24** | **92 RPCs** total |
| gRPC contract-only (unimplemented) | **7** | Operator session + command poll/ack/reject |
| MQTT enterprise publish tails | **12** | `internal/platform/mqtt/topics.go` |
| MQTT ingest patterns | **13** | Includes `shadow/desired` (code-only exception) |

Accepted exceptions: `tools/enterprise_flow/accepted_surface_exceptions.json`.

---

## OpenAPI / Postman / E2E coverage

| Artifact | machineCode activation coverage |
|----------|--------------------------------|
| `swagger_operations.go` | All 6 machine + machine-codes activation paths documented |
| `openapi_spec_test.go` | P0 ops include machine-codes paths |
| `tests/e2e/production/e2e-manifest.yaml` | `REST-MACHINE-003` uses `/machine-codes/{{machineCode}}/activation-codes` |
| Postman production-full | `machineCode` env var; catalog body variants |
| `docs/runbooks/machine-activation.md` | Updated for machineCode workflow |

Production E2E scripts: bootstrap uses generated `AVF…` code; runtime uses resolved `machineId` UUID.

---

## Prior reports

| Path | Status |
|------|--------|
| `docs/reports/machine-code-activation-minimal/` | 5 files (pre-production minimal scope) |
| `docs/reports/machine-code-activation-production/` | **This audit — first file** |

---

## Gaps to close in this verification effort

1. Introduce `MachineIdentityRef` struct in resolver
2. Add missing HTTP tests (DELETE-by-code, snake_case body, `POST /machines/AVF…`)
3. Run full local + production REST/gRPC/MQTT verification with evidence
4. Write remaining production reports (01–08)
