# Deployment topology (summary)

This document describes **how processes and dependencies relate** in production. Environment-specific hostnames, SSH targets, and managed-service IDs belong in operational runbooks (see links below)—not here.

## Logical planes

| Plane | Responsibility | Typical processes |
| ----- | ---------------- | ----------------- |
| **Control / Admin** | Interactive admins, integrations, OpenAPI contract | `cmd/api` HTTP `/v1/admin/*`, `/v1/auth/*`, commerce admin, reporting |
| **Machine runtime** | Native kiosk app RPCs (**Machine JWT**) | `cmd/api` **public** gRPC `avf.machine.v1` when `MACHINE_GRPC_ENABLED=true` |
| **Internal operations** | Service-to-service **read/query** RPCs (**service JWT**, loopback) | `cmd/api` optional `avf.internal.v1` when `INTERNAL_GRPC_ENABLED=true` |
| **Device realtime** | Telemetry ingress, backend→device commands | `cmd/mqtt-ingest`, MQTT broker; API publishes commands when MQTT publisher configured |
| **Async / durability** | Outbox publish, telemetry JetStream consumers, DLQ | `cmd/worker` + NATS JetStream when `NATS_URL` set |
| **Commerce hygiene** | Reconciliation, optional probes / refund enqueue | `cmd/reconciler` (+ optional Temporal when enabled) |

## Primary production shape (reference implementation)

The repository’s **primary** documented production path is a **split node** layout under `deployments/prod/`:

- **App node**: stateless Go binaries (`api`, `worker`, `reconciler`, `mqtt-ingest`, optional `temporal-worker`) behind TLS termination (e.g. Caddy).
- **Data node** (optional): broker-plane fallback (e.g. EMQX / NATS) when not using fully managed equivalents.

Authoritative operational detail (ports, compose profiles, migration hooks): **[`../runbooks/production-2-vps.md`](../runbooks/production-2-vps.md)**.

## Managed dependencies (typical)

- **PostgreSQL**: system of record (`DATABASE_URL`).
- **Redis**: optional cache, revocation, locks, HTTP/grpc limits (`REDIS_*`) — not SoR.
- **Object storage**: S3-compatible artifacts/media when enabled (`API_ARTIFACTS_*`, catalog media pipelines).
- **MQTT broker**: device publish/subscribe; TLS in production.
- **NATS JetStream**: telemetry buffering and outbox paths when required by env (`NATS_REQUIRED`, `API_REQUIRE_*` guards in bootstrap).

See also: [`transport-boundary.md`](transport-boundary.md), [`current-architecture.md`](current-architecture.md).
