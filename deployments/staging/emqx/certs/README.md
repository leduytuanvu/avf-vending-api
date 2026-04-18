# EMQX TLS certificate directory

Place real staging MQTT TLS material here on the VPS. Do **not** commit certificate or private key files to git.

Expected filenames for `base.hocon`:

- `ca.crt`
- `server.crt`
- `server.key`

Recommended handling:

- provision certificates on the server only
- restrict permissions so the private key is not world-readable
- keep this directory mounted read-only into the EMQX container

This repository intentionally ships **no** real certificate or key material.
