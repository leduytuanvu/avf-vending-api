# Market readiness go-live checklist

## Infrastructure

- [ ] DNS/TLS for `api.ldtv.dev`, gRPC host, MQTT `mqtt.ldtv.dev:8883`
- [ ] Postgres pooler limit vs sum(process `*_DATABASE_MAX_CONNS` × nodes)
- [ ] Redis in same region as API; `REDIS_CACHE_ENABLED` / `SALE_CATALOG_CACHE_TTL=45s`
- [ ] Cloudinary or media provider configured (`MEDIA_COMPANY_ID`, keys in VPS env only)
- [ ] Payment provider live keys (`PAYMENT_ENV=live`, webhook secret on VPS)
- [ ] MQTT `MQTT_TOPIC_LAYOUT=enterprise`, TLS, dedicated machine users

## Procedures

- [ ] Admin product publish: category → brand → tag → media → product → planogram → stock → publish
- [ ] Machine activation code issued before field install
- [ ] First boot: gRPC activation → token → bootstrap → catalog/media/inventory sync
- [ ] Rollback image digest documented

## Monitoring

- [ ] HTTP 5xx and p95 (`http_request_duration_seconds`)
- [ ] gRPC error rate
- [ ] `avf_db_pool_acquired_conns` / acquire duration
- [ ] Slow query log count (`DATABASE_SLOW_QUERY_LOG_MS`)
- [ ] MQTT disconnect / dispatch errors
- [ ] Outbox backlog / worker errors
- [ ] Payment webhook failures
- [ ] Vend failure rate

## Smoke (required before real sales)

```bash
BASE_URL=https://api.ldtv.dev ./scripts/production/smoke-market-readiness.sh
BASE_URL=https://api.ldtv.dev ACCESS_TOKEN=<token> ./scripts/production/smoke-market-readiness.sh
```

## Backup

- [ ] Postgres backup/restore drill documented for operator

## Incident

- [ ] Runbooks: deploy failure, rollback, postgres outage, MQTT broker outage
