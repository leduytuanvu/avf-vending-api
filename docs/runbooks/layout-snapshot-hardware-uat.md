# Physical TCN UAT — Layout lifecycle, snapshots, and history

Use a non-production TCN machine. Do not run destructive vend tests on production.

## Phase 0 — Runtime evidence (before changing defaults)

Collect these on the target machine **before** enabling capture or declaring history broken:

1. **API migration:** `SELECT version_id FROM goose_db_version;` — expect **34** (`00034_layout_snapshot_reason_periodic_5m`).
2. **Feature flag:** bootstrap / admin feature flags for `periodic_snapshot_capture_enabled` (defaults **false** in seed `00033`).
3. **App outbox:** Room `layout_snapshot_outbox` — count rows by `state` (`PENDING` vs `ACKED`).
4. **API history:** `GET /admin/machines/{machineId}/layout-history?limit=5` — confirm empty vs error vs rows.
5. **Logcat:** filter `LayoutSnapshotScheduler|LayoutSnapshotObserver|LAYOUT_SNAPSHOT_` for skip reasons (`periodic_capture_disabled`, `before_next_capture_at`, `duplicate_interval`, etc.).

Enable capture via admin feature flag (recommended) — do **not** flip app code default until ops approves fleet rollout.

## Preconditions

- API deployed with goose migration `00034` applied.
- Web admin deployed with layout history pages.
- Android build with `periodic_snapshot_capture_enabled=true` for the test machine (via bootstrap/admin flag).
- Technician credentials available.

## Hardware lifecycle and storefront recovery

1. Boot into normal storefront; confirm SELL_READY.
2. Confirm TCN runtime connected (technician diagnostics or logs: `TECH_HW_RUNTIME_READY`).
3. Enter technician area → **Chỉnh sửa sơ đồ & tồn kho**.
4. Verify top-bar layout dropdown lists layouts; active layout shows **Đang dùng**.
5. Select another layout for viewing only; confirm physical lanes/merge state unchanged on board.
6. Run one safe TCN planogram command (merge/split or motor rotate) if required for your test matrix.
7. Navigate back to storefront without force-killing the app.
8. Confirm:
   - TCN runtime reconnects (`TECH_HW_SESSION_EXIT_DONE status=RESTORED` or runtime connect logs).
   - Hardware health returns healthy; storefront is not stuck on **Máy đang bảo trì phần cứng.**
   - A test sale can complete (if allowed on test machine).

## Explicit layout activation (optional, test layout only)

9. Re-open planogram editor; select inactive layout.
10. Tap **Áp dụng layout**; confirm journal steps complete and active layout marker updates.
11. Return to storefront; repeat step 8.

## Planogram bulk inventory (60-slot timing)

12. Open planogram editor with refill session active; select all slots (or a 60-slot grid).
13. Run **Đặt tồn** or **Đầy tồn**; confirm blocking modal appears (white card, back blocked).
14. Stopwatch: bulk op should complete in **one** network round-trip (~seconds, not ~2s/slot).
15. Logcat: single `PLANOGRAM_BULK_OPERATION_START` / `PLANOGRAM_BULK_SUBMIT_DONE` pair per action.
16. Visual: grid stays responsive during modal; no multi-second per-slot stall.

## Capacity invariant spot checks

17. On one slot at **10/10**, set capacity **6** → expect **6/6** (stock clamped).
18. On **4/10**, set capacity **6** → expect **4/6** (stock unchanged).
19. On **4/6**, set capacity **10** → expect **4/10** (no auto-fill).

## Five-minute snapshot cadence

20. Leave machine on SERVICE_READY for ≥12 minutes (two intervals).
21. Confirm Android logs:
    - `LAYOUT_SNAPSHOT_TICK reason=PERIODIC_5M`
    - `LAYOUT_SNAPSHOT_CAPTURED` (or structured `LAYOUT_SNAPSHOT_SKIP` with documented reason)
    - `LAYOUT_SNAPSHOT_UPLOAD ... acked=`
22. Open Web → machine → layout history (per layout or all layouts).
23. Confirm new rows with reason `PERIODIC_5M`, distinct `captureSequence`, captured/received timestamps.
24. Retry/offline scenario: disable network for one interval, re-enable; confirm no duplicate rows for the same snapshot id / capture sequence.

## Web history inspection

25. Open an individual snapshot detail page; verify slot rows match device state at capture time.
26. Confirm opening history/detail does not change active layout on server or device.

## QR sandbox (configuration — not UI bypass)

27. API: set `PAYMENT_ENV` to sandbox/live per [enable-live-payment-production.md](./enable-live-payment-production.md) — **not** `cash_only`.
28. Admin: enable intended QR methods on the test machine (`momo`, `zalopay`, etc.).
29. Device: bootstrap refresh; logcat `PAYMENT_QR_UI_GATE` shows `deploymentSupported=true`, non-empty `configuredMethods`, `pspRuntimeReady=...`.
30. Complete one sandbox QR session; confirm provider UI visible and no prod money movement.

> **Note:** [production-sshd-recover-and-deploy.yml](../../.github/workflows/production-sshd-recover-and-deploy.yml) reverts app-node payment env to `cash_only` before recover rollout — re-apply live payment env after recover if QR UAT/production QR is required.

## Pass criteria

- Storefront recovers after planogram technician session.
- Bulk inventory uses one coordinator submit; modal blocks interaction until dismissed.
- Capacity scenarios match invariant above.
- Periodic snapshots appear on Web under correct machine/layout when flag enabled.
- Transport retries do not create duplicate history rows for the same capture.
- QR visible only when deployment + machine methods configured (CASH-only machines show no QR).
