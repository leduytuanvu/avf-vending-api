# Documentation

## HTTP API (integration)

- [API client classification](api/api-client-classification.md) — which clients (kiosk, technician, admin, webhook, device fallback, DevOps) should call which route groups; idempotency and offline-retry notes.
- [Kiosk app flow](api/kiosk-app-flow.md) — end-to-end sequence: provisioning, setup, commerce, vend, offline/replay, cash (with roadmap gaps noted).
- [Kiosk app implementation checklist](api/kiosk-app-implementation-checklist.md) — Android-focused contract: Room/cache, startup order, sale/refund, offline outbox, settlement, security boundaries.
- [API surface audit](api/api-surface-audit.md) — endpoint inventory, auth/idempotency/offline, fallback vs keep, pilot vs 1000-machine readiness, risks.
- [Machine setup: bootstrap, topology, planogram](api/setup-machine.md) — technician/admin flows and links to commerce.
- [Machine runtime HTTP writes](api/machine-runtime.md) — check-ins and config-apply acknowledgements on the control-plane HTTP surface.
- [Stock adjustments](api/inventory-adjustments.md) — `POST …/stock-adjustments`, idempotency, reasons.
- [MQTT device contract + HTTP command/vend fallbacks](api/mqtt-contract.md) — topics, QoS, retain, envelope, `vend-results` / `commands/poll`.
- [Internal gRPC queries](api/internal-grpc.md) — internal-only machine, telemetry, and commerce protobuf services.
- [Machine activation implementation handoff](api/machine-activation-implementation-handoff.md) — **shipped**; design + migration notes (`activation_http.go`).
- [Runtime sale catalog handoff](api/runtime-sale-catalog-implementation-handoff.md) — **shipped**; `GET /v1/machines/{machineId}/sale-catalog` (`sale_catalog_http.go`).
- [Telemetry reconcile / application ACK handoff](api/telemetry-reconcile-implementation-handoff.md) — **shipped**; device reconcile + status (`telemetry_reconcile_http.go`).
- [Commerce refund / cancel / vend-failure handoff](api/commerce-refund-cancel-implementation-handoff.md) — **shipped**; cancel/refund HTTP (`commerce_http.go`).

OpenAPI 3.0 (generated from `internal/httpserver/swagger_operations.go`): **`docs/swagger/swagger.json`** — regenerate with `python tools/build_openapi.py` (see repo `Makefile` **`swagger`** / **`swagger-check`**). When Swagger is enabled in production, the same spec is served at **`https://api.ldtv.dev/swagger/doc.json`** with UI at **`https://api.ldtv.dev/swagger/index.html`** (public documentation only; **`/v1/*`** calls still require **`Authorization: Bearer <JWT>`** unless a route is explicitly public). Maintainer notes: [swagger-openapi-appendix.md](api/swagger-openapi-appendix.md), [openapi-enterprise-upgrade-handoff.md](runbooks/openapi-enterprise-upgrade-handoff.md) (optional Swagger UX backlog; P0 paths are enforced by `tools/openapi_verify_release.py`).

## Architecture

- [Current architecture (as built)](architecture/current-architecture.md) — process map, transport policy, data flow map, enterprise gap list, and corrected docs drift.
- [Target architecture](architecture/target-architecture.md) — north-star vs **what runs in `cmd/*` today** (including optional NATS publish, MQTT ingest, reconciler list vs optional close-the-loop actions).
- [Migration / strangler strategy](architecture/migration-strategy.md) — coexistence with any prior system, parity gates, rollback posture, **annotated snapshot of this repo**.

Runtime-oriented package docs (see each `doc.go`): `internal/platform/mqtt`, `nats`, `objectstore`, `temporal` (SDK client + worker helpers; `cmd/temporal-worker` executes registered workflows), `clickhouse` (optional worker mirror path today, broader analytics later)—each states live vs partial vs future path.

## Operations

- [Release process (enterprise)](operations/release-process.md) — `develop` / `main`, Build, Security Release, **manual** production deploy, required run and evidence ids.
- [Production release checklist (operator)](operations/production-release-checklist.md) — governance, CI/Build/Security, staging id, backup id, digests, rollback, smoke, approval.
- [Field rollout checklist (vending)](operations/field-rollout-checklist.md) — machines, payments, dispense, MQTT, offline/retry, evidence owner.
- [CI/CD enterprise contract](operations/ci-cd-enterprise-contract.md) — what is enterprise-ready, what stays manual, evidence, and limitations.
- [API surface security](runbooks/api-surface-security.md) — RBAC, tenant binding for machine URLs, metrics/Swagger exposure, device HTTP fallback vs MQTT.
- [Enterprise API / backend / CI-CD audit report](runbooks/enterprise-api-backend-audit-report.md) — strict readiness verdicts (pilot vs 1000 machines), gaps, risks, release commands.
- [Incident runbook](../ops/RUNBOOK.md) — operator sessions, outbox, NATS, reconciler, MQTT ingest; log fields and SQL.
- [Metrics / signals](../ops/METRICS.md) — what Prometheus scrapes today vs log-based alerting.
- [2-VPS production topology](runbooks/production-2-vps.md) — deployment shape, edge model, ports, CI/CD flow.
- [2-VPS cutover and rollback](runbooks/production-cutover-rollback.md) — go-live checklist, rollback, and legacy fallback path.
- [2-VPS backup, restore, and DR](runbooks/production-backup-restore-dr.md) — managed-service backup expectations, restore drill, and recovery posture.
- [2-VPS day-2 incidents](runbooks/production-day-2-incidents.md) — app node loss, broker loss, DB/Redis issues, disk pressure, cert rotation, and vending-specific incidents.

## Local operations

- Docker stack: [deployments/docker/README.md](../deployments/docker/README.md)
- Environment template: [../.env.example](../.env.example)
- Observability sample configs: [otel](../ops/otel/otel-collector.yaml), [Prometheus](../ops/prometheus/prometheus.yml), [Loki](../ops/loki/config.yml), [Grafana provisioning](../ops/grafana/provisioning/) (paths relative to repository root)

## Repository root

See [../README.md](../README.md) for build commands, migration notes, layout, integration test requirements (`TEST_DATABASE_URL`), and CI / local gate targets (`make ci-gates`, `make ci`).
