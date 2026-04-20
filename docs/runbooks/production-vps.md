# Production VPS (Docker Compose)

## MQTT publisher (API)

When `API_REQUIRE_MQTT_PUBLISHER=true`, the API process **must** connect to the broker at startup. Required env:

- `MQTT_BROKER_URL` (e.g. `tcp://emqx:1883` from inside Compose)
- `MQTT_CLIENT_ID` **or** `MQTT_CLIENT_ID_API` (Compose often maps the latter into the container)
- `MQTT_USERNAME` / `MQTT_PASSWORD` after `deployments/prod/scripts/emqx_bootstrap.sh` has run (that script uses **`EMQX_API_KEY` / `EMQX_API_SECRET`** as HTTP Basic auth on `/api/v5/*` to create the built-in MQTT user; dashboard credentials are not involved)

If the API exits with a MQTT wiring error, fix env + broker reachability before re-running deploy.

## Deploy / smoke / public checks

From `deployments/prod`:

```bash
bash scripts/deploy_prod.sh
bash scripts/healthcheck_prod.sh
```

If public HTTPS checks fail while internal API checks pass, set `SKIP_PUBLIC_HTTPS=1` for a focused smoke pass (DNS / ACME propagation):

```bash
SKIP_PUBLIC_HTTPS=1 bash scripts/healthcheck_prod.sh
```

## Rollback (images only)

```bash
bash scripts/rollback_prod.sh
```
