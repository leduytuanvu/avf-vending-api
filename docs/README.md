# Documentation index

Central map for AVF Vending API documentation. Repository root [`README.md`](../README.md) covers build, layout, and CI gates.

## Canonical vs non-canonical

| Use this (canonical) | Not for day-to-day ops |
|----------------------|-------------------------|
| [`runbooks/README.md`](runbooks/README.md) | [`archive/`](archive/) — historical audits, FINAL reports, old gate evidence |
| [`audits/final-enterprise-audit.md`](audits/final-enterprise-audit.md) | [`archive/audits/`](archive/audits/) — superseded readiness snapshots |
| [`architecture/current-architecture.md`](architecture/current-architecture.md) | [`archive/cleanup/`](archive/cleanup/) — prior cleanup inventories |
| [`reports/README.md`](reports/README.md) — active phase reports | [`archive/verification/`](archive/verification/) — archived FINAL signoffs |
| [`testing/README.md`](testing/README.md) | Point-in-time reports under `archive/reports/` |

**2026-06 repo cleanup:** deletions and deferred decisions are recorded in [`reports/cleanup/2026-06-repo-cleanup-manifest.md`](reports/cleanup/2026-06-repo-cleanup-manifest.md).

**2026-07 repo cleanup:** structural pass summary [`reports/cleanup/2026-07-02-repo-cleanup-final-report.md`](reports/cleanup/2026-07-02-repo-cleanup-final-report.md); deep clean under [`reports/cleanup/`](reports/cleanup/) (`2026-07-02-deep-clean-*`); archived working docs in [`archive/cleanup/2026-07-02/`](archive/cleanup/2026-07-02/).

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

- [Production docs index](production/README.md)
- [Deployment runbook (index)](production/DEPLOYMENT_RUNBOOK.md)
- [Production troubleshooting](production/TROUBLESHOOTING.md)
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

Local, E2E, load, and production verification guides — see [testing/README.md](testing/README.md).

## Postman / OpenAPI

- **OpenAPI 3.0 (generated):** [`swagger/swagger.json`](swagger/swagger.json) — regenerate with `make swagger`
- **Postman collections (CI-checked):** [`../postman/`](../postman/) — collections, environments, production suite
- [Postman runbook](runbooks/postman.md)
- [Repomix generation guide](operations/repomix-generation-guide.md) — smaller LLM packs; excludes heavy Postman JSON
- [Machine gRPC production contract](api/machine-grpc-production-contract.md) — **Android runtime SoT** (RPC auth, idempotency, legacy fallbacks)
- [Android proto sync index](api/android-proto-sync.md) — generated RPC list from `proto/avf/machine/v1/`
- [API contract checks](api/api-contract-checks.md)

## Audits and archives

- [Active audits](audits/README.md) — enterprise readiness and gap analysis (**canonical** for current readiness)
- [Documentation archive](archive/README.md) — historical audits, verification FINALs, git-deploy reports (**non-canonical**)
- [2026-06 cleanup manifest](reports/cleanup/2026-06-repo-cleanup-manifest.md) — Phase 1 deletions and deferred moves
- [2026-07 structural cleanup final report](reports/cleanup/2026-07-02-repo-cleanup-final-report.md)
- [2026-07 deep clean reports](reports/cleanup/2026-07-02-deep-clean-baseline.md)

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
  - [Verification index](reports/verification/README.md) — archived bodies under [archive/reports/verification/](archive/reports/verification/)
  - [Archived FINAL / git-deploy evidence](archive/README.md) — historical signoff and deploy reports
  - Test coverage reports under `reports/test/` — e.g. MQTT full coverage (generated artifacts)

## Local dependencies

- Docker stack: [`../deployments/docker/README.md`](../deployments/docker/README.md)
- Environment template: [`../.env.example`](../.env.example)
- Observability sample configs: [`../deployments/docker/observability/`](../deployments/docker/observability/) (Prometheus, Loki, Grafana, OTel)

## Other

- [Supply chain pinning](security/supply-chain-pinning.md)
- [Vietnamese API guide](vi/huong-dan-api-tu-dang-nhap-den-ban-hang.md)
- [P2 structure refactor proposal](audits/REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md)
