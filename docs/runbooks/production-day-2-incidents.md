# Production day-2 incidents (2-VPS)

This runbook covers the missing operational scenarios for the new topology after cutover.

Use this with:

- `docs/runbooks/production-2-vps.md`
- `docs/runbooks/telemetry-production-rollout.md`
- `ops/RUNBOOK.md`

## App node loss

Symptoms:

- one app node stops responding
- load balancer marks one target unhealthy
- Prometheus shows one node failing readiness while the other remains healthy

First actions:

1. Confirm whether only one app node is affected.
2. Keep traffic on the healthy node.
3. Check the failed node:
   - host reachability
   - Docker daemon
   - `docker compose ps` in `deployments/prod/app-node`
   - disk and memory pressure
4. If the host is gone, rebuild it using the backed-up `.env.app-node` and release only that node:

```bash
cd /opt/avf-vending-api/deployments/prod/app-node
bash ../shared/scripts/bootstrap_prereqs.sh app-node
bash scripts/release_app_node.sh ghcr.io/<owner>/<repo>@sha256:<app> ghcr.io/<owner>/<repo>-goose@sha256:<goose>
```

5. Do not touch the healthy node unless the issue is clearly release-wide.

## MQTT broker loss when self-hosted fallback is used

Symptoms:

- device offline spike
- mqtt-ingest readiness flaps or errors
- EMQX status on `18083` fails
- clients cannot connect to `mqtt.ldtv.dev:8883`

First actions:

1. Check `deployments/prod/data-node/scripts/healthcheck_data_node.sh`.
2. Confirm EMQX container state and certificate files.
3. Confirm the `8883` listener is still bound and firewall rules did not change.
4. Re-run EMQX user bootstrap if the broker state was rebuilt:

```bash
cd /opt/avf-vending-api/deployments/prod/data-node
bash scripts/bootstrap_emqx_data_node.sh
```

5. If the fallback broker cannot be restored quickly and a managed MQTT endpoint exists, repoint `MQTT_BROKER_URL` and roll app nodes sequentially.

## Managed DB unavailable

Symptoms:

- API, worker, reconciler, and mqtt-ingest all fail readiness together
- errors reference Postgres connection or transaction failures

First actions:

1. Verify provider incident status and failover behavior.
2. Pause deploys and batch operational changes.
3. If a new DB endpoint is required, update `DATABASE_URL` from secrets backup and redeploy app nodes one at a time.
4. Verify readiness before re-enabling traffic changes.

## Redis unavailable

Symptoms:

- only Redis-backed features fail
- the core API may still be partially healthy depending on feature flags

First actions:

1. Confirm whether Redis is actually required by the current release.
2. Check provider health, auth, and TLS settings.
3. If the current rollout can run without Redis-backed features, disable the affected optional feature wiring instead of broad rollback.
4. If Redis is required, restore provider connectivity before restarting processes.

## Disk-full or log-growth incident

Symptoms:

- Docker cannot start or rotate containers cleanly
- NATS/EMQX or Prometheus/Loki stop writing
- node-exporter shows low root filesystem free space

First actions:

1. Check host usage:
   - Docker container logs
   - Prometheus data volume
   - Loki data volume
   - NATS data volume when fallback broker is used
2. Remove only known-safe old logs or stale images.
3. Do not delete active Postgres backups, object-storage state, or live broker volumes blindly.
4. If observability is consuming space, reduce retention safely before deleting raw data paths.
5. Re-run the failing node health check after headroom is restored.

## Certificate renewal or rotation

### API HTTPS (`caddy`)

1. Confirm `API_DOMAIN` still resolves correctly.
2. If ACME renewal fails, check:
   - public `80/443`
   - DNS
   - Caddy logs
3. If you must rotate manually, install the replacement certificate using the same app-node edge model and verify:
   - `https://api.ldtv.dev/health/live`
   - `https://api.ldtv.dev/version`

### MQTT TLS (`emqx`)

1. Replace the certificate material in `deployments/prod/emqx/certs/` on the data node.
2. Keep filenames aligned with `deployments/prod/emqx/base.hocon`:
   - `ca.crt`
   - `server.crt`
   - `server.key`
3. Restart only the EMQX service if possible.
4. Verify `mqtt.ldtv.dev:8883` reachability and device reconnect behavior.

## Machine offline spike

Symptoms:

- many devices stop reporting heartbeats
- fleet dashboards show a sharp offline jump

First actions:

1. Determine whether the spike is global or site-specific.
2. If global:
   - check EMQX or managed MQTT health
   - check mqtt-ingest health
   - check public/private DNS for `mqtt.ldtv.dev`
3. If site-specific:
   - validate ISP/power/local networking before changing the backend
4. Watch telemetry backlog and mqtt-ingest error logs while recovering.

## Payment callback delay spike

Symptoms:

- payment completion takes longer than expected
- unresolved orders or pending payment states increase

First actions:

1. Check API health and external payment provider reachability.
2. Check worker outbox lag and publish failures.
3. Check reconciler summaries for payment-provider probe lag if actions are enabled.
4. Confirm the provider webhook/callback path still reaches the public API edge.

## Vend-result mismatch surge

Symptoms:

- more orders where payment and vend outcome disagree
- reconciler list jobs surface more `vend_stuck` or duplicate-payment style anomalies

First actions:

1. Check `cmd/reconciler` health and recent `reconciler_job_summary` logs.
2. Check device telemetry freshness and command receipt ingestion.
3. Determine whether this is concentrated to one firmware/site/cohort.
4. If the spike follows a deploy, pause further rollout before changing reconciler flags.

## Telemetry backlog surge

Symptoms:

- `avf_telemetry_consumer_lag` rising
- worker readiness degrading
- mqtt-ingest healthy but backlog still growing

First actions:

1. Check worker health first, then JetStream/NATS health.
2. Check `avf_telemetry_projection_db_fail_consecutive_max` and Postgres load.
3. Check whether device traffic volume changed suddenly.
4. Use `docs/runbooks/telemetry-production-rollout.md` and `docs/runbooks/telemetry-jetstream-resilience.md` before tuning throughput knobs.

## Incident closeout checklist

Before closing any incident above:

1. record timeline and impacted nodes/services
2. record exact env or image changes made
3. record whether the issue was managed-service, fallback-broker, or app-node specific
4. add the recovery steps that actually worked to the team ops notes
