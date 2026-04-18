# Documentation

## Architecture

- [Target architecture](architecture/target-architecture.md) — north-star vs **what runs in `cmd/*` today** (including optional NATS publish, MQTT ingest, reconciler list vs optional close-the-loop actions).
- [Migration / strangler strategy](architecture/migration-strategy.md) — coexistence with any prior system, parity gates, rollback posture, **annotated snapshot of this repo**.

Runtime-oriented package docs (see each `doc.go`): `internal/platform/mqtt`, `nats`, `objectstore`, `temporal` (SDK scaffold; no `cmd/*` worker), `clickhouse` (placeholder + noop DI)—each states live vs partial vs future path.

## Operations

- [Incident runbook](../ops/RUNBOOK.md) — operator sessions, outbox, NATS, reconciler, MQTT ingest; log fields and SQL.
- [Metrics / signals](../ops/METRICS.md) — what Prometheus scrapes today vs log-based alerting.

## Local operations

- Docker stack: [deployments/docker/README.md](../deployments/docker/README.md)
- Environment template: [../.env.example](../.env.example)
- Observability sample configs: [otel](../ops/otel/otel-collector.yaml), [Prometheus](../ops/prometheus/prometheus.yml), [Loki](../ops/loki/config.yml), [Grafana provisioning](../ops/grafana/provisioning/) (paths relative to repository root)

## Repository root

See [../README.md](../README.md) for build commands, migration notes, layout, integration test requirements (`TEST_DATABASE_URL`), and CI / local gate targets (`make ci-gates`, `make ci`).
