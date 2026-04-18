# Migration strategy (strangler fig)

## Goal

Evolve this repository into an **enterprise-grade** backend without a risky “big bang” rewrite—using a **modular monolith** and **multiple processes** (`cmd/*`) that share libraries and Postgres, not premature microservice sprawl.

## Non-negotiable rule: do not delete a legacy system early

If a **prior implementation** (another repo, an older stack, or a parallel deployment) is still serving production traffic, it remains **valid until feature parity and operational confidence** exist for each slice you migrate.

This Go codebase is **additive** relative to any such system:

- It introduces layout under `cmd/` and `internal/`.
- Cutover is **incremental** and reversible at the routing and traffic layer.

*(Historical note: an earlier iteration of the vending system used Node/Nest + Prisma in a different tree. This repository is **Go-first** today; strangler steps still apply whenever traffic is moved from **any** legacy source.)*

## Strangler sequence (recommended)

1. **Bootstrap** the Go runtime, configuration, observability, and local dependency stack.
2. **Edge routing**: gateway or path-based routing sends a **small, low-risk** subset of traffic to Go (health, read-only metadata, or a single bounded context).
3. **Vertical slices**: migrate one bounded context at a time (identity, machines, checkout, etc.), keeping contracts stable at the HTTP boundary during each slice.
4. **Parity gates**: each slice requires automated tests + operational checks (metrics/alerts) before increasing traffic percentage.
5. **Decommission**: only after parity + soak + rollback drills, retire the legacy implementation for that slice.

## Coexistence rules

- Keep **API compatibility** stable for external consumers during migration (versioning where needed).
- Prefer **additive schema changes** in Postgres; avoid destructive migrations without a two-phase rollout plan.
- Treat **idempotency** and **auditability** as first-class requirements for any dual-write or replay-heavy transition.

## Rollback posture

Every increment should ship with:

- a **routing rollback** (send traffic back to the prior system),
- a **data safety story** (what happens if both systems write),
- and **operational visibility** (dashboards/alerts proving the slice is healthy).

## What is explicitly not “done” just because the repo exists

- **Full product surface** for every vending scenario behind HTTP (admin and device flows grow incrementally).
- **End-to-end PSP / refund automation** from the reconciler (reads and scheduling exist; external gateways are not wired in `cmd/reconciler` today).
- **ClickHouse analytics** as a live path (`internal/platform/clickhouse` is a **future** sink: noop when disabled, **fail-fast error** from `Open` when enabled—no driver, no writes).
- **Temporal-driven business workflows** in process startup (SDK wrapper exists; no workflow registration in `cmd/*` yet).
- **Domain gRPC** beyond **grpc.health.v1** on the optional internal listener.
- **Multi-region / active-active** topology, global routing, or cross-region replication—**out of scope** for what this repo implements; any future runbook language is operational aspiration, not an implied feature here.

## Current Go repository snapshot (hard reference)

The following is **descriptive of the codebase**, not a promise that every optional integration is enabled in every environment:

| Area | Status |
| ---- | ------ |
| **Processes** | `cmd/api` (HTTP + optional gRPC health-only listener), `cmd/worker` (reliability + optional NATS outbox publish), `cmd/reconciler` (commerce reconciliation **reads**), `cmd/mqtt-ingest` (MQTT → Postgres), `cmd/cli` (config / version). |
| **Postgres** | goose migrations under `migrations/`; sqlc under `db/schema/`, `db/queries/`, generated `internal/gen/db/`. |
| **Postgres OLTP patterns** | `internal/modules/postgres` implements durable **database** patterns (order + vend, payment + outbox, command + shadow ± outbox, receipts, MQTT ingest persistence) with idempotency aligned to schema — **not** Temporal workflow orchestration. |
| **Application layer** | `internal/app/*` owns commerce, device, fleet, reliability, and HTTP app composition; handlers in `internal/httpserver` stay thin. |
| **HTTP API** | `/health/*`, optional `/metrics`, JWT **`/v1`** admin lists, org-scoped lists, machine shadow access—see `internal/httpserver` and `internal/app/api`. |
| **NATS** | JetStream client, streams, outbox **publisher** in `internal/platform/nats`; **worker** enables it when `NATS_URL` is set. Consumer helpers exist; **no** `cmd/*` JetStream subscriber in this repo yet. |
| **MQTT** | Subscriber/router in `internal/platform/mqtt`; **mqtt-ingest** publishes classified envelopes to **NATS JetStream** when `NATS_URL` is set (`telemetryapp` bridge), else legacy direct `postgres.Store`. **Worker** consumes telemetry streams and writes rollups/snapshots/incidents. See `ops/TELEMETRY_PIPELINE.md`. |
| **Object storage** | S3-compatible **library** in `internal/platform/objectstore`—implemented but **not referenced** from `internal/app` or `cmd/*` yet. |
| **Temporal** | **SDK wrapper only** in `internal/platform/temporal`; **no** workflow worker or client dial from `cmd/*`. |
| **ClickHouse** | **Future path**; `internal/platform/clickhouse` is noop DI when disabled, **error if enabled** (no half-connected client). Not imported from `cmd/*` or bootstrap. |
| **gRPC** | `internal/grpcserver`: **grpc.health.v1 only** by default; `bootstrap.RunAPI` passes no `ServiceRegistrar`. `proto/avf/v1/skeleton.proto` is buf/codegen smoke only — **not** mounted as a service. |
| **Tests** | Unit/config tests always-on; Postgres **integration** tests under `internal/modules/postgres` require `TEST_DATABASE_URL` and migrations (see root `README.md`). |

When cutting over traffic from another system, update this snapshot and add links to runbooks, dashboards, and the exact routing flip checklist.

**Live ops notes** for this codebase: [`ops/RUNBOOK.md`](../../ops/RUNBOOK.md) (incidents, log queries, SQL) and [`ops/METRICS.md`](../../ops/METRICS.md).
