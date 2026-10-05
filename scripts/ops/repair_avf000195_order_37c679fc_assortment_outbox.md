# AVF000195 — Khôi phục đơn `37c679fc-49b6-4671-99ea-c6e9dbfdf7cc` (assortment + outbox seq 102)

Phiên log: 2026-10-05 ~16:26–16:28. `machineId=01a0a7e5-3c68-7895-b526-bcb6504bccfb`.

| Sự kiện | Chi tiết |
|---------|----------|
| Đơn local | `37c679fc-49b6-4671-99ea-c6e9dbfdf7cc` — partial vend A4 OK, A5 `TCN_NO_DROP`, refund 5k |
| OFFLINE_SALE push | `product is not in the machine's published assortment` (A4 `01a089d0-…`, A5 `01a089cf-…`) |
| Outbox stall | `CASH` seq **102**, key `cash_movement:…:230004` → `invalid cash movements payload` |
| Hậu quả | `OUTBOX_STREAM_STALLED` `commerceBlocked=true`; VEND → `commerce: not found` |

## A1 — Đồng bộ published assortment (admin / Postgres)

1. Trên admin: xác nhận slot **A4** / **A5** gán đúng product, planogram active.
2. Trên Postgres (read-only trước): chạy `repair_avf000195_order_37c679fc_assortment_outbox.sql` phần verify.
3. Publish/sync assortment primary cho máy (API `SyncAssortmentFromCurrentSlotConfigs` — fleet bootstrap hoặc admin publish flow tương đương).
4. Kiểm tra: gRPC `CreateQuote` 2 dòng A4+A5 **không** còn assortment error (máy log `GRPC_CREATE_QUOTE` OK).

## A2 — Giải phóng outbox seq=102 (máy)

1. Technician DB / sync screen: tìm `BusinessOutbox` `type=CASH`, `sequence_no=102`, `idempotency_key` chứa `230004`.
2. Export `payload_json`. Nếu `events[].device_event_id` / `occurred_at_millis` (snake_case) và push fail `invalid cash movements payload`:
   - **Sau deploy app** có remap: retry sync một lần; payload canonical phải có `deviceEventId`, `occurredAt`.
   - Nếu vẫn fail: **dead-letter** row (không xóa cả queue) để floor > 102 — app mới classify `InvalidArgument` → dead-letter ngay.
3. Trigger sync (`ImmediateSyncWorker` / restart app sync). Quan sát `OFFLINE_SALE_REPLAY_ACCEPTED` cho `offline-sale:37c679fc-…`.

## A3 — Xác nhận admin

- [https://admin.ldtv.dev/orders](https://admin.ldtv.dev/orders): order `37c679fc-…` với lines partial + cash/refund khớp payload offline.
- Outbox: không còn `OUTBOX_STREAM_STALLED` `floor=102` kéo dài.

## Code đã ship cùng incident

- API: `offline_sale` dùng `resolveCheckoutSaleLine` + pricing snapshot mirror.
- App sync: `SyncFailureClassifier.classifyFromMessage` cho `rpc error: code = InvalidArgument`.
- App mapper: remap `report_cash_movements` dù payload đã có `context`.
- App checkout: chặn gRPC local-shell checkout online khi quote prefetch báo assortment stale.
