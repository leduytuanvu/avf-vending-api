# Machine setup (bootstrap, topology, planogram)

All paths are under **`/v1`**, Bearer JWT, roles **`platform_admin`** or **`org_admin`** (plus machine access where noted). `platform_admin` must pass **`organization_id`** query on several admin routes—match your deployment’s tenancy rules.

## Fleet directory (sites, machines, technician assignments)

Organization-scoped admin APIs live under **`/v1/admin/organizations/{organizationId}/…`** (see OpenAPI). **`fleet_manager`** can mutate fleet writes (**`fleet:write`**); **`support`** is read-only on fleet lists where RBAC grants **`fleet:read`**. **`technician`** interactive JWTs must not pass fleet write middleware—assignments are performed by org/fleet admins only. Sites archive softly (**`status`: `active` \| `archived`**); machines archive (**retired**), suspend, or mark compromised without deleting historical rows. Technician machine bindings use **`technicians`** IDs on **`POST …/machines/{machineId}/technicians`** (`userId` path segment is the technician row UUID); self-assignment when the JWT carries a matching **`technician_id`** claim is rejected.

## GET `/v1/setup/machines/{machineId}/bootstrap`

- **Auth**: `RequireMachineURLAccess("machineId")` (machine on JWT).
- **Response**: `machine`, `topology.cabinets[]` with nested `slots[]`, `catalog.products[]` (assortment lines for assignment).
- **Copy-paste**: see OpenAPI **example** on this path in `docs/swagger/swagger.json` (`machineId` in path only; no body).
- **Fingerprints** (`catalogFingerprint`, `pricingFingerprint`, `planogramFingerprint`, `mediaFingerprint`) are stable hashes used with **CheckForUpdates** / gRPC parity.
- **Published planogram**: when enterprise versioning is in use, **`publishedPlanogramVersionId`** / **`publishedPlanogramVersionNo`** reflect **`machines.published_planogram_version_id`** (immutable history lives in **`machine_planogram_versions`**). Draft edits alone do not move these fields until **`POST …/planogram/drafts/{draftId}/publish`**.

## Enterprise planogram (draft → validate → publish → versions → rollback)

Organization-scoped routes under **`/v1/admin/organizations/{organizationId}/machines/{machineId}/`**:

| Method | Path | Notes |
| ------ | ---- | ----- |
| GET | `planogram` | Published pointer + draft rows (`snapshot` JSON per draft). |
| POST | `planogram/drafts` | Body `{ "snapshot": { … } }` — same slot shape as legacy draft (`planogramId`, `planogramRevision`, `items[]`). Does **not** change **`machine_slot_configs`** until publish. |
| PATCH | `planogram/drafts/{draftId}` | Replace snapshot and/or `status` (`editing` \| `validated`). |
| POST | `planogram/drafts/{draftId}/validate` | Validates assortment, topology, duplicates; sets draft **`validated`**. |
| POST | `planogram/drafts/{draftId}/publish` | Writes immutable **`machine_planogram_versions`** row, updates **`machines.published_planogram_version_id`**, applies **current** slot configs, bumps **`machine_configs`**, dispatches **`machine_planogram_publish`** when MQTT is wired. |
| GET | `planogram/versions` | Lists immutable versions (newest first). |
| POST | `planogram/versions/{versionId}/rollback` | Repoints published pointer to an existing version and reapplies runtime configs + config snapshot. |

**Audit**: publish and rollback emit **`machine.planogram_publish`** / **`machine.planogram_rollback`** on **`audit_events`** when enterprise audit is configured.

**Deferred (P1)**: cloning a draft from another machine or from **`planogram_templates`** — not implemented in this iteration; use explicit snapshot JSON from the source machine’s **`GET planogram/versions`** export path when needed.

## PUT `/v1/admin/machines/{machineId}/topology`

- **Body**: `operator_session_id` (UUID, **ACTIVE** session on this machine), `cabinets[]` (`code`, `title`, `sortOrder`, optional `metadata` object), `layouts[]` (`cabinetCode`, `layoutKey`, `revision`, `layoutSpec` object, `status`).
- **Response**: **204 No Content** on success.
- **Example**: OpenAPI request body example on this path.

## PUT `/v1/admin/machines/{machineId}/planograms/draft`

- **Body**: `operator_session_id`, `planogramId` (UUID string), `planogramRevision`, `syncLegacyReadModel` (bool), `items[]` per slot (`cabinetCode`, `layoutKey`, `layoutRevision`, `slotCode`, optional `legacySlotIndex`, optional `productId`, `maxQuantity`, `priceMinor`, optional `metadata`).
- **Response**: **204** on success.
- **Example**: OpenAPI request body example (same shape used for publish).

## POST `/v1/admin/machines/{machineId}/planograms/publish`

- **Headers**: **`Idempotency-Key`** (or `X-Idempotency-Key`) required.
- **Body**: same slot assignment shape as draft (`operator_session_id`, `planogramId`, `planogramRevision`, `syncLegacyReadModel`, `items[]`).
- **Response**: `desiredConfigVersion`, `planogramId`, `planogramRevision`, `command` (`commandId`, `sequence`, `dispatchState`, `replay`) after enqueueing **`machine_planogram_publish`** to MQTT command path (or **503** if MQTT publisher not configured).
- **Example**: OpenAPI request + **200** response example on this path.

## Commerce checkout (related)

- **QR / PSP**: `POST /v1/commerce/orders` → `payment-session` → provider webhook → vend commands → device path.
- **Cash**: `POST /v1/commerce/cash-checkout` (same totals body as create order; marks paid with provider **`cash`**).
- **Device vend result HTTP**: `POST /v1/device/machines/{machineId}/vend-results` (see `docs/api/mqtt-contract.md`). Headers: **`Idempotency-Key`**. Success body example is in OpenAPI; failure outcome (no inventory decrement):

```json
{
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "slot_index": 3,
  "outcome": "failed",
  "failure_reason": "motor_timeout",
  "correlation_id": "11111111-2222-3333-4444-555555555555"
}
```

Use Swagger UI (`/swagger/index.html` when enabled) or `docs/swagger/swagger.json` for exact field names and enums.
