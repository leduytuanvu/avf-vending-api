# AVF000195 — Kiểm tra đơn offline không lên web

Phiên log mẫu: 2026-10-02 23:40–23:44 (`machineId=01a0a7e5-3c68-7895-b526-bcb6504bccfb`).

## 1. DNS / API trên máy (ADB shell)

```bash
adb shell ping -c 1 machine-api.ldtv.dev
adb shell ping -c 1 mqtt.ldtv.dev
```

Nếu ping fail nhưng WiFi “có mạng”: sửa DNS/router hoặc captive portal trước khi kỳ vọng sync.

## 2. Logcat sau khi online ≥ 2 phút

Tìm (theo thứ tự):

| Tag / chuỗi | Ý nghĩa |
|-------------|---------|
| `COMMERCE_CONNECTIVITY_MODE_CHANGED ... ONLINE_HEALTHY` | App coi commerce online |
| `GRPC_BOOTSTRAP_RES ... OK` | Bootstrap API OK |
| `SYNC_RUN_STARTED businessOutboxPending=` | SyncEngine chạy, có hàng outbox |
| `SYNC_ANTI_HERD_BYPASS pendingBusinessOutbox=` | Bỏ trì anti-herd khi có đơn offline chờ sync |
| `OUTBOX_COMMERCE_PUSH_DEFERRED reason=commerce_grpc_unhealthy` | DNS/API chưa OK — outbox giữ local, không spam circuit breaker |
| `OUTBOX_SEQUENCE_REWIND_DELIVERED` | Rewind hàng DELIVERED local khi server cursor lùi — refill seq 1…N |
| `OUTBOX_SEQUENCE_GAP_WAIT` | Replay/drain dừng đúng head-of-line (chờ seq thấp hơn), không spam `expected 1 got 68` |
| `OUTBOX_SEQUENCE_HOLE` | Server cursor + min pending lệch >1 (seq 1..N đã mất local) — cần ops align cursor (mục 5) |
| `OUTBOX_RECONCILE_SKIP_CURSOR_LAG` | Reconcile PROCESSED nhưng chưa xóa row vì `seq > serverLast` — chờ push tăng cursor |
| `OUTBOX_SEQUENCE_MISMATCH` | Server báo lệch seq — client rewind + refresh cursor |
| `OFFLINE_SALE_REPLAY_ACCEPTED` | Server đã nhận đơn offline |
| `outbox_push_accepted` | Push outbox thành công |
| `OUTBOX_DRAIN_DEFERRED reason=sequence_hole` | APK mới: legacy drain bỏ qua khi hole (một dòng/sync run) |

**Cảnh báo:** `OFFLINE_SALE_OUTBOX_ENQUEUED` chỉ là **ghi local**, chưa lên server.

## 5. Sequence hole + cursor align

Workflow: **Production align AVF000195 offline sync cursor** (`production-align-avf000195-offline-sync-cursor.yml`)  
hoặc script: `scripts/ops/align_avf000195_offline_sync_cursor.sh` (xem `align_avf000195_offline_sync_cursor.sql`).

### Sự cố 2026-10-04 (log ~14:05)

| Mục | Giá trị |
|-----|---------|
| `machineId` | `01a0a7e5-3c68-7895-b526-bcb6504bccfb` |
| Đơn offline | `932704f6-cb43-461d-aef5-62d6cb74aaad` |
| Log | `OUTBOX_SEQUENCE_HOLE serverLast=0 minPending=4 expectedNext=1` |
| Align | `target_last_sequence = 3` (= minPending − 1), stream `offline` |

### Sự cố 2026-10-04 (log ~14:32–14:33)

| Mục | Giá trị |
|-----|---------|
| `machineId` | `01a0a7e5-3c68-7895-b526-bcb6504bccfb` (AVF000195) |
| Đơn offline | `cab88c38-a583-41b1-812e-488a75c9722f` |
| Log | `OUTBOX_SEQUENCE_HOLE serverLast=0 minPending=3 expectedNext=1`, `OUTBOX_DRAIN_DEFERRED reason=sequence_hole` |
| Align | `target_last_sequence = 2` (= minPending − 1) |

**GetSyncCursor trả `0` khi không có row** `machine_sync_cursors` — script cũ chỉ `UPDATE` sẽ **0 row** nếu chưa có cursor; dùng **UPSERT** trong `align_avf000195_offline_sync_cursor.sql` hoặc workflow với input `target_last_sequence`.

Workflow: **luôn** nhập `target_last_sequence = minPending − 1` từ dòng `OUTBOX_SEQUENCE_HOLE` mới nhất (input bắt buộc, không dùng default cũ).

Verify sau align: workflow **Production verify AVF000195 offline replay** hoặc `bash scripts/ops/verify_avf000195_offline_replay.sh` (read-only).

**Không** reset counter allocator local trên máy.

### Sự cố 2026-10-04 (log ~15:04–15:06)

| Mục | Giá trị |
|-----|---------|
| `machineId` | `01a0a7e5-3c68-7895-b526-bcb6504bccfb` (AVF000195) |
| Đơn offline | `ffc489f5-0bcd-45f3-971a-ce709f7c2056` (vend fail A1), `0a6984ee-7139-4d18-b943-9a843da0e6d3` (vend OK A2) |
| Log | `OUTBOX_SEQUENCE_HOLE serverLast=2 minPending=4 expectedNext=3`, `OUTBOX_DRAIN_DEFERRED reason=sequence_hole` |
| Align | `target_last_sequence = 3` (= minPending − 1) |

Logcat sau reconnect: không còn hole `serverLast=2` / `minPending=4`; tìm `OFFLINE_SALE_REPLAY_ACCEPTED` cho hai `orderId` trên.

### Sự cố 2026-10-04 (log ~15:45–15:46)

| Mục | Giá trị |
|-----|---------|
| `machineId` | `01a0a7e5-3c68-7895-b526-bcb6504bccfb` (AVF000195) |
| Đơn offline | `caea5197-7295-47f6-b9d8-7d1af2fc1207` (vend OK A3, 15k cash) |
| Log | `OUTBOX_SEQUENCE_HOLE serverLast=3 minPending=37 expectedNext=4`, `OUTBOX_SEQUENCE_GAP_WAIT expected=4 gotHead=37`, `OUTBOX_DRAIN_DEFERRED reason=sequence_hole` |
| Align | `target_last_sequence = 36` (= minPending − 1); GH Actions apply run verified `offline.last_sequence=36` |
| Verify | `verify_avf000195_offline_replay.sh` — `PRIMARY_ORDER=caea5197-…`, `MIN_CURSOR=36` (order row chờ kiosk drain sau align) |

Mất mạng ~15:45:40 (`UnknownHostException: api.ldtv.dev`). Đơn đã enqueue local; sau `ONLINE_HEALTHY` sync chạy nhưng hole chặn push tới khi align 36.

## 6. Forensics seq thiếu (1–3 và 4–36)

Dump `sync_queue` (business outbox) theo `sequence_no`, `status`, `entity_type`:

```bash
adb shell "run-as com.avf.vending.tcn sqlite3 databases/avf_vending.db \
  \"SELECT sequence_no, status, entity_type, entity_id FROM sync_queue WHERE sequence_no BETWEEN 1 AND 40 ORDER BY sequence_no;\""
```

(Release `com.avf.vending.tcn` thường `Package is not debuggable` — dùng log `OUTBOX_SEQUENCE_HOLE` / technician dump.)

| Kết quả thường gặp | Ý nghĩa |
|--------------------|---------|
| Không có row PENDING trong `expectedNext .. minPending-1` | Hole hợp lệ — align `last_sequence = minPending - 1` |
| Row seq thấp `DELIVERED` nhưng đã xóa khỏi pending | Seq đã push hoặc reconcile xóa; cursor server lùi sau align ops |
| `OUTBOX_SEQUENCE_GAP_WAIT expected=4 gotHead=37` (15:46) | Thiếu seq 4–36 local; align `target=36` |

**15:45 forensics (log):** `serverLast=3`, `minPending=37` ⇒ không còn pending 4–36; grep `OUTBOX_RECONCILE`, `OUTBOX_RECONCILE_SKIP_CURSOR_LAG`, `OUTBOX_SEQUENCE_REWIND_DELIVERED`.

## 7. Stuck `PROCESSING` (tách khỏi sequence hole)

| Dấu hiệu | Ý nghĩa |
|----------|---------|
| `OUTBOX_READY_DECISION ready=false`, `stuckCritical=16+` | Outbox kẹt `PROCESSING` / `semantic_evidence:PROCESSING` |
| `BILL_VEND_RESTORE_DEFERRED reason=readiness_gate` | Bill restore chờ readiness — không thay hole |
| `OUTBOX_DRAIN_DEFERRED reason=sequence_hole` | Ưu tiên align cursor theo `OUTBOX_SEQUENCE_HOLE` |

Recovery PROCESSING: runbook riêng (release claim / reconcile); không reset `sequence_no` allocator; không auto-skip seq thiếu trên client.
