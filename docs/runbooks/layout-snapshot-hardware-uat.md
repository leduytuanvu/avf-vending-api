# Physical TCN UAT — Layout lifecycle, snapshots, and history

Use a non-production TCN machine. Do not run destructive vend tests on production.

## Preconditions

- API deployed with goose migration `00034` applied.
- Web admin deployed with layout history pages.
- Android build with `periodic_snapshot_capture_enabled=true` for the test machine.
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

## Five-minute snapshot cadence

12. Leave machine on SERVICE_READY for ≥12 minutes (two intervals).
13. Confirm Android logs:
    - `LAYOUT_SNAPSHOT_TICK reason=PERIODIC_5M`
    - `LAYOUT_SNAPSHOT_CAPTURED`
    - `LAYOUT_SNAPSHOT_UPLOAD ... acked=`
14. Open Web → machine → layout history (per layout or all layouts).
15. Confirm new rows with reason `PERIODIC_5M`, distinct `captureSequence`, captured/received timestamps.
16. Retry/offline scenario: disable network for one interval, re-enable; confirm no duplicate rows for the same snapshot id / capture sequence.

## Web history inspection

17. Open an individual snapshot detail page; verify slot rows match device state at capture time.
18. Confirm opening history/detail does not change active layout on server or device.

## Pass criteria

- Storefront recovers after planogram technician session.
- Periodic snapshots appear on Web under correct machine/layout.
- Transport retries do not create duplicate history rows for the same capture.
