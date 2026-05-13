# Production smoke checklist — OpenAPI JSON and metrics

Quick operator checklist for verifying **public** vs **ops** HTTP behavior after deploy.

## Case 1 — Public metrics should be hidden

```bash
curl -i https://api.example.com/metrics
```

- **Expected:** HTTP **404** (or route not registered) when `METRICS_EXPOSE_ON_PUBLIC_HTTP` is not enabled in production.

## Case 2 — Ops metrics should work from server / private network

```bash
curl -i http://127.0.0.1:8081/metrics
```

- **Expected:** HTTP **200** when `METRICS_ENABLED=true` and `HTTP_OPS_ADDR` listens on that port.
- If `METRICS_SCRAPE_TOKEN` is set, add `Authorization: Bearer <token>`.

## Case 3 — OpenAPI JSON disabled

```bash
curl -i https://api.example.com/swagger/doc.json
```

- **Expected:** HTTP **404** when `HTTP_OPENAPI_JSON_ENABLED` is false/unset or production allow flag is false.

## Case 4 — OpenAPI JSON enabled for Postman

**Required env:**

- `HTTP_SWAGGER_UI_ENABLED=false`
- `HTTP_OPENAPI_JSON_ENABLED=true`
- `PRODUCTION_OPENAPI_JSON_ALLOWED=true`

```bash
curl -i https://api.example.com/swagger/doc.json
```

- **Expected:** HTTP **200**, `Content-Type` JSON, body contains OpenAPI metadata (e.g. `openapi` and `paths`).

## Case 5 — Wrong env / no restart

- **Symptom:** Internal **404** and public **404** for `doc.json`.
- **Check:** Env inside container, container restart after env change, deployed image digest/tag.

## Case 6 — Proxy issue

- **Symptom:** Internal `curl` to `127.0.0.1:8080` returns **200** for `doc.json`, public URL **404**.
- **Check:** Caddy / load balancer routing, upstream port, path stripping, and TLS site config.
