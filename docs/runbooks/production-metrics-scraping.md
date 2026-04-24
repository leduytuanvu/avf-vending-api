# Production API metrics scraping

## Default (recommended)

In production, the API serves **`/metrics` on the operations listener** (`HTTP_OPS_ADDR`, typically **`:8081`** on a private or loopback bind), not on the public API port (`HTTP_ADDR`, often **`:8080`** behind the edge). This keeps process and HTTP metrics off the internet while still allowing Prometheus to scrape from the private network.

- **Enable metrics:** `METRICS_ENABLED=true`
- **Ops bind:** set `HTTP_OPS_ADDR` (required when metrics are on and public exposure is off; see server startup validation).
- **Public port:** `METRICS_EXPOSE_ON_PUBLIC_HTTP` defaults to **false** in production when unset, so **`/metrics` is not registered on the main router**.

Prometheus job **`avf_api_metrics`** in `deployments/prod/observability/prometheus/prometheus.yml` scrapes each app node at **`http://<private-host>:8081/metrics`**. Replace hostnames with your inventory (the file uses `*.internal.example.com` placeholders).

## Readiness script

From a host that can reach private ops ports, set **`API_METRICS_URL`** to the ops scrape URL, for example:

```bash
export API_METRICS_URL=http://127.0.0.1:8081/metrics
```

Then run `deployments/prod/scripts/check_monitoring_readiness.sh` (see the script header for other required variables).

## Optional: metrics on the public listener

If you must expose **`/metrics` on `HTTP_ADDR`** (not recommended), set **`METRICS_EXPOSE_ON_PUBLIC_HTTP=true`**. In production this requires **`METRICS_SCRAPE_TOKEN`** (minimum length 16). Clients must send **`Authorization: Bearer <token>`**.

Configure Prometheus with a scrape authorization block (for example `authorization` / `credentials_file` pointing at the bearer token) so scrapes succeed. Prefer the ops listener and network policy instead.

## Optional: bearer token on the ops listener

If **`METRICS_SCRAPE_TOKEN`** is set (minimum **16** characters when used with **`METRICS_EXPOSE_ON_PUBLIC_HTTP=true`** in production), the API protects **`GET /metrics` on both listeners** where metrics are registered:

- **Public** (`HTTP_ADDR`): unchanged — `Authorization: Bearer <token>` required when the route is mounted.
- **Private ops** (`HTTP_OPS_ADDR`): the same token is required for scrapes. Health endpoints **`/health/live`** and **`/health/ready`** on the ops mux stay **unauthenticated** so private probes and load balancers are unaffected.

## Labels and secrets

Do not put API keys, tokens, or other secrets into Prometheus **metric names or label values**. Use low-cardinality, non-sensitive labels only (for example HTTP method, route pattern, status class).
