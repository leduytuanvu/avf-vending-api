# Production cutover and rollback (2-VPS)

This runbook is for the new 2-VPS production topology under:

- `deployments/prod/app-node/`
- `deployments/prod/data-node/`
- `deployments/prod/shared/`

Use this together with:

- `docs/runbooks/production-2-vps.md`
- `deployments/prod/app-node/README.md`
- `deployments/prod/data-node/README.md`

## Scope

Use this runbook for:

- initial production cutover from the legacy single-VPS path to the new 2-VPS path
- planned production promotions on the 2-VPS path
- fast rollback to the previous 2-VPS revision
- emergency fallback to the legacy single-VPS path when the new topology is unstable

## Preconditions before cutover

1. Confirm the real env files exist on the hosts:
   - `deployments/prod/app-node/.env.app-node`
   - `deployments/prod/data-node/.env.data-node` only if self-host fallback NATS/MQTT is in use
2. Confirm image refs are immutable digests:
   - `APP_IMAGE_REF=ghcr.io/...@sha256:...`
   - `GOOSE_IMAGE_REF=ghcr.io/...@sha256:...`
3. Confirm DNS is ready:
   - `api.ldtv.dev` -> the edge/load balancer or both app nodes, depending on your routing model
   - `mqtt.ldtv.dev` -> data node only when self-hosted fallback EMQX is used
4. Confirm managed dependencies are reachable from both app nodes:
   - PostgreSQL
   - Redis if used
   - object storage
   - managed MQTT if used instead of the fallback data node
5. Confirm the private network path exists between app nodes and the fallback data node for:
   - `4222/tcp` NATS
   - `8883/tcp` MQTT/TLS
6. If you run observability separately, confirm it is live before traffic moves. Grafana is not part of this repo snapshot.

## Pre-cutover validation

Run from the repository root before touching production:

```bash
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml config
docker compose --env-file deployments/prod/app-node/.env.app-node.example -f deployments/prod/app-node/docker-compose.app-node.yml --profile migration config
docker compose --env-file deployments/prod/data-node/.env.data-node.example -f deployments/prod/data-node/docker-compose.data-node.yml config
```

On each target host:

```bash
cd /opt/avf-vending-api/deployments/prod/app-node
bash ../shared/scripts/bootstrap_prereqs.sh app-node
bash scripts/healthcheck_app_node.sh
```

If the fallback data node is part of the cutover:

```bash
cd /opt/avf-vending-api/deployments/prod/data-node
bash ../shared/scripts/bootstrap_prereqs.sh data-node
bash scripts/healthcheck_data_node.sh
```

## Production cutover checklist

1. Freeze non-essential production changes.
2. Record the current production image refs, DNS records, env file checksums, and current rollback command.
3. Verify the latest managed PostgreSQL snapshot/PITR window and any object-storage versioning assumptions before deploy.
4. If the fallback data node is used, release it first:

```bash
cd /opt/avf-vending-api/deployments/prod/data-node
bash scripts/release_data_node.sh
```

5. Release app nodes sequentially:

```bash
cd /opt/avf-vending-api/deployments/prod/app-node
export APP_NODE_HOSTS="app-node-a app-node-b"
export APP_NODE_REMOTE_DIR=/opt/avf-vending-api/deployments/prod/app-node
bash scripts/release_app_cluster.sh ghcr.io/<owner>/<repo>@sha256:<app> ghcr.io/<owner>/<repo>-goose@sha256:<goose>
```

6. Confirm node A is healthy before node B is updated.
7. Verify public API HTTPS:
   - `GET https://api.ldtv.dev/health/live`
   - `GET https://api.ldtv.dev/health/ready`
   - `GET https://api.ldtv.dev/version`
8. If self-hosted MQTT fallback is in use, verify MQTT/TLS reachability on `mqtt.ldtv.dev:8883`.
9. Watch the first 15-30 minutes of:
   - API readiness
   - worker outbox lag
   - telemetry consumer lag
   - mqtt-ingest error rate
   - EMQX/NATS health when fallback services are used
10. Remove any temporary maintenance window notices only after metrics and logs stabilize.

## Immediate rollback on the 2-VPS path

Use this when the new release is bad but the 2-VPS topology itself is still sound.

Cluster rollback:

```bash
cd /opt/avf-vending-api/deployments/prod/app-node
export APP_NODE_HOSTS="app-node-a app-node-b"
bash scripts/rollback_app_cluster.sh
```

Single-node rollback:

```bash
cd /opt/avf-vending-api/deployments/prod/app-node
bash scripts/rollback_app_node.sh
```

Fallback data-node rollback:

```bash
cd /opt/avf-vending-api/deployments/prod/data-node
bash scripts/rollback_data_node.sh
```

After rollback:

1. Confirm readiness on both app nodes.
2. Confirm the public API edge is back.
3. Confirm background lag is returning to baseline.
4. Write down whether schema or env drift remains. Image rollback does not undo schema changes.

## Emergency fallback to legacy single-VPS path

Use this only if the new 2-VPS topology itself is broken and the old single-VPS deployment is still available.

1. Freeze client/device changes.
2. Point traffic away from the 2-VPS app nodes.
3. Re-activate the legacy host with:
   - `deployments/prod/docker-compose.prod.yml`
   - the last known-good `deployments/prod/.env.production`
4. Run the legacy health checks:

```bash
cd /opt/avf-vending-api/deployments/prod
bash scripts/healthcheck_prod.sh
```

5. Repoint public DNS or load balancer targets back to the legacy path.
6. Do not re-enable the 2-VPS path until the root cause is understood.

## Operator stop conditions

Stop the cutover immediately if any of these happens:

- node A does not pass readiness after release
- public API `/health/ready` stays down for more than 2 minutes
- outbox publish failures become sustained
- telemetry consumer lag rises continuously instead of draining
- managed PostgreSQL or fallback NATS/EMQX is unhealthy
- rollback commands fail on the first node
