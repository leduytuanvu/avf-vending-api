# QR latency: infra scaling decision (do not scale before numbers)

## Default

**Do not** upgrade Hostinger app-node CPU/RAM or enable a second app-node **only** to improve time-to-QR. QR is dominated by:

1. Sequential gRPC: CreateQuote → CreateOrderFromQuote → CreatePaymentSession (each up to ~30s client deadline).
2. Kiosk WAN RTT (VN → API KL).
3. `psp_create_ms` (API KL → MoMo VN, sync; QR payload required).

VPS CPU is relevant only when sustained API container CPU is high **while** `psp_create_ms` is a small fraction of wall clock.

## Measure first

| Signal | Source | Scale VPS when |
|--------|--------|----------------|
| Quote CPU work | `CREATE_QUOTE_PHASES` (`pricing_ms`, `persist_ms`, `pricing_mode=machine_snapshot_topology`) | High `pricing_ms` at `line_count=1` **and** low `psp_create_ms` |
| Payment DB | `CREATE_PAYMENT_SESSION_PHASES.order_load_ms` | High vs `psp_create_ms` consistently |
| Order vend inserts | `CREATE_ORDER_FROM_QUOTE_PHASES` (`vend_insert_ms`, `vend_unit_count`) | High `vend_insert_ms` with large qty → consider bulk insert (code), not VPS |
| API saturation | `docker stats` on app-node | Sustained >70% of compose CPU limit **and** quote phases high |
| Pool exhaustion | `avf_db_pool_acquired_conns` vs max | Sustained near max with elevated `pricing_ms` / `order_load_ms` |

## When geography beats more vCPU

If `psp_create_ms` + kiosk RTT account for **>~60%** of tap→QR on p95:

- Prefer **API egress closer to VN / MoMo** (ops project).
- Keep Postgres on Supabase `ap-southeast-1` unless DB RTT is separately proven high.
- Redis does **not** accelerate CreateQuote (not on that path).

## Actions explicitly deferred without data

- Second app-node B (doubles pooler pressure).
- Increasing HTTP/gRPC timeouts (masks failures, does not shorten QR).
- Fire-and-forget PSP create (breaks sync QR payload contract).

## Record outcomes

After each measurement matrix run, append date, scenario, client parse summary, and correlated server phase medians to `qr-payment-latency-measurement.md` (baseline section).
