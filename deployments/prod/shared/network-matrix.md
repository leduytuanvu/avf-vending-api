# Production 2-VPS network matrix

This matrix makes the hardened edge assumptions explicit for the primary split production topology.

## Public DNS names

- `api.ldtv.dev`: public HTTPS API entrypoint terminated by `caddy` on each app node
- `mqtt.ldtv.dev`: public raw MQTT/TLS entrypoint terminated by EMQX on the data node when the self-hosted fallback broker is used

No separate public admin hostname is defined by default in this repo. Keep operator-only surfaces private and reach them by VPN, private network, or SSH tunnel.

## App node ports

- `22/tcp`: SSH, operator IPs only
- `80/tcp`: public ACME HTTP challenge and optional redirect handling
- `443/tcp`: public HTTPS for the API domain

Keep these app-node ports internal-only:

- `8080/tcp`: API container HTTP, reachable only from `caddy`
- `8081/tcp`: API ops listener, loopback only
- `9091/tcp`: worker metrics/health, loopback only
- `9092/tcp`: reconciler metrics/health, loopback only
- `9093/tcp`: mqtt-ingest metrics/health, loopback only
- `9094/tcp`: temporal-worker metrics/health, loopback only when the Temporal profile is enabled

## Data node ports

- `22/tcp`: SSH, operator IPs only
- `8883/tcp`: raw MQTT over TLS for devices and app processes when the fallback EMQX broker is in use

Keep these data-node ports internal-only:

- `4222/tcp`: NATS client port, app-node private network only
- `1883/tcp`: plaintext MQTT, default loopback/private-use only and never public edge
- `8222/tcp`: NATS monitoring, loopback or operator tunnel only
- `18083/tcp`: EMQX dashboard/API, loopback or operator tunnel only

## Exposure rules

1. `caddy` handles HTTP/HTTPS only. It does not proxy MQTT/TCP.
2. MQTT devices should target `mqtt.ldtv.dev:8883` when you run the fallback data node.
3. If you use normal DNS for MQTT, publish only the MQTT broker hostname; do not point MQTT clients at the API hostname.
4. Restrict `22/tcp` to operator IPs or a bastion.
5. Restrict NATS, Redis, Postgres, metrics, and admin ports to private networking only.

The legacy single-host compose file at `deployments/prod/docker-compose.prod.yml` should not be treated as the primary network model for production.
