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

## Current production deployment posture

For this repository snapshot, the primary production deployment path is the split `2-VPS` layout under `deployments/prod/`:

- `app-node/` for stateless application runtime concerns
- `data-node/` for optional self-hosted broker/data-plane services that still remain in the interim topology
- `shared/` for shared edge config and release helpers

Managed dependency posture for that production path:

- PostgreSQL is expected to be managed and reached through `DATABASE_URL`
- Redis is expected to be managed when enabled and reached through env
- object storage is expected to be managed S3-compatible and reached through env
- only the broker-style fallback plane remains optionally self-hosted on `data-node/`

The older single-host compose path at `deployments/prod/docker-compose.prod.yml` is retained only as a legacy rollback option. It should not be treated as the primary production topology or as the recommended enterprise target deployment.

## What is explicitly not “done” just because the repo exists

- **Full product surface** for every vending scenario behind HTTP (admin and device flows grow incrementally).
- **End-to-end PSP / refund automation** from the reconciler as a default-on flow (reads/logging ship by default; probe/refund adapters are only active when `RECONCILER_ACTIONS_ENABLED=true`).
- **ClickHouse analytics** as a general live telemetry path (the current implementation is limited to the optional worker outbox mirror path).
- **Broad Temporal-driven business workflows** beyond the currently registered compensation/review set in `cmd/temporal-worker`.
- **Broad internal gRPC coverage** beyond the current query/read surface on the optional internal listener.
- **Multi-region / active-active** topology, global routing, or cross-region replication—**out of scope** for what this repo implements; any future runbook language is operational aspiration, not an implied feature here.

## Current Go repository snapshot (hard reference)

The following is **descriptive of the codebase**, not a promise that every optional integration is enabled in every environment:

| Area | Status |
| ---- | ------ |
| **Processes** | `cmd/api` (HTTP + optional internal gRPC query listener), `cmd/worker` (reliability + optional NATS outbox publish + telemetry consumers), `cmd/reconciler` (commerce reconciliation with optional actions mode), `cmd/mqtt-ingest` (MQTT → NATS/Postgres ingest), `cmd/temporal-worker` (Temporal compensation/review worker), `cmd/cli` (config / version). |
| **Postgres** | goose migrations under `migrations/`; sqlc under `db/schema/`, `db/queries/`, generated `internal/gen/db/`. |
| **Postgres OLTP patterns** | `internal/modules/postgres` implements durable **database** patterns (order + vend, payment + outbox, command + shadow ± outbox, receipts, MQTT ingest persistence) with idempotency aligned to schema — **not** Temporal workflow orchestration. |
| **Application layer** | `internal/app/*` owns commerce, device, fleet, reliability, and HTTP app composition; handlers in `internal/httpserver` stay thin. |
| **HTTP API** | `/health/*`, optional `/metrics`, JWT **`/v1`** admin lists, org-scoped lists, machine shadow access—see `internal/httpserver` and `internal/app/api`. |
| **NATS** | JetStream client, streams, outbox **publisher** in `internal/platform/nats`; **worker** enables it when `NATS_URL` is set. `cmd/worker` also runs telemetry JetStream consumers. There is still **no** in-repo consumer for worker outbox subjects. |
| **MQTT** | Subscriber/router in `internal/platform/mqtt`; **mqtt-ingest** publishes classified envelopes to **NATS JetStream** when `NATS_URL` is set (`telemetryapp` bridge), else legacy direct `postgres.Store`. **Worker** consumes telemetry streams and writes rollups/snapshots/incidents. See `ops/TELEMETRY_PIPELINE.md`. |
| **Object storage** | S3-compatible storage is **implemented for API artifacts** when `API_ARTIFACTS_ENABLED=true`; broader OTA/diagnostic usage is still follow-on work. |
| **Temporal** | `internal/platform/temporal` now backs a real `cmd/temporal-worker`; `cmd/api`, `cmd/worker`, and `cmd/reconciler` can schedule selected workflow follow-up behind `TEMPORAL_SCHEDULE_*` flags. |
| **ClickHouse** | Optional **worker** path for mirroring published outbox events when analytics flags are enabled; **not** yet the general telemetry analytics plane. |
| **gRPC** | `internal/grpcserver`: `grpc.health.v1` plus internal machine/telemetry/commerce query services when `GRPC_ENABLED=true`; auth uses Bearer JWT validation on gRPC metadata. `proto/avf/v1/internal_queries.proto` is the in-repo contract source. |
| **Tests** | Unit/config tests always-on; Postgres **integration** tests under `internal/modules/postgres` require `TEST_DATABASE_URL` and migrations (see root `README.md`). |

For the concise current-state freeze, see [`current-architecture.md`](current-architecture.md). When cutting over traffic from another system, update both documents and add links to runbooks, dashboards, and the exact routing flip checklist.

**Live ops notes** for this codebase: [`ops/RUNBOOK.md`](../../ops/RUNBOOK.md) (incidents, log queries, SQL) and [`ops/METRICS.md`](../../ops/METRICS.md).
