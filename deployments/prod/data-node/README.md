# Production data node

This directory is the primary data-plane companion for the current `2-VPS` production topology.

It is optional because the interim target prefers managed services where available, but when you still self-host broker infrastructure this is the only recommended place for it.

It intentionally contains only the self-hosted components that still make sense on a budget split deployment:

- `nats`: JetStream for the current production worker/telemetry path
- `emqx`: MQTT broker fallback until you move to a managed MQTT endpoint

What is intentionally not here:

- PostgreSQL, because the target topology prefers managed Postgres
- Redis, because it is optional for this repo and should stay managed when needed
- object storage, because the target topology prefers managed S3-compatible storage

## Files

- `docker-compose.data-node.yml`: fallback broker stack
- `.env.data-node.example`: bind and broker credential template
- `../shared/env-matrix.md`: env ownership and target managed-service split
- `../shared/network-matrix.md`: edge and internal port exposure matrix
- `../shared/scripts/release_data_node.sh`: shared remote rollout entrypoint used by operators and CI
- `../shared/scripts/healthcheck_data_node.sh`: shared remote healthcheck wrapper used by operators and CI
- `scripts/release_data_node.sh`: idempotent data-node rollout
- `scripts/healthcheck_data_node.sh`: explicit broker health verification
- `scripts/rollback_data_node.sh`: rollback to the previous snapshotted compose/env revision
- `scripts/bootstrap_emqx_data_node.sh`: idempotent EMQX built-in user bootstrap for app MQTT clients

## Network contract

App nodes connect to the data node over host ports, not Docker-internal service discovery:

- `nats://<data-node-private-ip>:4222`
- `ssl://mqtt.ldtv.dev:8883` for the hardened public MQTT path
- `tcp://127.0.0.1:1883` only for local/private troubleshooting when you intentionally keep plaintext MQTT enabled

Keep dashboard/monitor ports private:

- NATS monitor: `8222`
- EMQX dashboard/API: `18083`

MQTT exposure is separate from HTTP edge handling:

- `caddy` on the app nodes does not proxy MQTT/TCP
- publish a DNS name such as `mqtt.ldtv.dev` directly to the data node if you use the fallback broker
- terminate raw MQTT/TLS in EMQX on `8883`
- keep plaintext `1883` loopback-only or private-only

Keep `EMQX_API_KEY`, `EMQX_API_SECRET`, `MQTT_USERNAME`, and `MQTT_PASSWORD` in `.env.data-node` even though the compose file itself only needs the dashboard defaults and node cookie. Those values exist so the current operator/bootstrap flow can keep provisioning or rotating the MQTT application user without changing the app-node env contract.

## Validation

From this directory:

```bash
docker compose --env-file .env.data-node.example -f docker-compose.data-node.yml config
bash ../shared/scripts/bootstrap_prereqs.sh data-node
bash scripts/healthcheck_data_node.sh
```

## Release flow

```bash
bash ../shared/scripts/bootstrap_prereqs.sh data-node
export DATA_NODE_HOST=data-node-vps
export PRODUCTION_DEPLOY_ROOT=/opt/avf-vending-api
bash ../shared/scripts/release_data_node.sh
```

The release path is deterministic and idempotent:

1. validate env, compose, and host prerequisites
2. snapshot the current env/compose revision into `.deploy/previous`
3. pull `nats` and `emqx`
4. restart the fallback broker stack
5. re-run EMQX app-user bootstrap
6. verify explicit health endpoints

## Rollback

```bash
bash scripts/rollback_data_node.sh
```

## Notes

- This stack is the only supported place for self-hosted broker services in the current production topology.
- If you later adopt managed MQTT and/or managed NATS, update only app-node env such as `MQTT_BROKER_URL` and `NATS_URL`.
- `deployments/prod/docker-compose.prod.yml` remains only as a legacy single-host rollback path.
- The legacy single-host path is not the primary production path and is not recommended for the enterprise target deployment.
- Install real MQTT TLS certificates under `../emqx/certs/` before exposing `8883`.
