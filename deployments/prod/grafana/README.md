# Grafana (production)

Dashboard JSON and provisioning YAML live under:

`deployments/prod/observability/grafana/provisioning/`

Prometheus scrape `job` labels (for example `avf_mqtt_ingest_metrics`) must match panel queries; see `deployments/prod/observability/prometheus/prometheus.yml`.

This directory is a **pointer** for workflows that expect `deployments/prod/grafana/*`.
