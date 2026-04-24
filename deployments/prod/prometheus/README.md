# Prometheus (production)

Canonical Prometheus configuration and rule files:

- `deployments/prod/observability/prometheus/prometheus.yml`
- `deployments/prod/observability/prometheus/alerts.yml`

Validate rules before deploy:

```bash
promtool check rules deployments/prod/observability/prometheus/alerts.yml
```

Alert runbooks: `docs/runbooks/production-observability-alerts.md`

This directory is a **pointer** for workflows that expect `deployments/prod/prometheus/*`.
