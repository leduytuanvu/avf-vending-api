# Operations assets

Sample configs for local or lab stacks live under subfolders (`prometheus`, `loki`, `otel`, `grafana`).

For the hardened 2-VPS production topology, use the deployable assets under `deployments/prod/observability/` together with the app-node overlay `deployments/prod/docker-compose.observability.yml`.

**Primary support doc:** [RUNBOOK.md](RUNBOOK.md) — incidents, log fields, SQL checks, and alert ideas.

**Production day-2 docs:** `docs/runbooks/production-cutover-rollback.md`, `docs/runbooks/production-backup-restore-dr.md`, and `docs/runbooks/production-day-2-incidents.md`.

**Metrics reality:** [METRICS.md](METRICS.md) — what is scraped today vs what is log-derived.

**Process / capability map:** [PROCESSES.md](PROCESSES.md) — which binary is required for which feature.

**Local Docker:** [../deployments/docker/README.md](../deployments/docker/README.md) — compose profiles (`broker`, `observability`, `experimental`).
