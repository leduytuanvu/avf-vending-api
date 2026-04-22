# Production backup, restore, and DR (2-VPS)

This runbook documents what must be backed up, what is expected from managed services, and how to run a restore drill for the 2-VPS production topology.

## Backup ownership model

Target production mode:

- PostgreSQL: managed service
- Redis: managed service only when enabled
- object storage: managed S3-compatible
- app nodes: stateless
- fallback data node: optional self-hosted NATS + EMQX only

That means backup posture should separate:

1. **Managed services** that must provide snapshots or PITR
2. **Repo-managed config** that must be copied outside the VPSes
3. **Fallback self-hosted broker state** that must be treated as disposable unless you explicitly choose to preserve it

## What must be backed up

### Managed PostgreSQL

Minimum expectation:

- automated daily snapshots enabled
- PITR enabled with a retention window that matches your business recovery objective
- restore tested at least once per quarter

Verify before every production cutover:

1. last successful snapshot timestamp
2. PITR retention window and region
3. target instance size that a restore would use
4. operator access to initiate restore
5. current connection string and SSL requirements

Document outside the repo:

- provider console path for restore
- restore contact/owner
- current RPO and RTO target

### Object storage

Expected posture for production artifact buckets:

- versioning enabled
- bucket policy narrowed to only required principals
- lifecycle rules reviewed so they do not purge live or recent recovery versions too early
- encryption at rest enabled through the provider

Verify before every production cutover:

1. bucket versioning is still enabled
2. delete protection or recovery path exists for accidental object deletes
3. the app credentials in production are least-privilege and do not grant unnecessary bucket admin operations
4. presign/download path still works with the active bucket policy

### Config and env files

Back up these files outside the app nodes and outside the repo checkout:

- `deployments/prod/app-node/.env.app-node` from each app node
- `deployments/prod/data-node/.env.data-node` when fallback data services are used
- `deployments/prod/observability/.env.observability` where observability is deployed

Rules:

- store encrypted copies in your password manager, secrets manager, or encrypted backup vault
- keep one copy per node when node-specific values differ
- record checksums and last rotation date
- never rely on shell history as backup

### Fallback data node state

If you use self-hosted fallback NATS/EMQX:

- NATS JetStream data under the Docker volume is operationally useful but should not be treated as the primary system of record
- EMQX built-in auth state may be needed for recovery if you do not recreate users automatically

Recommended posture:

- treat Postgres as authoritative business state
- treat NATS/EMQX fallback state as rebuildable
- keep the broker env, credentials, and certificate material backed up

## What does not require backup from the app nodes

- container images, because they are rebuilt and pinned by digest
- application binaries on disk, because they come from the published images
- local app-node filesystem business state, because the intended topology is stateless

## Restore drill checklist

Run at least quarterly in staging or an isolated recovery environment.

1. Choose a recovery point:
   - latest snapshot
   - a PITR timestamp
2. Restore PostgreSQL to an isolated instance.
3. Restore or recreate object storage access with read-only validation first.
4. Copy backed-up env files into an isolated recovery environment.
5. Bring up the app-node stack against the restored Postgres:

```bash
docker compose --env-file deployments/prod/app-node/.env.app-node -f deployments/prod/app-node/docker-compose.app-node.yml up -d
```

6. If the fallback data node is part of the drill, bring it up too:

```bash
docker compose --env-file deployments/prod/data-node/.env.data-node -f deployments/prod/data-node/docker-compose.data-node.yml up -d
```

7. Validate:
   - `/health/live`
   - `/health/ready`
   - `/version`
   - a known machine lookup
   - a known order/payment read path
   - object download/presign path if artifacts are enabled
8. Confirm the restored data is isolated from production.
9. Record:
   - restore start and end time
   - PITR timestamp or snapshot id used
   - failures and manual fixes needed
   - whether RTO and RPO targets were met

## DR event: managed PostgreSQL unavailable

Symptoms:

- both app nodes fail readiness for multiple DB-backed processes
- migrations and health checks fail
- worker and mqtt-ingest logs show connection errors

First actions:

1. Confirm provider status and whether the outage is regional or instance-specific.
2. Freeze deploys and non-essential operational changes.
3. Confirm whether failover is automatic in your managed plan.
4. If failover is not automatic or recovery is slow, initiate provider restore or replica promotion.
5. Update `DATABASE_URL` only after the replacement endpoint is confirmed healthy.
6. Re-run app-node health checks after endpoint change.

Do not:

- repeatedly restart all app services before provider status is known
- attempt schema downgrade as a first reaction

## DR event: Redis unavailable

This repo treats Redis as optional for features that actually need it.

First actions:

1. Identify whether the active production feature set depends on Redis.
2. Check provider health and TLS/auth settings.
3. If Redis is optional for the current release, consider disabling the affected feature wiring rather than failing the entire stack.
4. If Redis is required for the current release, restore provider connectivity first, then restart only the affected processes.

## DR event: fallback MQTT broker loss

This applies only when `MQTT_BROKER_URL` points at the self-hosted fallback EMQX data node.

First actions:

1. Check `data-node/scripts/healthcheck_data_node.sh`.
2. Confirm certificate files still exist under `deployments/prod/emqx/certs/`.
3. Confirm port `8883` is reachable privately and publicly as intended.
4. If EMQX is unrecoverable, restore the data node from config and re-bootstrap MQTT users.
5. If you have a managed MQTT endpoint available, repoint `MQTT_BROKER_URL` on the app nodes and redeploy app nodes sequentially.

## DR event: config loss

If an app node is rebuilt and env files are missing:

1. Restore the node-specific `.env.app-node` from encrypted backup.
2. Confirm unique values such as `COMPOSE_PROJECT_NAME`, `APP_NODE_NAME`, `APP_INSTANCE_ID`, `MQTT_CLIENT_ID_API`, and `MQTT_CLIENT_ID_INGEST`.
3. Run `bash ../shared/scripts/bootstrap_prereqs.sh app-node`.
4. Release the node with the last known-good image refs.

## Recovery evidence to retain

After any real recovery or drill, store:

- exact snapshot or PITR point used
- restored env file checksum
- final health-check output
- open gaps that still required manual provider work
