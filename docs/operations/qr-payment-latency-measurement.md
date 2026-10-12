# QR payment latency measurement

Use this checklist before/after quote prefetch, batch pricing, or checkout snapshot changes.

## 1 SKU vs many SKU (client)

1. Capture logcat during one QR checkout per scenario (1 line, 5 lines, high qty).
2. Parse phases:

```powershell
adb logcat -d | .\avf-vending-app\scripts\diagnostics\parse-qr-payment-timing.ps1
```

Tags: `QR_PAYMENT_TIMING`, `GRPC_CREATE_QUOTE_*`, `GRPC_CREATE_PAYMENT_SESSION_*`, `PAYMENT_RAIL_DECISION`.

Compare:

- Time from checkout tap to first `QR_PAYMENT_TIMING` with payload ready.
- Count of `GRPC_CREATE_QUOTE_START` (expect **1** when prefetch fingerprint matches).
- `GRPC_CREATE_QUOTE_RESULT durationMs` vs line count.

## Server quote + payment phases

On an app node:

```bash
APP_NODE_DIR=/opt/avf/app-node ./scripts/production/measure-create-payment-session.sh
```

Correlate client + server:

```bash
SSH_HOST=root@<app-vps> APP_NODE_DIR=/opt/avf/app-node \
  CLIENT_LOGCAT=./qr-timing.log \
  ./scripts/production/correlate-qr-payment-latency.sh
```

Structured logs:

| Log | Fields |
|-----|--------|
| `CREATE_QUOTE_PHASES` | `line_count`, `pricing_ms`, `persist_ms`, `total_ms`, `has_pricing_snapshot` |
| `CREATE_PAYMENT_SESSION_PHASES` | `order_load_ms`, `psp_create_ms`, `total_ms`, … |

**Interpretation:** If `psp_create_ms` dominates `total_ms` on payment session, quote work is not the QR bottleneck. If `CREATE_QUOTE_PHASES pricing_ms` scales with SKU count, batch pricing may still need tuning.

## Weak network

Throttle (e.g. 3G profile) and repeat. Prefetch hit should remove one full `CreateQuote` RTT before materialize/payment.
