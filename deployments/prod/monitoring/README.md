# Production monitoring (AVF vending API)

The maintained **compose stack** for Prometheus, Grafana, Loki, and blackbox probes lives under:

`deployments/prod/observability/`

- Prometheus config and alert rules: `deployments/prod/observability/prometheus/`
- Grafana provisioning (dashboards, datasources): `deployments/prod/observability/grafana/`

On-call documentation for alert names and response lives in:

`docs/runbooks/production-observability-alerts.md`

**API Prometheus scrape:** by default, production keeps **`/metrics` off the public API port**; scrape **`HTTP_OPS_ADDR`** (see `avf_api_metrics` in `deployments/prod/observability/prometheus/prometheus.yml`, usually **`:8081`**). If **`METRICS_SCRAPE_TOKEN`** is set, configure scrape **`authorization`** (Bearer) for **`/metrics`** on that ops port. Details: `docs/runbooks/production-metrics-scraping.md`.

**Readiness check (live cluster):** from a host that can reach private ops ports (bastion, app node), run `deployments/prod/scripts/check_monitoring_readiness.sh` with `API_METRICS_URL`, `MQTT_INGEST_METRICS_URL`, and `WORKER_METRICS_URL` (see that script header). It writes `monitoring-readiness-result.json` and exits non-zero if required metrics or health checks fail.

This directory is a **stable entry point** for paths that reference `deployments/prod/monitoring/*` in internal docs or automation.
