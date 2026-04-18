# EMQX (production lean profile)

This deployment runs a **single-node** EMQX 5.x container. Defaults:

- **MQTT TCP** listener on `0.0.0.0:1883` inside the container; published on the host as `1883` for `mqtt.ldtv.dev` (DNS A record to the VPS).
- **Dashboard** binds to **127.0.0.1:18083** only — use SSH port-forward for admin UI (`ssh -L 18083:127.0.0.1:18083 user@vps`).
- **Anonymous MQTT clients are disabled** (`EMQX_ALLOW_ANONYMOUS=false`).
- **Password authentication** via the **built-in database** is enabled from Compose (`EMQX_AUTHENTICATION__1__...`). Application users are **data**, not config: run `scripts/emqx_bootstrap.sh` after EMQX is up so the MQTT user from `MQTT_USERNAME` / `MQTT_PASSWORD` exists (idempotent).

## MQTT TLS (honest status)

**This profile does not automate MQTT over TLS (8883).** Devices use plaintext MQTT on **1883** unless you add certificates and EMQX SSL listener configuration yourself.

Follow-up (manual):

1. Obtain a certificate for `mqtt.ldtv.dev` (same ACME as HTTP, or DNS challenge).
2. Mount cert/key into the EMQX container and enable `listeners.ssl.default` in EMQX configuration.
3. Open **8883/tcp** in UFW only after TLS is working; prefer disabling **1883** publicly once clients migrate.

Document any listener changes in your runbook; do not commit real keys.
