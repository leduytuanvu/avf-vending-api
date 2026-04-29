# Internal gRPC (split-ready, monolith-only)

This document describes the **optional second gRPC listener** for read-only, service-to-service queries. It exists so the modular monolith can later extract workers or auxiliary services without redesigning contracts—**no microservices or distributed transactions are introduced in this phase**.

## Listener and configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `INTERNAL_GRPC_ENABLED` | `false` | When true, the process listens for `avf.internal.v1` RPCs. |
| `INTERNAL_GRPC_ADDR` | `127.0.0.1:9091` | Bind address. Staging/production validation requires a loopback host so the port is not publicly exposed by default. |
| `INTERNAL_GRPC_SERVICE_TOKEN_SECRET` | (empty) | HS256 secret for internal bearer tokens. In `development` / `test`, the verifier may fall back to `HTTP_AUTH_JWT_SECRET` when this is unset (not for production). |
| `INTERNAL_GRPC_SHUTDOWN_TIMEOUT` | `15s` | Graceful shutdown budget. |
| `INTERNAL_GRPC_UNARY_HANDLER_TIMEOUT` | `60s` | Default unary deadline when the client does not set one. |
| `INTERNAL_GRPC_HEALTH_ENABLED` | `true` | Registers `grpc.health.v1` on the internal server. |
| `INTERNAL_GRPC_REFLECTION_ENABLED` | `true` in dev/test | **Forbidden in production** when enabled together with internal gRPC (config validation). |

The public machine/runtime listener remains `GRPC_*` (`GRPC_ENABLED`, `GRPC_ADDR`, …) and serves `avf.machine.v1` with **machine JWT** on protected RPCs. **Do not confuse the two.**

## Authentication

Internal query RPCs require a **dedicated service bearer JWT**, not admin user JWT and not machine JWT.

- **Audience (`aud`)**: `avf-internal-grpc`
- **Type (`typ`)**: `service`
- **Role**: `service` (same role string as other platform service principals)

Issue tokens with [`IssueInternalServiceAccessJWT`](../../internal/platform/auth/internal_grpc_token.go) (or an equivalent operator process) using `INTERNAL_GRPC_SERVICE_TOKEN_SECRET`. Handlers additionally enforce tenant scope: a service token with `org_id` may only read data for that organization (platform-wide tokens omit `org_id`).

### mTLS (future)

The interceptor stack is **Bearer-first** and documented as **mTLS-ready**: a future phase can add client certificate validation in front of or alongside the unary chain without changing application query ports.

## Proto package

Protobuf sources live under `proto/avf/internal/v1/`. **Go stubs** are generated into `internal/gen/avfinternalv1/` (not next to the `.proto` files) because a directory named `internal` on the import path is subject to Go’s internal visibility rules and cannot be imported from `internal/grpcserver` and other packages.

- `catalog_query.proto` — `InternalCatalogQueryService`
- `inventory_query.proto` — `InternalInventoryQueryService`
- `commerce_query.proto` — `InternalCommerceQueryService`
- `payment_query.proto` — `InternalPaymentQueryService`
- `reporting_query.proto` — `InternalReportingQueryService`
- `machine_query.proto` — `InternalMachineQueryService`, `InternalTelemetryQueryService`

Regenerate with `make proto` (runs `buf generate --exclude-path avf/internal` then `buf generate --template buf.gen.avfinternal.yaml --path avf/internal/v1`).

Catalog and reporting responses return **UTF-8 JSON strings** matching existing HTTP/reporting DTO shapes (no binary media on gRPC).

## Implementation map (monolith)

- Server construction: [`NewInternalGRPCServer`](../../internal/grpcserver/internal_grpc_listen.go)
- Auth interceptor: [`unaryInternalServiceTokenAuthInterceptor`](../../internal/grpcserver/internal_grpc_interceptors.go)
- Handlers reuse **existing** app ports (`internal/app/api` query services, payment/order read paths, `salecatalog.SnapshotBuilder`, `ReportingService`) via [`RegisterInternalQueryServices`](../../internal/grpcserver/internal_queries.go)—**no duplicated SQL**.

Bootstrap wires the internal server in [`internal/bootstrap/api.go`](../../internal/bootstrap/api.go) when `INTERNAL_GRPC_ENABLED=true`, in parallel with HTTP and the public gRPC server.

## Extraction path (later)

1. Move `RegisterInternalQueryServices` and its deps behind a small **composition root** in the extracted process.
2. Keep the same protos and package `avf.internal.v1` for wire compatibility.
3. Point callers at the same loopback or private network address; prefer **mTLS + network policy** instead of exposing the listener on a public interface.
4. Replace in-process calls with gRPC only at process boundaries; **do not** fork business logic into new handlers.

## Testing

`internal/grpcserver` tests cover: missing token, wrong org scope, rejection of user and machine JWTs, successful read-only calls, unary-only service descriptors, and `Get*`-only method naming.
