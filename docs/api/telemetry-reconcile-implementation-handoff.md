# Telemetry application-level reconcile / status — implementation handoff

**Implementation status:** **Shipped** — `internal/httpserver/telemetry_reconcile_http.go` (`POST .../events/reconcile`, `GET .../events/{idempotencyKey}/status`), mounted from `internal/httpserver/server.go`.

**Goal (original):** Mounted HTTP APIs so devices can confirm **business-level** persistence of critical telemetry by `idempotency_key` / `dedupe_key`, independent of MQTT **PUBACK**.

## Why this exists

- Documented gap: [`docs/api/mqtt-contract.md`](mqtt-contract.md) § *Application-level ACK* — **QoS 1 `PUBACK` is not an application ACK**; see also [`internal/app/telemetryapp/critical_telemetry_ack_doc_test.go`](../../internal/app/telemetryapp/critical_telemetry_ack_doc_test.go).
- **Device rule:** remove a critical event from the **durable local outbox** only when reconcile returns **`status=processed`** (see contract below).

## Required routes

| Method | Path | Auth |
|--------|------|------|
| POST | `/v1/device/machines/{machineId}/events/reconcile` | Machine JWT for same machine, or user JWT with machine access (align with device bridge policy — today [`device_http.go`](../../internal/httpserver/device_http.go) may still be admin-only; **P0** includes allowing machine token on device paths per activation/tenant plan). |
| GET | `/v1/device/machines/{machineId}/events/{idempotencyKey}/status` | Same |

**Path encoding:** `idempotencyKey` may contain `:`. Use **URL-escaped** path segments on clients; server **`PathUnescape`** before lookup. If friction is high, add alternate **`GET .../events/status?idempotency_key=`** (document both; OpenAPI can show query variant as primary).

## Request / response shapes

Match the product brief (camelCase JSON consistent with other `/v1` handlers).

**Batch body:**

```json
{
  "idempotencyKeys": ["key1", "key2"]
}
```

- Length **1–500**; each key non-empty, max length e.g. **512** (align with `device_telemetry_events.dedupe_key` / envelope limits).
- Empty array → **400** `invalid_argument`.

**Item `status` enum (canonical):**

- `not_found` — no durable record for this machine + key (device should **retry** with backoff unless max attempts).
- `accepted` — accepted at API/ingest boundary but OLTP projection not complete (optional; see storage).
- `processing` — worker owns the message (optional).
- `processed` — **safe to delete from device outbox**; duplicate replays must not double-apply (existing `AppendDeviceTelemetryEdgeEvent` / inventory dedupe).
- `failed_retryable` — **`retryable: true`**; transient failure.
- `failed_terminal` — **`retryable: false`**; operator alert.

Map internal storage to **`eventType`** when known (e.g. `events.vend`); else `null`.

## Security

- Resolve principal → must **only** return data for **`machineId` in the URL** matching token scope.
- If a key exists **only** on another machine: return **`not_found`** for that item (no cross-tenant / cross-machine leak). Optionally increment `telemetry_reconcile_forbidden_total` when the raw key “looks” bound to a different machine **only if** that can be done without oracle; otherwise rely on machine-scoped queries only.
- Invalid JWT → **401**; wrong machine scope → **403**.

## Storage strategy

### Option A — Minimal (fast P0)

Derive status only from existing OLTP tables (no migration):

- **`device_telemetry_events`:** `(machine_id, dedupe_key)` unique partial index already exists — if row present → **`processed`** for edge-style events ([`AppendDeviceTelemetryEdgeEvent`](../../internal/modules/postgres/telemetry_store.go)).
- **`inventory_events`:** `metadata->>'idempotency_key'` + `machine_id` ([`AppendInventoryEventFromDeviceTelemetry`](../../internal/modules/postgres/telemetry_store.go)).
- **`device_command_receipts`:** `dedupe_key` + `machine_id`.
- **`incidents`:** if dedupe is stored on upsert path, include in resolver.

**Limitation:** cannot distinguish **`accepted` vs `processing` vs in-flight JetStream** without extra state; for those, return **`not_found`** with **`retryable: true`** until OLTP row appears (conservative, matches “don’t drop from outbox early”).

### Option B — Explicit state machine (recommended for spec completeness)

Add migration **`machine_event_statuses`**:

- `id` (uuid), `organization_id`, `machine_id`, `idempotency_key` (text), `event_type` (text, nullable)
- `status` (text CHECK), `accepted_at`, `processed_at`, `failure_reason` (text), `created_at`, `updated_at`
- **UNIQUE (`machine_id`, `idempotency_key`)**

**Writers:**

1. **mqtt-ingest / bounded pipeline:** when a **critical** message is validated and accepted into the durable path, **upsert** `accepted` + `accepted_at` (optional, requires hook in [`bounded_pipeline.go`](../../internal/app/telemetryapp/bounded_pipeline.go) or JetStream publish success path).
2. **Worker** ([`jetstream_workers.go`](../../internal/app/telemetryapp/jetstream_workers.go)): at start of handle → `processing`; after successful `Append*` → `processed` + `processed_at`; on terminal projection error → `failed_terminal` + reason; on retryable error → `failed_retryable`.

**Reconcile handler** reads this table first; falls back to Option A for legacy rows if backfilling is not done.

## Idempotency key alignment

Devices must use the **same** string the backend stores:

- Critical MQTT path: envelope **`idempotency`** / payload **`dedupe_key`**; stable derivation in [`internal/platform/telemetry/critical_idempotency.go`](../../internal/platform/telemetry/critical_idempotency.go) and tests in [`offline_replay_contract_test.go`](../../internal/platform/mqtt/offline_replay_contract_test.go).

Document in [`mqtt-contract.md`](mqtt-contract.md) that reconcile keys **must** match OLTP `dedupe_key` / envelope idempotency.

## Prometheus metrics

Register with **`promauto`** (same pattern as [`mqtt_ingest_prom.go`](../../internal/app/telemetryapp/mqtt_ingest_prom.go)):

- `avf_telemetry_reconcile_requests_total` (counter)
- `avf_telemetry_reconcile_items_total{status}` (counter vec)
- `avf_telemetry_reconcile_not_found_total` (counter)
- `avf_telemetry_reconcile_forbidden_total` (counter)

HTTP handler increments; labels must stay low-cardinality.

## HTTP implementation notes

- New file e.g. [`internal/httpserver/telemetry_reconcile_http.go`](../../internal/httpserver/telemetry_reconcile_http.go): `mountTelemetryReconcileRoutes(r, app, deps)`.
- Wire in [`server.go`](../../internal/httpserver/server.go) under `/v1/device/machines/{machineId}` alongside vend-results/poll (same auth + rate-limit group as other device POSTs if applicable).
- Service layer: [`internal/app/telemetryapp/reconcile_service.go`](../../internal/app/telemetryapp/reconcile_service.go) (batch + single status) calling postgres/sqlc.

## Ingest enforcement

- **Critical** events without identity already increment **`telemetry_ingest_critical_missing_identity_total`** — extend tests so **missing idempotency** on required critical types is **rejected** before silent drop (see existing worker errors for `events.vend` / `events.cash`).

## OpenAPI

- Add `DocOp*` blocks in [`swagger_operations.go`](../../internal/httpserver/swagger_operations.go).
- Add to **`REQUIRED_OPERATIONS`** in [`tools/build_openapi.py`](../../tools/build_openapi.py).
- `make swagger` && `make swagger-check`.

## Tests

| Case | Expect |
|------|--------|
| Processed edge event | `processed`, `retryable: false` |
| Unknown key | `not_found`, `retryable: true` |
| `failed_retryable` / `failed_terminal` | flags as specified |
| Cross-machine token | **403** on route; batch items never leak other machines |
| Batch size 501 | **400** |
| Duplicate critical replay | still **one** OLTP row ([`telemetry_idempotency_integration_test.go`](../../internal/modules/postgres/telemetry_idempotency_integration_test.go) patterns) |

## Doc updates (ship with code)

- [`docs/api/mqtt-contract.md`](mqtt-contract.md): replace “gap — P0” with links to these routes; restate **outbox deletion only on `processed`**.
- [`docs/api/api-surface-audit.md`](api-surface-audit.md): mark reconcile **implemented** when mounted.
- [`docs/README.md`](README.md): link this handoff until the feature is merged.

## Acceptance

```text
Routes in Chi + tests + go test ./...
make swagger && make swagger-check
```

---

**Plan mode:** this file is the executable spec; implementation requires Agent mode or manual apply.
