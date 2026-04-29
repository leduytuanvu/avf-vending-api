# Fleet rollout & bulk provisioning

Tenant-scoped admin APIs drive **bulk machine provisioning** (batch metadata + optional activation codes) and **MQTT-backed fleet rollouts** (`fleet_rollout_apply` via the command ledger).

## Bulk provisioning

| Method | Path | Permission |
| --- | --- | --- |
| `POST` | `/v1/admin/organizations/{organizationId}/provisioning/machines/bulk` | `fleet:write` |
| `GET` | `/v1/admin/organizations/{organizationId}/provisioning/batches/{batchId}` | `fleet:read` |

- Machines are created through the existing fleet validation rules (`status` defaults to `provisioning`).
- Optional activation codes return plaintext **once** in the bulk response (same semantics as single-machine flows).
- `GET …/batches/{batchId}` exposes activation **status** fields joined from activation codes for operational export/review.

## Fleet rollout campaigns

Rollouts mutate remote intent by appending MQTT commands that bump `desired_state` keys:

| `rollout_type` | Shadow key updated |
| --- | --- |
| `config_version` | `config_version` |
| `catalog_version` | `catalog_version` |
| `media_version` | `media_version` |
| `planogram_version` | `planogram_version` |
| `app_version` | `app_version` (desired shadow pin for kiosk builds) |

### Strategy JSON (`strategy`)

| Field | Meaning |
| --- | --- |
| `machine_ids` | Explicit machine UUID list (highest precedence). |
| `site_ids`, `statuses`, `model` | Filter organization machines (`model` is substring match). |
| `tag_ids` | Machine must have **all** listed catalog tag UUIDs (`machine_tag_assignments`). |
| `tag_slugs` | Same as `tag_ids`, resolved case-insensitively on `tags.slug` within the tenant. Unknown slug → `400 invalid_argument`. |
| `canary_percent` | Float **1–99**: selects the first *k* machines (stable UUID sort) after filters, where `k = ceil(n * pct / 100)` (minimum **1** machine when *n > 0*). |
| `confirm_full_rollout` | Must be **true** to target **all** filtered machines without `machine_ids` or canary (guards accidental full-fleet pushes). |
| `rollback_version` | Required before **Rollback**: prior bundle/version string applied via **new** `fleet_rollout_apply` commands (no silent shadow patching). |

Tag filters AND with site/status/model filters. Assign tags to machines by inserting rows into **`machine_tag_assignments`** (migration `00070_p14_rollout_machine_tags_app_version.sql`).

### Lifecycle endpoints

All under `/v1/admin/organizations/{organizationId}/rollouts…`, `fleet:write` except `GET` (`fleet:read`):

| Action | Behavior |
| --- | --- |
| `POST /rollouts` | Creates `pending` campaign (no dispatch yet). |
| `POST …/{rolloutId}/start` | Resolves targets, persists rows, enters `running`, dispatches MQTT chunks until paused/cancelled or work completes. |
| `POST …/{rolloutId}/pause` | Sets `paused`; dispatch loops exit cleanly. |
| `POST …/{rolloutId}/resume` | Sets `running` and resumes dispatch. |
| `POST …/{rolloutId}/cancel` | Terminal cancel + skips pending targets. |
| `POST …/{rolloutId}/rollback` | Moves succeeded targets back to pending under rollback phase using `rollback_version`. |

Target rows advance via **command ledger attempts** (`RolloutRefreshTargetFromLatestAttempt` mirrors ACK/NACK/timeouts).

## Command ledger operations

Tenant-scoped listing and operator retry/cancel (mounted on the organization admin router):

| Method | Path | Permission |
| --- | --- | --- |
| `GET` | `/v1/admin/organizations/{organizationId}/commands` | `fleet:read` |
| `GET` | `/v1/admin/organizations/{organizationId}/commands/{commandId}` | `fleet:read` |
| `POST` | `/v1/admin/organizations/{organizationId}/commands/{commandId}/retry` | `device_commands:write` |
| `POST` | `/v1/admin/organizations/{organizationId}/commands/{commandId}/cancel` | `device_commands:write` |

- **Retry** re-dispatches when the latest attempt is not terminal (`completed`, `nack`, `duplicate`, `late`); requires a persisted **idempotency key** on the command. Wrong-org access returns **404**.
- **Cancel** marks open `pending`/`sent` attempts as **`admin_cancelled`** (best-effort operator halt).

Separate **health / timeline / dispatch** helpers remain under `…/operations/…`.

### Operational prerequisites

- Machines must be **`active`** for remote dispatch (`MQTTCommandDispatcher` rejects other statuses).
- `machine_shadow` must exist before rollout dispatch; first command append normally materializes shadow—if a target shows `shadow_missing`, seed or drive a noop command once.
- Audit events for rollout lifecycle use **mandatory** enterprise audit (`RecordCritical`) on `rollout_campaigns` (aligned with other admin control planes).

## Related runbooks

- `docs/runbooks/canary-rollout.md`
- `docs/runbooks/mqtt-command-stuck.md`
