# 2-VPS production env matrix

This matrix is for the primary production split:

- `app-node`: repeatable stateless workload stack. Run the same compose file on both production VPSes.
- `data-node`: optional self-hosted fallback plane for broker-style state when you are not using managed endpoints yet.

## App node

Set these in `app-node/.env.app-node` on every app VPS:

- image refs: `APP_IMAGE_REF`, `GOOSE_IMAGE_REF`
- public edge: `API_DOMAIN`, `PUBLIC_BASE_URL`, `CADDY_ACME_EMAIL`, `UPSTREAM_API`, `CADDY_MAX_REQUEST_BODY`
- managed/shared state: `DATABASE_URL`, `NATS_URL`, `MQTT_BROKER_URL`
- optional managed cache: `REDIS_ADDR`, `REDIS_PASSWORD`
- auth/app runtime: `APP_ENV`, `HTTP_AUTH_*`, `READINESS_STRICT`, `LOG_*`
- per-node MQTT identity: `MQTT_CLIENT_ID_API`, `MQTT_CLIENT_ID_INGEST`
- worker health endpoints: `WORKER_METRICS_LISTEN`, `RECONCILER_METRICS_LISTEN`, `MQTT_INGEST_METRICS_LISTEN`
- optional Temporal wiring: `TEMPORAL_ENABLED`, `TEMPORAL_HOST_PORT`, `TEMPORAL_NAMESPACE`, `TEMPORAL_TASK_QUEUE`, `TEMPORAL_WORKER_METRICS_LISTEN`
- optional object storage: `API_ARTIFACTS_ENABLED`, `S3_BUCKET`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` or `S3_REGION`, `S3_ENDPOINT`, `S3_USE_PATH_STYLE`

## Data node

Set these in `data-node/.env.data-node` only when you run the fallback stack:

- bind controls: `PRIVATE_BIND_IP`, `MQTT_PLAINTEXT_BIND_IP`, `MQTT_TLS_BIND_IP`, `NATS_MONITOR_BIND_IP`, `EMQX_DASHBOARD_BIND_IP`
- EMQX bootstrap/admin: `EMQX_DASHBOARD_USERNAME`, `EMQX_DASHBOARD_PASSWORD`, `EMQX_API_KEY`, `EMQX_API_SECRET`, `EMQX_NODE_COOKIE`
- MQTT public DNS/TLS: `MQTT_PUBLIC_HOSTNAME`, `EMQX_SSL_ENABLED`
- MQTT app/device auth: `MQTT_USERNAME`, `MQTT_PASSWORD`

## Preferred production mapping

Use this split by default:

- PostgreSQL: managed
- Redis: managed when needed, otherwise unset
- object storage: managed S3-compatible
- NATS: self-hosted on the data node today, replaceable later by changing `NATS_URL`
- MQTT: self-hosted EMQX on the data node today, replaceable later by changing `MQTT_BROKER_URL`

No application code should need to change when Postgres, Redis, S3, NATS, or MQTT endpoints move, as long as the target service matches the existing env contract.

The legacy single-host env file at `deployments/prod/.env.production.example` is retained only for rollback compatibility. It is not the primary production path.

## Edge notes

- Public API HTTPS belongs on `API_DOMAIN` through `caddy`
- Public MQTT/TLS belongs on `MQTT_PUBLIC_HOSTNAME` directly to EMQX on `8883`
- Do not assume the HTTP reverse proxy handles MQTT/TCP
- Keep admin/ops listeners such as `HTTP_OPS_ADDR`, `8222`, and `18083` private
