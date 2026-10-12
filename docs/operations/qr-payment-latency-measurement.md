# QR payment latency measurement

Use this checklist before/after quote prefetch, batch pricing, or checkout snapshot changes.

## Archived baseline (no adb device, 2026-10-13)

- Field capture matrix **not run** (`adb devices` empty on 2026-10-13 after deploy).
- Prior log excerpt (`.cursor_log_1625_vend.txt`): failed prefetch quotes + materialize `CreateQuote` ~2981ms for 3 lines — indicates **duplicate quote RPC** on old APK and quote-bound materialize.
- Production API deployed: `git_sha` **`e7002faa`** (2026-10-12T23:41:52Z) — snapshot topology quote + `CREATE_ORDER_FROM_QUOTE_PHASES` + `pricing_mode` on `CREATE_QUOTE_PHASES`.
- Kiosk APK: `assembleTcnProductionRelease` built locally; **not installed** (no adb). Install path: `app/build/outputs/apk/tcnProduction/release/`.

Re-run `run-qr-latency-measurement-matrix.ps1` when a kiosk is on adb and append results below.

### Infra decision (pre-matrix)

Per [qr-infra-scaling-decision.md](qr-infra-scaling-decision.md): **no VPS scale** until matrix + server logs show quote/DB dominance over `psp_create_ms` + kiosk RTT.

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
| `CREATE_QUOTE_PHASES` | `line_count`, `pricing_ms`, `persist_ms`, `total_ms`, `has_pricing_snapshot`, `pricing_mode` |
| `CREATE_ORDER_FROM_QUOTE_PHASES` | `vend_unit_count`, `vend_insert_ms`, `total_ms` |
| `CREATE_PAYMENT_SESSION_PHASES` | `order_load_ms`, `psp_create_ms`, `total_ms`, … |

**Bulk vend insert:** only implement if `vend_insert_ms` is material vs `total_ms` at high `vend_unit_count`; logging is live for that decision (see `qr-infra-scaling-decision.md`).

**Interpretation:** If `psp_create_ms` dominates `total_ms` on payment session, quote work is not the QR bottleneck. If `CREATE_QUOTE_PHASES pricing_ms` scales with SKU count, batch pricing may still need tuning.

## Weak network

Throttle (e.g. 3G profile) and repeat. Prefetch hit should remove one full `CreateQuote` RTT before materialize/payment.

## Guided matrix (Windows)

From `avf-vending-app` with kiosk on adb:

```powershell
.\scripts\diagnostics\run-qr-latency-measurement-matrix.ps1
# or one scenario:
.\scripts\diagnostics\run-qr-latency-measurement-matrix.ps1 -Scenario 5sku
```

After each QR checkout on production API (`CREATE_QUOTE_PHASES` in server logs), compare `pricing_ms` for `line_count=1` vs `5`.
