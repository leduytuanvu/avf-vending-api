# EMQX (production lean profile)

This deployment runs a **single-node** EMQX 5.x container. Defaults:

- **MQTT TCP** listener on `0.0.0.0:1883` inside the container for internal Docker-network traffic only.
- **MQTT TLS** listener wiring is prepared in repo config for `0.0.0.0:8883`, but it stays **disabled** until real certificates are installed and the listener is explicitly enabled.
- **Dashboard** binds to **127.0.0.1:18083** only — use SSH port-forward for admin UI (`ssh -L 18083:127.0.0.1:18083 user@vps`).
- **Anonymous MQTT clients are disabled** (`EMQX_ALLOW_ANONYMOUS=false`).
- **Password authentication** via the **built-in database** is enabled from Compose (`EMQX_AUTHENTICATION__1__...`). Application users are **data**, not config: run `scripts/emqx_bootstrap.sh` after EMQX is up so the MQTT user from `MQTT_USERNAME` / `MQTT_PASSWORD` exists (idempotent).

## What is automated vs manual

Automated in repo:

- EMQX container wiring mounts `deployments/prod/emqx/base.hocon`
- EMQX mounts `deployments/prod/emqx/certs/` read-only at `/opt/emqx/etc/certs`
- host port `8883/tcp` is published for the TLS listener path
- plaintext `1883` is no longer published on the host; it remains available only on the internal Docker network for in-stack traffic

Still intentionally manual:

- obtaining a real certificate for the MQTT hostname
- placing `ca.crt`, `server.crt`, and `server.key` on the VPS under `deployments/prod/emqx/certs/`
- enabling the TLS listener in `deployments/prod/emqx/base.hocon`
- deciding whether to keep server-only TLS (`verify_none`) or move to client-certificate validation (`verify_peer`)

## MQTT TLS status

Current safe default:

- `1883` is **internal-only / transitional**
- `8883` is the intended public enterprise path
- the TLS listener is defined but disabled until certificate material exists

Activation steps:

1. Obtain a certificate for your MQTT hostname such as `mqtt.ldtv.dev`.
2. Copy the real files to `deployments/prod/emqx/certs/` on the VPS with these names:
   - `ca.crt`
   - `server.crt`
   - `server.key`
3. Edit `deployments/prod/emqx/base.hocon` and change:
   - `listeners.ssl.default.enable = false` to `true`
4. Restart EMQX with your normal production operational flow.
5. Update clients to use `ssl://...:8883` (or equivalent TLS transport setting) before considering any further tightening.

## Enterprise-oriented hardening path

Server-authenticated TLS is the conservative first step:

- keep `verify = verify_none`
- keep `fail_if_no_peer_cert = false`
- authenticate clients with the existing EMQX built-in database username/password flow

After client migration, you can harden further:

- require client certificates by switching to `verify = verify_peer`
- set `fail_if_no_peer_cert = true`
- document CA rotation and certificate issuance/runbook steps outside git

## Bootstrap expectations

- `scripts/emqx_bootstrap.sh` still manages the MQTT application user in the built-in database
- TLS certificate provisioning is **separate** from MQTT user bootstrap
- do not commit real keys, CA bundles, or issued certificates into this repository
