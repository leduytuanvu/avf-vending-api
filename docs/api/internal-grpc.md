# Internal gRPC queries

This repository exposes an **internal-only** gRPC surface from `cmd/api` when `INTERNAL_GRPC_ENABLED=true`. This is separate from **public machine gRPC** (**`avf.machine.v1`**, toggled by **`MACHINE_GRPC_ENABLED`** / legacy **`GRPC_*`**).

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
- `InternalPaymentQueryService`
  - `GetPaymentById`
  - `GetLatestPaymentForOrder`
- `InternalCatalogQueryService`
  - `GetSaleCatalogSnapshot`
- `InternalInventoryQueryService`
  - `GetMachineSlotInventory`
- `InternalReportingQueryService`
  - `GetSalesSummary`

These RPCs are backed by the same `internal/app/*` and Postgres read paths used by the HTTP API. They do **not** replace public HTTP contracts, and they do **not** carry device/runtime traffic.

## Contract source

- Proto source: `proto/avf/internal/v1/*.proto`
- Generated Go stubs: `internal/gen/avfinternalv1/*.pb.go`
- Machine-facing public gRPC: `proto/avf/machine/v1/*.proto`; see [`../local/grpc-local-test.md`](../local/grpc-local-test.md)
- Transport wiring: `internal/grpcserver/*`

## Auth and exposure

- Internal gRPC is expected to bind to loopback by default (`127.0.0.1:9091`); production/staging validation rejects non-loopback exposure.
- Business RPCs require `authorization: Bearer <service JWT>` in gRPC metadata (`typ=service`, `aud=avf-internal-grpc`).
- User/Admin JWTs and Machine JWTs are rejected on this listener.
- `grpc.health.v1` remains available without Bearer auth for internal health checks.
- Reflection is dev/test-only by default and forbidden in production when internal gRPC is enabled.

## Local grpcurl examples

Set `INTERNAL_TOKEN` to a service token issued with `typ=service` and `aud=avf-internal-grpc`, then call the loopback listener:

```bash
grpcurl -plaintext \
  -H "authorization: Bearer ${INTERNAL_TOKEN}" \
  -d '{"organization_id":"11111111-1111-1111-1111-111111111111","payment_id":"77777777-7777-7777-7777-777777777777"}' \
  127.0.0.1:9091 avf.internal.v1.InternalPaymentQueryService/GetPaymentById

grpcurl -plaintext \
  -H "authorization: Bearer ${INTERNAL_TOKEN}" \
  -d '{"organization_id":"11111111-1111-1111-1111-111111111111","order_id":"44444444-4444-4444-4444-444444444444"}' \
  127.0.0.1:9091 avf.internal.v1.InternalPaymentQueryService/GetLatestPaymentForOrder
```

## Notes

- Machine and telemetry RPCs enforce organization access after loading the authoritative machine/snapshot organization.
- Commerce RPCs require `organization_id`, `order_id`, and `slot_index`, then reuse the existing commerce checkout-state query path.
- Payment RPCs require `organization_id`, then reuse existing payment/order read paths to enforce tenant ownership.
- Timestamps are returned as protobuf `Timestamp` values and should be serialized by clients as RFC3339 with timezone offset when rendered into logs or JSON.
