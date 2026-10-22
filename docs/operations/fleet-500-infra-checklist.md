# Fleet ~500 machines — infra limits (do not apply while idle)

Live snapshot 22 Oct 2026 03:16 UTC: both VPS idle (load under 0.2, API CPU a few percent, RAM ~1G of 8G). **Do not raise these limits until fleet is tens-hundreds of machines and metrics show saturation.** Do **not** switch VPS provider for QR/API speed.

## Current vs 500-machine targets

| Knob | Live (idle, ~few machines) | Apply around ~50–100 | ~500 machines |
|------|----------------------------|----------------------|---------------|
| `TELEMETRY_STREAM_MAX_BYTES` | 256 MiB | 512 MiB | 2 GiB |
| NATS compose CPU/RAM | 0.35 / 384M | keep | ~1.0 / 1G |
| EMQX compose CPU/RAM | 0.6 / 768M (~223M used) | keep | ~1.5 / 1.5G |
| Postgres compose | 1.0 CPU / 1.5G (~134M used) | keep | 2 CPU / 4G on same VPS A |
| API compose CPU | 1.50 | keep | 2.00 if p95 high and CPU over 70% |
| `API_DATABASE_MAX_CONNS` | 4 | 10-25 | 25-35 / node; 2 nodes under ~70% of PgBouncer 200 |
| App-node C | off | off | off unless A CPU + pool proven |
| VPS SKU | Hostinger KVM2 2 vCPU / 8G KL | keep | consider A 16G only if Postgres/telemetry saturates |
| Swap | 0 | 1–2G or more RAM before 500 | same |

JetStream disk worst case ≈ 6 × `TELEMETRY_STREAM_MAX_BYTES` + 30% free. B has 90G free today.

## Do not

- Change VPS region/provider to make QR faster (PSP + kiosk RTT dominate).
- Enable a third app-node to hide QR latency (doubles DB clients).
- Raise HTTP/gRPC timeouts expecting shorter responses.
- Delete the paused Supabase project until 48h + off-box backup are done.
- Point production `DATABASE_URL` at localhost.

## Apply order when fleet grows

1. Confirm `docker stats` and `avf_db_pool_*` / JetStream `nats stream info` Bytes near MaxBytes.
2. Raise `TELEMETRY_STREAM_MAX_BYTES` then NATS/EMQX limits on B.
3. Raise Postgres CPU/RAM on A.
4. Raise API CPU / `API_DATABASE_MAX_CONNS` last, after pooler headroom check.
5. Split NATS+EMQX to a third VPS only if B disk/CPU is the bottleneck.

See [production-resource-sizing.md](./production-resource-sizing.md) and [telemetry-jetstream-resilience.md](../runbooks/telemetry-jetstream-resilience.md).
