# EMQX (production lean profile)

This deployment runs a **single-node** EMQX 5.x container. Defaults:

- **MQTT TLS** listener is the primary external production path on `0.0.0.0:8883`.
- **MQTT TCP** listener exists only for internal/private compatibility on `127.0.0.1:1883` in repo config and must not be treated as the public production listener.
- **Dashboard** binds to **127.0.0.1:18083** only — use SSH port-forward for admin UI (`ssh -L 18083:127.0.0.1:18083 user@vps`).
- **Anonymous MQTT clients are disabled** (`EMQX_ALLOW_ANONYMOUS=false`).
- **Password authentication** via the **built-in database** is enabled from Compose (`EMQX_AUTHENTICATION__1__...`). Application users are **data**, not config: run `scripts/emqx_bootstrap.sh` after EMQX is up so the MQTT user from `MQTT_USERNAME` / `MQTT_PASSWORD` exists (idempotent).

## What is automated vs manual

Automated in repo:

- EMQX container wiring mounts `deployments/prod/emqx/base.hocon`
- EMQX mounts `deployments/prod/emqx/certs/` read-only at `/opt/emqx/etc/certs`
- TLS listener is enabled in `deployments/prod/emqx/base.hocon`
- host port `8883/tcp` is published for the public TLS listener path
- plaintext `1883` must remain loopback-only or private-network-only when it is kept for compatibility
- shared production bootstrap checks now fail fast if the required TLS certificate files are missing while `EMQX_SSL_ENABLED=true`

Still intentionally manual:

- obtaining a real certificate for the MQTT hostname
- placing `ca.crt`, `server.crt`, and `server.key` on the VPS under `deployments/prod/emqx/certs/`
- deciding whether to keep server-only TLS (`verify_none`) or move to client-certificate validation (`verify_peer`)
- **per-machine topic ACL**: copy `acl.conf.example` to `/opt/emqx/etc/acl.conf` (replace `TOPIC_PREFIX` with `MQTT_TOPIC_PREFIX`, usually `avf/devices`), provision **distinct** MQTT usernames for API publish (`avf-mqtt-api` pattern), mqtt-ingest (`avf-mqtt-ingest` pattern), and field devices (username equals machine UUID when using percent-u rules in the ACL file); merge `authorization.snippet.hocon` and set `authorization.enable = true` after the file exists on disk

## Per-machine ACL (EMQX)

Repo templates:

- `deployments/prod/emqx/acl.conf.example` — file-based rules; machine **A** cannot publish/subscribe machine **B** topics when MQTT username is scoped to machine UUID.
- `deployments/prod/emqx/authorization.snippet.hocon` — wire authorization to `/opt/emqx/etc/acl.conf`. **Start with `authorization.enable = false`** for greenfield bring-up, then flip to `true` once `acl.conf` is installed and VMQ/CLI tests pass.

Without ACLs, authentication alone does not stop a compromised client from publishing to another machine’s topic tree; **enable authorization in production** before wide field rollout.

## MQTT TLS status

Current production posture:

- `8883` is the public production listener and is TLS-first
- `1883` is **private-network-only / compatibility-only**
- public production clients should use `ssl://<mqtt-host>:8883`
- plaintext external exposure is not an acceptable final production posture

Required setup:

1. Obtain a certificate for your MQTT hostname such as `mqtt.ldtv.dev`.
2. Copy the real files to `deployments/prod/emqx/certs/` on the VPS with these names:
   - `ca.crt`
   - `server.crt`
   - `server.key`
3. Keep `EMQX_SSL_ENABLED=true` in the data-node env unless you are in a tightly scoped internal-only exception.
4. Restart EMQX with your normal production operational flow.
5. Point production clients at `ssl://...:8883` (or the equivalent TLS transport setting).

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
- `deployments/prod/shared/scripts/bootstrap_prereqs.sh data-node` now requires the certificate files when TLS is enabled
- do not commit real keys, CA bundles, or issued certificates into this repository
