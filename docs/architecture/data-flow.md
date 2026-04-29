# Data flow overview

High-level sequences for the **implemented** transports. OpenAPI details live in **`docs/swagger/swagger.json`**; machine RPC contracts in **`proto/avf/machine/v1`**; MQTT shapes in **[`../api/mqtt-contract.md`](../api/mqtt-contract.md)**.

## Admin REST: login and Bearer session

User JWT issuance uses **`POST /v1/auth/login`** and **`POST /v1/auth/refresh`** (no Bearer on those routes). Subsequent **`/v1/*`** calls send **`Authorization: Bearer <access_token>`**. MFA and lockout policies are enforced in **`internal/app/auth`** and **`internal/config`** (see **[`../runbooks/configuration.md`](../runbooks/configuration.md)**).

```mermaid
sequenceDiagram
  participant Admin as Admin Web
  participant API as cmd/api HTTP
  participant Auth as app/auth + Postgres

  Admin->>API: POST /v1/auth/login (organizationId, email, password)
  API->>Auth: validate credentials / MFA policy
  Auth-->>API: access + refresh tokens
  API-->>Admin: 200 JSON tokens
  Admin->>API: GET /v1/admin/... Bearer access token
  API->>Auth: JWT middleware + RBAC
  Auth-->>API: principal + permissions
  API-->>Admin: JSON response
```

## Machine runtime: gRPC (primary) vs legacy HTTP (deprecated)

**Primary:** **`avf.machine.v1`** with **Machine JWT** metadata (`authorization: Bearer …`). See **[`../api/machine-grpc.md`](../api/machine-grpc.md)**.

**Legacy:** Deprecated vending HTTP routes under `/v1/setup`, `/v1/commerce` (machine), `/v1/device`, etc., register only when **`ENABLE_LEGACY_MACHINE_HTTP=true`** (see `internal/httpserver/server.go`). Production should keep **`ENABLE_LEGACY_MACHINE_HTTP=false`** unless explicitly migrating. OpenAPI may still list paths for documentation; treat them as non-registered when the flag is off.

```mermaid
sequenceDiagram
  participant App as Vending App
  participant GRPC as cmd/api avf.machine.v1
  participant AppLayer as internal/app/*
  participant PG as PostgreSQL

  App->>GRPC: Unary RPC + Machine JWT metadata
  GRPC->>AppLayer: handler delegates (commerce, catalog, inventory, ...)
  AppLayer->>PG: transactional mutations / reads
  PG-->>AppLayer: rows
  AppLayer-->>GRPC: protobuf response + MachineResponseMeta
  GRPC-->>App: OK / gRPC status + error_code contract
```

## Backend → machine commands (MQTT + ledger)

Commands append **command ledger** rows and publish to MQTT; devices ACK via MQTT (and receipts ingested). HTTP **`POST …/commands/poll`** is a **degraded legacy bridge**, not the primary delivery path.

```mermaid
sequenceDiagram
  participant API as cmd/api
  participant MQTT as MQTT publisher
  participant Broker as MQTT broker
  participant Device as Machine
  participant Ingest as cmd/mqtt-ingest / ingest path
  participant PG as PostgreSQL

  API->>PG: append command_ledger + lease/outbox as applicable
  API->>MQTT: publish command envelope
  MQTT->>Broker: TLS publish
  Broker->>Device: subscribed topic
  Device->>Broker: ACK / receipt / telemetry
  Broker->>Ingest: ingress topics
  Ingest->>PG: receipts, telemetry projections
```

## Payment webhook → order timeline

Provider callbacks use **HTTPS** + **HMAC** (no User JWT). Idempotency keys dedupe PSP retries; order timelines aggregate commerce events.

```mermaid
sequenceDiagram
  participant PSP as Payment provider
  participant API as POST .../webhooks
  participant Commerce as app/commerce
  participant PG as PostgreSQL

  PSP->>API: HTTPS JSON + HMAC headers
  API->>Commerce: verify signature + replay window
  Commerce->>PG: idempotent persist payment/order rows + timeline evidence
  Commerce-->>API: 200 + replay semantics as applicable
  API-->>PSP: acknowledgement
```

## Async outbox (NATS JetStream)

Transactional **`outbox_events`** rows are published by **`cmd/worker`** with retries, backoff, Postgres DLQ, and optional JetStream DLQ subject—see **[`../runbooks/outbox.md`](../runbooks/outbox.md)** and **[`../runbooks/outbox-dlq-debug.md`](../runbooks/outbox-dlq-debug.md)**.

## Related

- [`transport-boundary.md`](transport-boundary.md) — ownership matrix.
- [`current-architecture.md`](current-architecture.md) — process map and drift freeze.
- [`deployment-topology.md`](deployment-topology.md) — roles of nodes/services.
