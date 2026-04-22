# EMQX TLS certificate directory

Place real production MQTT TLS material here on the VPS. Do **not** commit certificate or private key files to git.

For the hardened 2-VPS edge model, issue the server certificate for the public MQTT DNS name you expect clients to use
(for example `mqtt.ldtv.dev`) and terminate raw MQTT/TLS directly in EMQX on `8883`. The HTTP reverse proxy on the
app nodes does not proxy MQTT/TCP.

Expected filenames for `base.hocon`:

- `ca.crt`
- `server.crt`
- `server.key`

Recommended handling:

- provision certificates on the server only
- restrict permissions so the private key is not world-readable
- keep this directory mounted read-only into the EMQX container

This repository intentionally ships **no** real certificate or key material.
