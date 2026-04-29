# Production readiness (single-cluster / HA-friendly API)

**Normative integration contract (kiosk vs admin vs MQTT):** **[`../architecture/production-final-contract.md`](../architecture/production-final-contract.md)**.

**P2 documentation bridge:** For **100–1000 machine** go-live **field** proof, use **[`testing/field-test-cases.md`](../testing/field-test-cases.md)** + **[`operations/field-pilot-checklist.md`](../operations/field-pilot-checklist.md)** / **[`operations/field-rollout-checklist.md`](../operations/field-rollout-checklist.md)** together with **[`production-release-readiness.md`](production-release-readiness.md)** (storm + monitoring tiers). Config rules below remain **architecture** prerequisites (non-contradiction with managed PostgreSQL **+** **[`production-2-vps.md`](production-2-vps.md)** app nodes).

This runbook summarizes **configuration gates** and **observability** expectations for deploying the AVF API and workers to production (roughly 100–1000 machines, no mandatory microservice split).

## Config validation (fail-fast)

Load order: `config.Load()` in `cmd/api`, workers, and `go run ./cmd/cli -validate-config`. Staging and production enforce stricter rules than development.

| Area | Rule (summary) |
|------|----------------|
| **JWT (Admin/User REST)** | Production: `USER_JWT_*` aliases are preferred; legacy `HTTP_AUTH_*` remains supported. HS256 requires `USER_JWT_SECRET` / `HTTP_AUTH_JWT_SECRET` ≥ 32 bytes; asymmetric modes require `USER_JWT_LOGIN_SECRET` / `HTTP_AUTH_LOGIN_JWT_SECRET` ≥ 32 bytes (HS256 session tokens). Supported modes: `rs256_pem`, `rs256_jwks`, `ed25519_pem`, `jwt_jwks` (RS256+Ed25519 via JWKS with `kid`). Optional `USER_JWT_ALG` / `HTTP_AUTH_JWT_ALG` labels `HS256` / `RS256` / `EdDSA` for config consistency checks. |
| **JWT (Machine gRPC)** | `MACHINE_JWT_*` separates machine runtime token validation from Admin/User JWT. Default local mode remains HS256 with backward-compatible `HTTP_AUTH_*` secret fallback. Enterprise modes support `MACHINE_JWT_MODE=rs256_pem`, `rs256_jwks`, `ed25519_pem`, or `jwt_jwks`, with `MACHINE_JWT_ALG`, `MACHINE_JWT_AUDIENCE=avf-machine-grpc`, issuer, JWKS cache TTL, and `kid`-based key rotation. |
| **JWT (Internal Service)** | `SERVICE_JWT_SECRET` is the preferred alias for split-ready internal gRPC bearer tokens (`typ=service`, `aud=avf-internal-grpc`); `INTERNAL_GRPC_SERVICE_TOKEN_SECRET` remains supported. |
| **Machine gRPC mTLS** | Optional: `GRPC_TLS_ENABLED`, `GRPC_TLS_CERT_FILE`, `GRPC_TLS_KEY_FILE`, `GRPC_TLS_CLIENT_CA_FILE`, `GRPC_TLS_CLIENT_AUTH` (`no` / `request` / `require`). Device identity in client certs: URI SAN prefix `GRPC_MTLS_MACHINE_ID_URI_PREFIX` (default `urn:avf:machine:`). Registered fingerprints in `machine_device_certificates`; lifecycle hooks register, revoke, rotate, and audit metadata. **Cert-only** RPCs require explicit `GRPC_MACHINE_AUTH_CERT_ONLY_ALLOWED=true` plus client CA + request/require (see config validation). |
| **Data retention** | Telemetry cleanup uses `TELEMETRY_*` settings. Enterprise operational cleanup is worker-only and disabled by default via `ENTERPRISE_RETENTION_CLEANUP_ENABLED=false`; see [`data-retention.md`](data-retention.md). |
| **CORS** | Staging/production: `HTTP_CORS_ALLOWED_ORIGINS` must be **set explicitly** (comma-separated list, or empty to disable browser CORS middleware). Wildcard `*` is rejected. |
| **Commerce webhooks** | Staging/production: HMAC secret (`COMMERCE_PAYMENT_WEBHOOK_SECRET` or legacy aliases / `COMMERCE_PAYMENT_WEBHOOK_SECRETS_JSON`) required unless using an explicitly documented unsafe path (unsafe flags are **forbidden** in staging/production). `COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED` is only valid in development/test. |
| **gRPC** | Production: `GRPC_REFLECTION_ENABLED` must be false. When `GRPC_ENABLED=true`, `GRPC_HEALTH_USE_PROCESS_READINESS` must be true so `grpc.health.v1` reflects the same readiness as HTTP. |
| **MQTT** | Staging/production: TLS required (`ssl://`/`tls://` URL or `MQTT_TLS_ENABLED=true`). Plain `tcp://` without TLS fails validation. `MQTT_INSECURE_SKIP_VERIFY` is not allowed in staging/production. |
| **Internal gRPC** | Production: internal reflection must be off; service token rules apply when enabled. |
| **Metrics on public HTTP** | Production: if `METRICS_EXPOSE_ON_PUBLIC_HTTP=true`, `METRICS_SCRAPE_TOKEN` (min 16 chars) is required. |

Use your orchestrator’s **preflight** step to run `-validate-config` with the same env as the running pod.

## Health endpoints

- **`/health/live`**: lightweight; use for kube `livenessProbe`.
- **`/health/ready`**: dependency checks when strict readiness is configured; use for `readinessProbe`. **Do not** expose internal error strings to clients; logs on the server carry detail.

Machine and internal **gRPC** expose `grpc.health.v1` when enabled; health status should align with process readiness when `GRPC_HEALTH_USE_PROCESS_READINESS=true`.

## Metrics to watch

Prometheus series (non-exhaustive; scrape API ops port or worker ports as deployed):

- **HTTP:** `avf_http_requests_total` (method, route pattern, status class).
- **gRPC:** `avf_grpc_requests_handled_total`, `avf_grpc_request_duration_seconds`.
- **DB pool (API):** `avf_db_pool_*` when registered.
- **Commerce webhook:** `avf_commerce_payment_webhook_requests_total` (`result` label; subsystem `commerce`).
- **Outbox (worker):** `avf_worker_outbox_*` (pending, lag, DLQ publish failures) — see [outbox-dlq-debug.md](./outbox-dlq-debug.md).
- **MQTT command publish (API):** `avf_mqtt_publish_duration_seconds` (`result`).
- **Device heartbeat (ingest):** `avf_device_heartbeat_ingest_total` (`result`).

## Logging

Structured JSON logs should include **`request_id`** and **`trace_id`** (when a span is active) on HTTP and gRPC paths. Do not log raw `Authorization` headers, webhook secrets, or JWT material.

## Related runbooks

- [payment-webhook-debug.md](./payment-webhook-debug.md)
- [mqtt-command-debug.md](./mqtt-command-debug.md)
- [outbox-dlq-debug.md](./outbox-dlq-debug.md)
- [local-dev.md](./local-dev.md)
- [production-metrics-scraping.md](./production-metrics-scraping.md)
