# Telemetry and device ingest pipeline

This document describes how **high-frequency device traffic** is handled without overloading Postgres OLTP, aligned with the **~1000 machine / lean VPS** posture.

## Classification

| Class | MQTT / source | JetStream buffer (`AVF_TELEMETRY_*`) | Postgres |
|-------|----------------|----------------------------------------|------------|
| `heartbeat` | `.../telemetry` (`event_type` heartbeat / health.*) | ~2h max age, bounded bytes | `machine_current_snapshot.last_heartbeat_at` only |
| `state` | `.../shadow/reported` | ~6h | `machine_shadow` + `machine_current_snapshot` + `machine_state_transitions` (meaningful diffs) |
| `metrics` | `.../telemetry` (metrics.*, default unknown) | ~6h | `telemetry_rollups` (`1m` buckets) — **no raw rows** |
| `incident` | `.../telemetry` (`incident.*`, `alert.*`) | ~24h | `machine_incidents` (deduped when `dedupe_key` set) |
| `command_receipt` | `.../commands/receipt` | ~72h | `device_command_receipts` + command ledger transitions (existing workflow) |
| `diagnostic_bundle_ready` | `.../telemetry` (`diagnostic.*`) | ~7d | `diagnostic_bundle_manifests` (metadata only; **blobs in object storage**) |

## What must **not** land in hot Postgres

- Raw per-message heartbeat / metrics history (the legacy `device_telemetry_events` table is **not** populated by the NATS bridge path).
- Debug / trace / serial dumps (stay on device unless an explicit diagnostic workflow runs).
- Long-term MQTT payloads (NATS JetStream is the short buffer; retention is **age + max bytes** per stream).

## NATS JetStream policy

Streams are created/updated in `internal/platform/nats/telemetry_streams.go`:

- **Limits** retention (not interest-based) with `DiscardOld`.
- **MaxAge** per class (heartbeat 2h … diagnostic 7d).
- **MaxBytes** per stream (currently **256 MiB** each) so disk cannot grow without bound on a small VPS.

## Processes

1. **`cmd/mqtt-ingest`** — subscribes to MQTT, classifies, publishes envelopes to JetStream (`TELEMETRY_*` streams). When `NATS_URL` is unset, it falls back to **legacy** direct Postgres ingest (warned in logs).
2. **`cmd/worker`** — ensures telemetry streams + durable consumers, runs **pull consumers** (`internal/app/telemetryapp/jetstream_workers.go`), and runs a **daily retention** tick (`postgres.RunTelemetryRetention`) when `RetentionTick` / `TelemetryRetention` are configured.

## Postgres retention defaults (`RunTelemetryRetention`)

| Data | Default retention |
|------|-------------------|
| Financial / operator / audit OLTP | **Not** pruned by this job |
| `machine_state_transitions` | 60 days |
| `machine_incidents` low/medium/info | 90 days |
| `machine_incidents` high/critical | 180 days |
| `telemetry_rollups` granularity `1m` | 30 days |
| `telemetry_rollups` granularity `1h` | 180 days (rows must exist — see gaps below) |
| `diagnostic_bundle_manifests` | 365 days |

## HTTP read APIs (not raw streams)

Mounted under `/v1/machines/{machineId}/telemetry/*` with `RequireMachineURLAccess`:

- `GET .../snapshot` — current projected snapshot.
- `GET .../incidents` — recent persisted incidents.
- `GET .../rollups` — rollup buckets only (`granularity=1m|1h`, `from`/`to` RFC3339).

## Honest gaps / follow-ups

- **Coarse (`1h`) rollup materialization** from `1m` is **not** implemented yet; only the schema and read path accept `granularity=1h`.
- **Object storage HEAD verification** for diagnostic manifests is **not** wired; manifests assume the device/uploader already placed bytes at `storage_key`.
- **MQTT TLS** and **per-tenant NATS accounts** remain deployment concerns outside this document.
