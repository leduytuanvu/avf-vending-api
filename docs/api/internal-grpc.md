# Internal gRPC queries

This repository now exposes an **internal-only** gRPC surface from `cmd/api` when `GRPC_ENABLED=true`.

## Scope

The current gRPC API is intentionally read-focused and service-to-service only:

- `InternalMachineQueryService`
  - `GetMachineSummary`
  - `GetMachineState`
  - `GetMachineCabinetSlotSummary`
- `InternalTelemetryQueryService`
  - `GetLatestMachineTelemetry`
  - `GetMachineIncidentSummary`
- `InternalCommerceQueryService`
  - `GetOrderPaymentVendState`

These RPCs are backed by the same `internal/app/*` and Postgres read paths used by the HTTP API. They do **not** replace public HTTP contracts, and they do **not** carry device/runtime traffic.

## Contract source

- Proto source: `proto/avf/v1/internal_queries.proto`
- Generated Go stubs: `proto/avf/v1/*.pb.go`
- Transport wiring: `internal/grpcserver/*`

## Auth and exposure

- gRPC is **internal-only** and expected to bind to a private address.
- Business RPCs require `authorization: Bearer <JWT>` in gRPC metadata.
- `grpc.health.v1` remains available without Bearer auth for internal health checks.
- Reflection is not registered by default.

## Notes

- Machine and telemetry RPCs enforce organization access after loading the authoritative machine/snapshot organization.
- Commerce RPCs require `organization_id`, `order_id`, and `slot_index`, then reuse the existing commerce checkout-state query path.
- Timestamps are returned as protobuf `Timestamp` values and should be serialized by clients as RFC3339 with timezone offset when rendered into logs or JSON.
