# Production OpenAPI JSON, Swagger UI, and metrics

This note describes how the API handles **`GET /swagger/doc.json`**, Swagger UI, and **`GET /metrics`** in production, and how to debug **404** responses safely.

## Why public `/metrics` often returns 404

In **`APP_ENV=production`**, the main HTTP listener (`HTTP_ADDR`, typically `:8080` behind Caddy) **does not register `/metrics` by default**. That avoids exposing the Prometheus text format on the same origin as customer-facing APIs.

Operators should scrape metrics from the **ops / private listener** set with **`HTTP_OPS_ADDR`** (for example `0.0.0.0:8081` inside the container, or `127.0.0.1:8081` when bound to loopback only). That listener serves:

- `GET /metrics`
- `GET /health/live`
- `GET /health/ready`

Public `/metrics` is only mounted when **`METRICS_EXPOSE_ON_PUBLIC_HTTP=true`**, which in production additionally requires **`METRICS_SCRAPE_TOKEN`** (minimum length enforced by config) and **`PRODUCTION_PUBLIC_METRICS_ENDPOINT_ALLOWED=true`**. This path is **discouraged**; prefer the ops listener.

### Protecting `/metrics` with `METRICS_SCRAPE_TOKEN`

If **`METRICS_SCRAPE_TOKEN`** is set, clients must send:

```http
Authorization: Bearer <token>
```

The same gate applies when the token is configured—on whichever listener exposes `/metrics` (public or ops). Do not commit real tokens; generate a long random value in your secret store.

## Why public `/swagger/doc.json` returns 404

The OpenAPI document is served only when **`HTTP_OPENAPI_JSON_ENABLED=true`**.

In production, if you set **`HTTP_OPENAPI_JSON_ENABLED=true`**, config also requires **`PRODUCTION_OPENAPI_JSON_ALLOWED=true`**. Otherwise the process **fails fast at startup** with a clear configuration error.

If either flag is missing or false, **`GET /swagger/doc.json`** is **not mounted** and returns **404**. That is expected and fail-closed.

### Postman import without Swagger UI

Use **JSON only**:

- `HTTP_SWAGGER_UI_ENABLED=false`
- `HTTP_OPENAPI_JSON_ENABLED=true`
- `PRODUCTION_OPENAPI_JSON_ALLOWED=true`

Swagger UI in production requires **`HTTP_SWAGGER_UI_ENABLED=true`** and **`PRODUCTION_SWAGGER_UI_ALLOWED=true`**; defaults keep it off.

## Internal container route vs public URL

- **Inside the container**, curl the process on `127.0.0.1:8080` for public routes and `127.0.0.1:8081` (or your `HTTP_OPS_ADDR`) for ops routes.
- **Through Caddy / the load balancer**, only routes published as public may differ (for example if the edge returns 404 for paths the app would serve internally).

## Troubleshooting commands (placeholders)

Inspect effective env inside the API container (no secrets in this grep):

```bash
docker exec <api-container> sh -lc 'env | sort | grep -E "^(APP_ENV|HTTP_OPENAPI_JSON_ENABLED|PRODUCTION_OPENAPI_JSON_ALLOWED|HTTP_SWAGGER_UI_ENABLED|METRICS_ENABLED|METRICS_EXPOSE_ON_PUBLIC_HTTP|HTTP_OPS_ADDR)="'
```

Internal public listener (from inside the same network namespace as the app):

```bash
docker exec <api-container> sh -lc 'curl -i http://127.0.0.1:8080/swagger/doc.json | head -40'
```

Public (via TLS hostname):

```bash
curl -i https://<production-domain>/swagger/doc.json
```

Ops metrics (replace port if `HTTP_OPS_ADDR` differs):

```bash
curl -i http://127.0.0.1:8081/metrics
```

With scrape token (use your secret manager; do not paste real tokens into shells that log history):

```bash
curl -i -H "Authorization: Bearer $METRICS_SCRAPE_TOKEN" http://127.0.0.1:8081/metrics
```

### Symptom → cause

| Symptom | Likely cause |
|--------|----------------|
| Internal **404** and public **404** | OpenAPI JSON not enabled, wrong env, container not restarted, or wrong image tag. |
| Internal **200** but public **404** | Reverse proxy / Caddy / LB path or upstream mismatch. |
| `GET /metrics` **404** on `:8080` in production | Expected when metrics are ops-only; use `HTTP_OPS_ADDR`. |
| Startup panic / error about `PRODUCTION_OPENAPI_JSON_ALLOWED` | OpenAPI JSON explicitly enabled without production allow flag. |

## Read-only smoke

`scripts/test/run-readonly-smoke.sh` honors **`APP_ENV`**, **`SMOKE_EXPECT_OPENAPI_JSON`**, **`SMOKE_EXPECT_PUBLIC_METRICS`**, **`OPS_BASE_URL`**, and **`METRICS_SCRAPE_TOKEN`**. It writes **`docs/reports/test/readonly-smoke.json`** and **`docs/reports/test/readonly-smoke.md`** with **redacted** body snippets (tokens are never printed).

See also: `docs/testing/production-smoke-openapi-metrics-checklist.md`.
