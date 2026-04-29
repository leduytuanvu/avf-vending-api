# Machine gRPC in production (`cmd/api`)

Machine-facing **`avf.machine.v1`** RPCs listen on **`GRPC_ADDR`** when **`GRPC_ENABLED=true`** or **`MACHINE_GRPC_ENABLED=true`** (production requires **`MACHINE_GRPC_ENABLED=true`** explicitly — see [`transport-boundary.md`](../architecture/transport-boundary.md)).

## Fail-closed surface

| Requirement | Notes |
| ----------- | ----- |
| TLS termination | **`GRPC_TLS_ENABLED=true`** with **`GRPC_TLS_CERT_FILE`** / **`GRPC_TLS_KEY_FILE`**, **or** **`GRPC_BEHIND_TLS_PROXY=true`** when TLS terminates at Caddy/Nginx/LB (plaintext **h2c** only on the trusted bridge). Plain TCP gRPC **without** either flag is **rejected** when **`APP_ENV=production`**. |
| Public URL | **`GRPC_PUBLIC_BASE_URL`** must be set (e.g. **`grpcs://machine-api.example.com:443`**) so operators and apps agree on the advertised endpoint. |
| Mutual exclusion | **`GRPC_BEHIND_TLS_PROXY=true`** cannot be combined with **`GRPC_TLS_ENABLED=true`** — choose one termination point. |
| Reflection | **`GRPC_REFLECTION_ENABLED`** must stay **`false`** in production (validated). |
| Health | **`GRPC_HEALTH_USE_PROCESS_READINESS=true`** in production so **`grpc.health.v1.Health/Check`** tracks real readiness (validated). |

## Expected vending endpoint

Clients should target **`grpcs://machine-api.<your-domain>:443`** (TLS). Match **`GRPC_PUBLIC_BASE_URL`** to that URI.

## Caddy (reverse proxy → plaintext upstream)

Terminate TLS on **`machine-api.<domain>`**, forward **`h2c`** to the API container’s **`GRPC_ADDR`** (e.g. **`api:9090`**).

Sample site block (adapt paths/domains):

```caddyfile
machine-api.example.com {
	tls /etc/ssl/machine-api/fullchain.pem /etc/ssl/machine-api/privkey.pem
	reverse_proxy h2c://api:9090 {
		transport http {
			versions h2c
		}
	}
}
```

Store full samples next to compose under **`deployments/prod/examples/`**.

On the API process set **`GRPC_BEHIND_TLS_PROXY=true`**, **`GRPC_ADDR=:9090`**, **`GRPC_TLS_ENABLED=false`**, and **`GRPC_PUBLIC_BASE_URL=grpcs://machine-api.example.com:443`**.

## Direct TLS on the API process

Set **`GRPC_TLS_ENABLED=true`**, PEM paths, optional **`GRPC_TLS_CLIENT_CA_FILE`** / **`GRPC_TLS_CLIENT_AUTH`** for mTLS. **`GRPC_BEHIND_TLS_PROXY=false`**.

## Health check

From any host that can reach the listener:

```bash
grpcurl -plaintext api:9090 grpc.health.v1.Health/Check
```

Behind TLS termination use **`grpcs://`** / `-cacert` as appropriate:

```bash
grpcurl machine-api.example.com:443 grpc.health.v1.Health/Check
```

Compose healthchecks may eventually extend HTTP probes with **`grpc_health_probe`**; until then rely on **`grpcurl`** or internal probes.

## Smoke script

From repo root (requires **`grpcurl`** on **`PATH`**):

```bash
make machine-grpc-smoke
```

See **`scripts/grpc_machine_smoke.sh`**.

## Size limits

**`GRPC_MAX_RECV_MSG_SIZE`** / **`GRPC_MAX_SEND_MSG_SIZE`** set **`grpc.Server`** bounds (bytes). **`0`** keeps library defaults (~4 MiB).

## Related

- Local development (plaintext): [`../local/grpc-local-test.md`](../local/grpc-local-test.md)
- API contract: [`../api/machine-grpc.md`](../api/machine-grpc.md)
