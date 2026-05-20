# Documentation index

Central map for AVF Vending API documentation. Repository root [`README.md`](../README.md) covers build, layout, and CI gates.

## Architecture

System design, transport boundaries, and phased roadmaps.

- [Current architecture (as built)](architecture/current-architecture.md)
- [Production final contract](architecture/production-final-contract.md) — normative pilot→fleet boundaries
- [Enterprise target model](architecture/enterprise-target-model.md)
- [Transport boundary](architecture/transport-boundary.md)
- [Data flow overview](architecture/data-flow.md)
- [Deployment topology](architecture/deployment-topology.md)
- [Migration / strangler strategy](architecture/migration-strategy.md)
- [P0 / P1 / P2 implementation roadmap](architecture/p0-p1-p2-implementation-roadmap.md)
- [Target architecture](architecture/target-architecture.md)

## Production deployment

Operator checklists, smoke tests, metrics, and production migration safety.

- [Production release checklist](production/production-release-checklist.md)
- [Production smoke tests](production/production-smoke-tests.md)
- [Production data migration safety](production/production-data-migration-safety.md)
- [Production backup / restore drill](production/production-backup-restore-drill.md)
- [Production OpenAPI and metrics](production/production-openapi-and-metrics.md)
- [Production metrics reference](production/production-metrics.md)
- [Field pilot checklist](production/field-pilot-checklist.md)
- [Field rollout checklist](production/field-rollout-checklist.md)
- [Product media offline cache — production migration](runbooks/product-media-offline-cache-production-migration.md)

**2-VPS runbooks:** [production-2-vps](runbooks/production-2-vps.md) · [cutover/rollback](runbooks/production-cutover-rollback.md) · [backup/restore/DR](runbooks/production-backup-restore-dr.md) · [day-2 incidents](runbooks/production-day-2-incidents.md)

## Database / migrations

- **Goose migrations (production source of truth):** [`../migrations/`](../migrations/)
- **sqlc schema mirror:** [`../db/schema/`](../db/schema/)
- [Migration safety runbook](runbooks/migration-safety.md)
- [Backup evidence for production migrations](runbooks/backup-evidence-for-production-migrations.md)
- [Production data migration safety](production/production-data-migration-safety.md)

## Deployment & environments

Release process, staging gates, secrets contract, and environment matrix.

- [Environments](deployment/environments.md)
- [Release process (enterprise)](deployment/release-process.md)
- [Staging / pre-prod gate](deployment/staging-preprod-gate.md)
- [Two-VPS rolling production deploy](deployment/two-vps-rolling-production-deploy.md)
- [Deployment secrets](deployment/deployment-secrets.md)
- [GitHub repository governance (Settings UI)](deployment/github-governance.md)
- [Deploy monitoring SLO](deployment/deploy-monitoring-slo.md)
- [Artifact retention](deployment/artifact-retention.md)
- [Release evidence retention](deployment/release-evidence-retention.md)

## CI/CD

- [CI/CD release runbook](runbooks/cicd-release.md)
- [CI/CD enterprise contract](cicd/ci-cd-enterprise-contract.md)
- [Staging → production gate](cicd/staging-production-gate.md)
- [CI/CD final audit](cicd/CI_CD_FINAL_AUDIT.md)
- [Deployment staging-to-prod gate (runbook)](runbooks/deployment-staging-to-prod-gate.md)

## Testing

Local, E2E, load, and production verification guides.

- [Local testing guide](testing/local-testing-guide.md)
- [Integrated REST / gRPC / MQTT production verification](testing/integrated-rest-grpc-mqtt-production-verification.md)
- [gRPC local testing (grpcurl)](testing/grpc-local-test.md)
- [E2E local test guide](testing/e2e-local-test-guide.md)
- [Load test harness](testing/load-test.md)
- [Field test cases](testing/field-test-cases.md)
- [Production canary test guide](testing/production-canary-test-guide.md)
- [Production test execution order](testing/05_PRODUCTION_TEST_EXECUTION_ORDER.md)

## Postman / OpenAPI

- **OpenAPI 3.0 (generated):** [`swagger/swagger.json`](swagger/swagger.json) — regenerate with `make swagger`
- **Postman collections (CI-checked):** [`../postman/`](../postman/) — collections, environments, production suite
- [Postman runbook](runbooks/postman.md)
- [Swagger / OpenAPI appendix](api/swagger-openapi-appendix.md)
- [API contract checks](api/api-contract-checks.md)

## HTTP API (integration)

**Hub docs:** [Admin REST](api/admin-rest.md) · [Machine gRPC](api/machine-grpc.md)

- [API client classification](api/api-client-classification.md)
- [Kiosk app flow](api/kiosk-app-flow.md)
- [MQTT device contract](api/mqtt-contract.md)
- [Internal gRPC queries](api/internal-grpc.md)
- [Machine setup](api/setup-machine.md)
- [Payment webhooks](api/payment-webhook-security.md)

## Troubleshooting / runbooks

Incident and day-2 procedures: **[`runbooks/README.md`](runbooks/README.md)**

| Area | Examples |
|------|----------|
| Outages | [Postgres](runbooks/postgres-outage.md), [Redis](runbooks/redis-outage-behavior.md), [MQTT broker](runbooks/mqtt-broker-outage.md) |
| Payments | [Reconciliation](runbooks/payment-reconciliation.md), [Webhook debug](runbooks/payment-webhook-debug.md) |
| Worker / outbox | [Outbox](runbooks/outbox.md), [DLQ debug](runbooks/outbox-dlq-debug.md) |
| Deploy failures | [Deploy failure](runbooks/deploy-failure.md), [Rollback](runbooks/rollback-production.md) |

## Audits & reports

- **Audits (readiness / gap / cleanup):** [`audits/README.md`](audits/README.md)
- **Generated phase reports:** [`reports/README.md`](reports/README.md)
  - [Production deploy reports](reports/production-deploy/) — failure analysis, recovery, migration evidence
  - [Verification reports](reports/verification/) — REST/gRPC/MQTT/E2E/Postman/full-system verification output
  - [Test coverage reports](reports/test/) — e.g. MQTT full coverage
- **Repository cleanup audits:** [`audits/REPO_CLEANUP_AUDIT.md`](audits/REPO_CLEANUP_AUDIT.md)

## Local dependencies

- Docker stack: [`../deployments/docker/README.md`](../deployments/docker/README.md)
- Environment template: [`../.env.example`](../.env.example)
- Observability sample configs: [`../deployments/docker/observability/`](../deployments/docker/observability/) (Prometheus, Loki, Grafana, OTel)

## Other

- [Supply chain pinning](security/supply-chain-pinning.md)
- [Vietnamese API guide](vi/huong-dan-api-tu-dang-nhap-den-ban-hang.md)
- [Repository structure cleanup audit](audits/REPO_STRUCTURE_CLEANUP_AUDIT.md)
- [Repository cleanup audit (2026-05-20)](audits/REPO_CLEANUP_AUDIT.md)
