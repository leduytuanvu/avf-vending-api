# Admin operations API (P1.2)

Bearer JWT interactive admins only (`RequireDenyMachinePrincipal`). Paths are scoped under `/v1/admin/organizations/{organizationId}/…`; platform admins must supply `{organizationId}` in the path.

Machine-readable contract: OpenAPI tag **Operations** in `docs/swagger/swagger.json` (regenerate with `make swagger` after changing `swagger_operations.go` annotations). Static drift is enforced by **`make api-contract-check`** (see [api-contract-checks.md](./api-contract-checks.md)).

RBAC overview:

| Area | Read | Mutate |
|------|------|--------|
| Machine health / timeline | `fleet:read` **or** `telemetry:read` | — |
| Commands list/detail | `fleet:read` | — |
| Retry / cancel / dispatch MQTT commands | — | `machine:command` |
| Inventory anomalies list | `inventory:read` **or** `fleet:read` | — |
| Resolve anomaly / reconcile marker | — | `inventory:write` |

## Machine health

- **GET** `/operations/machines/health` — paginated health rows (`limit`/`offset` query via shared admin pagination helpers).
- **GET** `/machines/{machineId}/health` — single-machine projection.

Fields mirror ops dashboards: connectivity counts, MQTT session hint (`mqttConnected`), snapshot-derived versions (`configVersion`, `catalogVersion`, `mediaVersion` from `machine_current_snapshot.reported_state`), telemetry staleness (`telemetryFreshnessSeconds`, sentinel `-1` when snapshot timestamp unavailable), latest incident code (`lastErrorCode`), open inventory anomaly counts.

## Timeline

- **GET** `/machines/{machineId}/timeline` — merges `command_ledger`, `machine_command_attempts` (organization scoped), `order_timelines` via machine orders, and recent `machine_check_ins`.

## Commands

- **GET** `/commands` — delegates to existing fleet admin SQL (`FleetAdminListCommands`) using standard fleet list filters (`machine_id`, `from`, `to`, attempt status filters).
- **GET** `/commands/{commandId}` — ledger row plus ordered attempts (`dispatchState` derived via `MapAttemptTransportState`).
- **POST** `/commands/{commandId}/retry` — calls `MQTTCommandDispatcher.AdminRetryLedgerCommand`:
  - Requires non-empty `command_ledger.idempotency_key` (replay safety).
  - Rejects terminal successes/supersedes (`completed`, `duplicate`, `late`, `nack`).
  - Surfaces `503 capability_not_configured` when MQTT publisher absent (same semantics as other remote-command surfaces).
- **POST** `/commands/{commandId}/cancel` — marks open `pending`/`sent` attempts failed with `timeout_reason='admin_cancelled'`.
- **POST** `/machines/{machineId}/commands` — creates new ledger rows (`commandType`, optional JSON `payload`) with mandatory `Idempotency-Key` header (aligned with diagnostics dispatch patterns).

Enterprise audit hooks (`fleetAudit`) fire on retry/cancel/dispatch/anomaly resolve/reconcile marker mutations.

## Inventory anomalies

Detectors upsert rows idempotently via fingerprints:

| Type | Detector |
|------|----------|
| `negative_stock` | `machine_slot_state.current_quantity < 0` |
| `manual_adjustment_above_threshold` | `inventory_events` adjustments with `\|quantity_delta\| ≥ 50` within last 365 days |
| `stale_inventory_sync` | Published planogram version differs from snapshot acknowledgement |

Additional enum values remain reserved for future enrichments (`stock_mismatch_after_fill`, `vend_without_stock_decrement`, `slot_missing_product_but_stock`).

- **GET** `/inventory/anomalies` — optional `refresh=true` runs detectors before listing.
- **GET** `/machines/{machineId}/inventory/anomalies` — machine-filtered listing with optional refresh.
- **POST** `/inventory/anomalies/{anomalyId}/resolve` — closes open rows (`resolution_note` optional JSON `{ "note": "..." }`).
- **POST** `/machines/{machineId}/inventory/reconcile` — append-only `inventory_events` marker (`event_type=reconcile`) with metadata `{reason, source:"admin_api"}`.

Resolve/Reconcile flows rely on Postgres FK integrity (`resolved_by` optional UUID referencing platform accounts — parsed from JWT subject when UUID-shaped).

## Operational notes

- Command retry piggybacks on `AppendCommandUpdateShadow` replay semantics — never fabricates synthetic idempotency keys.
- Cancelling attempts does **not** delete ledger rows; it prevents further publishes until new attempts exist.
- Telemetry freshness derives from `machine_current_snapshot.updated_at`; absence yields sentinel `-1` seconds — omit client-side when unset.
