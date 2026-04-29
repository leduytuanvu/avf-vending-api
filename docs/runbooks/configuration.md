# Configuration validation (`APP_ENV` and secrets)

All binaries load configuration through [`internal/config`](../../internal/config). Invalid combinations fail fast at startup — **production (`APP_ENV=production`) is fail-closed** where documented below.

## Validate locally without starting the API

```bash
go run ./cmd/cli -validate-config
```

Exit code **2** means validation failed; inspect stderr.

---

## Production guardrails (overview)

When **`APP_ENV=production`**, the loader rejects unsafe combinations:

| Area | Rule |
|------|------|
| Readiness | `READINESS_STRICT=true` |
| Commerce webhooks | `COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=false` (and unsigned unsafe hatch forbidden in staging/production) |
| Swagger UI | `HTTP_SWAGGER_UI_ENABLED=true` only with `PRODUCTION_SWAGGER_UI_ALLOWED=true` |
| OpenAPI JSON (`GET /swagger/doc.json`) | Unset defaults **off**. Explicit `HTTP_OPENAPI_JSON_ENABLED=true` requires `PRODUCTION_OPENAPI_JSON_ALLOWED=true` |
| MQTT | Non-loopback brokers must use TLS (`ssl://` / `tls://` / `mqtts://` / `wss://`, or `MQTT_TLS_ENABLED=true` with `tcp://`). Non-loopback requires MQTT username/password or mutual TLS unless `PRODUCTION_ALLOW_ANONYMOUS_MQTT=true`. `MQTT_INSECURE_SKIP_VERIFY=false` |
| gRPC (machine) | `GRPC_TLS_ENABLED=true` or `GRPC_BEHIND_TLS_PROXY=true`; `GRPC_PUBLIC_BASE_URL` must be **`grpcs://`**; reflection off by default; machine JWT + idempotency required (`validateProduction` in `deployment_env.go`) |
| Public Prometheus | `METRICS_EXPOSE_ON_PUBLIC_HTTP=true` requires `METRICS_SCRAPE_TOKEN` (≥16 chars) **and** `PRODUCTION_PUBLIC_METRICS_ENDPOINT_ALLOWED=true` |
| Interactive JWT | Explicit **`HTTP_AUTH_MODE` or `USER_JWT_MODE`**; explicit **`MACHINE_JWT_MODE`**; HS256 secrets ≥32 bytes, not documentation placeholders, not a single repeated byte; access TTL 5m–24h and refresh TTL 1h–180d with access < refresh |
| Machine JWT | `MACHINE_AUTH_REQUIRE_AUDIENCE=true`; HS256 requires issuer (`AUTH_ISSUER` or `MACHINE_JWT_ISSUER`) |
| Wiring flags | `API_REQUIRE_OUTBOX_PUBLISHER=true`, `API_REQUIRE_NATS_RUNTIME=true`. With **`MQTT_CLIENT_ID_API`** set, **`API_REQUIRE_MQTT_PUBLISHER=true`** |
| Redis-backed features | `CACHE_ENABLED` / rate limits / session features require `REDIS_ADDR` or `REDIS_URL` (see `RedisRuntimeFeatures.validate`) |
| Object storage / media | With **`API_ARTIFACTS_ENABLED`** / **`OBJECT_STORAGE_ENABLED`**, `OBJECT_STORAGE_BUCKET` and `OBJECT_STORAGE_PUBLIC_BASE_URL` are required |
| Telemetry / NATS | Staging and production require `NATS_URL` ([`validateProductionTelemetryNATS`](../../internal/config/mqtt_device_telemetry.go)); outbox publisher required in production (`deployment_env.go`) |

**Staging (`APP_ENV=staging`)** uses similar staging/production checks where marked (MQTT TLS, JWT placeholders, Redis usage); staging-specific URL/topic rules apply (`PUBLIC_BASE_URL`, `MQTT_TOPIC_PREFIX`, sandbox payments).

**Development (`APP_ENV=development`)** keeps permissive defaults so local stacks stay simple.

---

## MQTT ingest-only notes

Processes such as **`cmd/mqtt-ingest`** should use **`MQTT_CLIENT_ID_INGEST`** (not **`MQTT_CLIENT_ID_API`**) unless the binary publishes commands; **`MQTT_CLIENT_ID_API`** triggers stricter production rules tied to **`API_REQUIRE_MQTT_PUBLISHER`**.

---

## Related docs

- [Local development](./local-dev.md)
- Security appendix on webhook signatures (`docs/api/payment-webhook-security.md`)
