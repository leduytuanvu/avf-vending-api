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
| `OUTBOX_SEQUENCE_GAP_WAIT expected=N gotHead=M` với **M &lt; N** | **Stale local lag** (cursor server đã ≥ N−1 nhưng queue còn pending seq thấp) — APK mới: `OUTBOX_SEQUENCE_STALE_PRUNED` / `OUTBOX_SEQUENCE_REALIGNED` |
| `OUTBOX_SEQUENCE_HOLE` | Server cursor + min pending lệch >1 (seq 1..N đã mất local) — cần ops align cursor (mục 5) |
| `OUTBOX_SEQUENCE_STALE_PRUNED` / `OUTBOX_SEQUENCE_REALIGNED` | App đã dọn hàng seq ≤ serverLast và/hoặc gán lại seq commerce từ `serverLast+1` |
| `OUTBOX_SEQUENCE_GAP_FAILED_RETRY_NOW` | FAILED trong gap (seq kẹt head-of-line, backoff) được đẩy retry ngay trong reconcile |
| `OUTBOX_SEQUENCE_PHANTOM_GAP_NOOP` | Gap phantom không compact (đã align hoặc không còn pending shift) — xem `blockingInGap` / `rowsInGap` |
| `OUTBOX_POST_REINSTALL_READY` | Cursor + head pending khớp `serverLast+1` — an toàn test offline sau cài lại |
| `OUTBOX_STALE_BELOW_CURSOR_UNRESOLVED` | Commerce stale dưới cursor nhưng server vẫn PENDING — **không** xóa local; kiểm tra cursor align |
| `OUTBOX_RECONCILE_SKIP_CURSOR_LAG` | Reconcile PROCESSED nhưng chưa xóa row vì `seq > serverLast` — chờ push tăng cursor |
| `OUTBOX_SEQUENCE_MISMATCH` | Server báo lệch seq — client rewind + refresh cursor |
| `OFFLINE_SALE_REPLAY_ACCEPTED` | Server đã nhận đơn offline |
| `outbox_push_accepted` | Push outbox thành công |

**Cảnh báo:** `OFFLINE_SALE_OUTBOX_ENQUEUED` (hoặc legacy `OFFLINE_SALE_REPLAY_START`) chỉ là **ghi local**, chưa lên server.

## 3. Nếu vẫn không thấy trên web

- Xác nhận máy đúng site/machine trên web (AVF000195).
- Technician / DB: pending `sync_queue` với `entity_type` `offline_sale`, `vend`.
- Gửi log từ reconnect đến khi thấy `SYNC_RUN_STARTED` hoặc `BusinessOutbox`.

## 4. Anti-herd (trước bản sửa app)

UUID máy này có thể trì sync reconnect ~95s. Bản app mới **bypass anti-herd** khi `businessOutboxPending > 0`.

## 5. Sequence hole + cursor align (một lần trên Postgres)

Khi log có `OUTBOX_SEQUENCE_HOLE` hoặc `OUTBOX_SEQUENCE_GAP_WAIT expected=1 gotHead=51` (server `last_sequence=0`, pending bắt đầu từ 51):

1. Trên kiosk (technician): dump `sync_queue` theo `sequence_no`, `status`, `entity_type` — xác nhận không còn row seq 1..50 (PENDING/DELIVERED).
2. Xác nhận không cần backfill đơn 1..50 lên admin.
3. **Prod Postgres** (machine `01a0a7e5-3c68-7895-b526-bcb6504bccfb` / AVF000195):
   - Backup `machine_sync_cursors` cho `machine_id` này.
   - Set `last_sequence = MIN(sequence_no) - 1` từ hàng commerce PENDING (ví dụ pending min 51 → set `50`).
4. Cài APK có head-of-line drain + reconcile guard; reconnect; tìm `outbox_push_accepted` / `OFFLINE_SALE_REPLAY_ACCEPTED`.

**Không** reset counter allocator local trên máy (giữ `sequence_no` monotonic).

### Sự cố 2026-10-04 (mất mạng ~14:05, AVF000195)

| Mục | Giá trị |
|-----|---------|
| `machineId` | `01a0a7e5-3c68-7895-b526-bcb6504bccfb` |
| Đơn offline (vend OK local) | `932704f6-cb43-461d-aef5-62d6cb74aaad` |
| Log chốt | `OUTBOX_SEQUENCE_HOLE serverLast=0 minPending=4 expectedNext=1` |
| Cursor align | `last_sequence = 3` (= `minPending - 1`) |

1. Trên kiosk: xác nhận **không** còn outbox/sync row với `sequence_no` 1, 2, 3.
2. Postgres prod — **backup trước**, rồi align (điều chỉnh tên bảng/stream nếu schema khác):

```sql
-- Backup
SELECT * FROM machine_sync_cursors
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb';

-- Chỉ khi admin không có đơn commerce seq 1–3 và device không còn row 1–3
UPDATE machine_sync_cursors
SET last_sequence = 3, updated_at = NOW()
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
  AND last_sequence < 3;
```

3. Reconnect kiosk; trong logcat tìm `OFFLINE_SALE_REPLAY_ACCEPTED` / `outbox_push_accepted` cho order `932704f6-…`.
4. APK mới (tùy chọn): một dòng `OUTBOX_DRAIN_DEFERRED reason=sequence_hole` mỗi sync run thay vì hàng trăm `GAP_WAIT source=drain`.

### Sự cố 2026-10-04 (log ~15:04–15:06, AVF000195)

| Mục | Giá trị |
|-----|---------|
| Đơn offline | `ffc489f5-…` (A1 fail), `0a6984ee-…` (A2 OK) |
| Log | `OUTBOX_SEQUENCE_HOLE serverLast=2 minPending=4 expectedNext=3` |
| Align | `target_last_sequence = 3` — workflow **Production align AVF000195 offline sync cursor** |
| Verify | Workflow **Production verify AVF000195 offline replay** hoặc `scripts/ops/verify_avf000195_offline_replay.sh` |

**Ops:** `target_last_sequence` = **minPending − 1** từ log mới nhất; không dùng giá trị incident cũ (ví dụ `2` sau khi `minPending=4`).

### Sự cố 2026-10-04 (log ~15:45–15:46, AVF000195)

| Mục | Giá trị |
|-----|---------|
| Đơn offline | `caea5197-7295-47f6-b9d8-7d1af2fc1207` (A3, 15k) |
| Log | `OUTBOX_SEQUENCE_HOLE serverLast=3 minPending=37`, `OUTBOX_DRAIN_DEFERRED reason=sequence_hole` |
| Align | `target_last_sequence = 36` (sau align trước `last_sequence=3`) |
| Verify | `PRIMARY_ORDER=caea5197-…`, `MIN_CURSOR=36` — workflow **Production verify AVF000195 offline replay** |

### Sự cố 2026-10-04 (log ~16:14–16:16, AVF000195)

| Mục | Giá trị |
|-----|---------|
| Đơn offline (cash 10k, A1+A2, vend OK local) | `54a327c2-3ecd-4014-b90c-121fed479cd6` |
| Log | `OUTBOX_SEQUENCE_GAP_WAIT expected=37` (cursor align 36 **đúng** — không phải hole 3→37) |
| Blocker | `OUTBOX_COMMERCE_PUSH_FAILED type=OFFLINE_SALE` `reason=offline event insert failed` @ seq **37** |
| Forensics SQL | `avf-vending-api/scripts/ops/verify_avf000195_offline_replay.sql` |
| Remediation (sau SQL) | `avf-vending-api/scripts/ops/repair_avf000195_seq37_incident.sql` — **không** bump `last_sequence` lên 37 nếu insert vẫn REJECTED |

Sau deploy API (logging PG + migration `00038`):

1. Prod logs ~16:15:56 +07: grep `MACHINE_OFFLINE_INSERT_ERROR` / `PushOfflineEvents` — lấy `sqlstate`, `constraint_name`.
2. Chạy repair script theo case A–D; reconnect kiosk.
3. Logcat: `OFFLINE_SALE_REPLAY_ACCEPTED` / `outbox_push_accepted` cho `54a327c2-…`; admin order 10k, 2 line.
4. Cursor prod: `last_sequence` ≥ 37.

**Forensics prod (sau deploy API `d0ebde7b`, 2026-10-04 ~16:43 +07):**

| Kiểm tra | Kết quả |
|----------|---------|
| `machine_sync_cursors.offline.last_sequence` | **36** (đúng — chờ push seq 37) |
| `machine_offline_events` seq 37 / `54a327c2` | **0 row** (insert lần trước không ghi ledger) |
| `orders` `54a327c2-…` | **chưa có** — kiosk cần drain outbox khi online |
| Repair DB (case B–D) | **Không** cần cho đến khi có row seq 37 kẹt hoặc order trùng |

Workflow: **Production verify AVF000195 offline replay** — `min_cursor=36`, `primary_order=54a327c2-…`, `require_orders=true` khi đã sync.

Trên kiosk: WiFi OK → logcat `SYNC_RUN_STARTED` / `OFFLINE_SALE_REPLAY_ACCEPTED` hoặc `OUTBOX_COMMERCE_PUSH_FAILED` (reason mới có `sqlstate`).

### Sự cố 2026-10-04 (log ~19:39–19:40, AVF000195)

| Mục | Giá trị |
|-----|---------|
| Đơn offline (cash 10k, A2+A3, vend OK local) | `55a21114-7f0b-4067-a99a-85a8f5916292` |
| Log | `OUTBOX_SEQUENCE_GAP_WAIT expected=37 gotHead=3..22` — **không** `OUTBOX_SEQUENCE_HOLE` (min pending &lt; 37) |
| Nguyên nhân | Cursor prod `last_sequence=36` đúng; local còn pending seq 1–22 + allocator chưa sàn → replay kẹt trước seq 37 |
| APK fix | Reconcile: prune stale ≤36, `OUTBOX_SEQUENCE_REALIGNED floor=37`, sàn `outbox_stream_state` |
| Verify | `PRIMARY_ORDER=55a21114-…`, `MIN_CURSOR=36` — `OFFLINE_SALE_REPLAY_ACCEPTED` sau reconnect |

**Unblock tạm (trước APK):** xác nhận admin không thiếu đơn seq 1–36 → xóa/markProcessed row `sync_queue` với `sequence_no <= 36` (non-commerce an toàn hơn) + `outbox_stream_state.nextSequence >= 37` cho stream `offline`.

### Sự cố 2026-10-04 (log ~20:47–20:48, AVF000195)

| Mục | Giá trị |
|-----|---------|
| Đơn offline (cash 10k, A9+A10, vend OK local) | `a1d6c15d-e9da-4b23-836e-086214a36bc5` |
| Đơn online ngay trước mất mạng | `7a8ff96e-4779-4460-b190-8274c869a115` (cursor server **37**) |
| Log | `OUTBOX_SEQUENCE_HOLE serverLast=37 minPending=58 expectedNext=38`, `OUTBOX_DRAIN_DEFERRED reason=sequence_hole` |
| Không có | `OUTBOX_SEQUENCE_PHANTOM_GAP_CLOSED` (gap **38–57 occupied** — còn row DELIVERED/PROCESSING/DEAD, không empty) |
| Nguyên nhân | Head-of-line: replay chỉ thấy pending từ **58**; dải 38–57 chiếm chỗ trong `sync_queue` nhưng không trong `listPendingForReplay` |
| APK fix | `reconcileOccupiedGapAboveCursor` + `OUTBOX_SEQUENCE_PHANTOM_GAP_SKIPPED` / heal DELIVERED+PROCESSING; phantom compact dùng **blocking** count |
| Ops unblock (forensics) | Dump `sync_queue` seq 38–57; nếu server đã PROCESSED hết → mark processed local. **Chỉ** `UPDATE machine_sync_cursors SET last_sequence=57` khi xác nhận server không thiếu commerce 38–57 |
| Verify | `PRIMARY_ORDER=a1d6c15d-…`, `MIN_CURSOR=37` — `OFFLINE_SALE_REPLAY_ACCEPTED` / `outbox_push_accepted` sau reconnect |

### Sự cố 2026-10-05 (log ~00:45–00:46, AVF000195)

| Mục | Giá trị |
|-----|---------|
| Đơn offline (cash 10k, B4+B6, vend OK local) | `42f63de0-2bc1-46c5-8797-a00111f5538b` |
| Head outbox kẹt (đơn cũ) | `86c726f8-7313-46eb-8e90-d7244b52b605` @ **seq 38** |
| Log | `OUTBOX_COMMERCE_PUSH_FAILED` `22P02: invalid input syntax for type json`, `OUTBOX_REPLAY_STALLED_AT_HEAD seq=38`, `OUTBOX_SEQUENCE_GAP_WAIT expected=38 gotHead=39..48` |
| API fix | `2d20ca0a` jsonb bind + self-hosted deploy **app-node A + B** (`2e0cf901` workflow) |
| Prod forensics (sau deploy) | `offline.last_sequence=37`; chưa có row seq 38 / orders — **chờ kiosk retry** |
| Verify | Workflow **Production verify AVF000195 offline replay** — `MIN_CURSOR=37`, `PRIMARY_ORDER=42f63de0-…`, `require_orders=true` sau reconnect |
| Logcat máy | `OFFLINE_SALE_REPLAY_ACCEPTED` / `outbox_push_accepted` cho `86c726f8` rồi `42f63de0` |

### Sự cố 2026-10-04 (log ~21:16–21:17, AVF000195)

| Mục | Giá trị |
|-----|---------|
| Đơn offline (cash 10k, B3+B5, vend OK local) | `ec10d814-e1af-4d17-a4c2-f2ee36fad956` |
| Đơn online ngay trước mất mạng | cursor server **37** (~20:46) |
| Log | `OUTBOX_SEQUENCE_GAP_WAIT expected=38 gotHead=39`, `OUTBOX_SEQUENCE_REWIND_DELIVERED serverLast=37 count=7`, `businessOutboxPending=26` |
| Nguyên nhân | Head-of-line: slot **38** occupied (FAILED backoff / DELIVERED / PROCESSING) nhưng head pending **39**; replay + drain dừng |
| APK fix | `OUTBOX_SEQUENCE_GAP_FAILED_RETRY_NOW`, phantom `NOOP`/`SKIPPED` rõ, replay reconcile khi gap ≤2, boot-gate reconcile trước enqueue gRPC |
| Verify | Grep `OFFLINE_SALE_REPLAY_ACCEPTED` / `outbox_push_accepted` cho `ec10d814-…`; `OUTBOX_POST_REINSTALL_READY` sau reconnect |

### Gỡ app / cài lại (test hygiene)

| Cách | Outbox local | Đơn chưa sync trước gỡ |
|------|----------------|-------------------------|
| **`adb install -r` (cài đè)** | Giữ `sync_queue` | Vẫn đẩy khi online |
| **Gỡ + cài mới** | DB trống | **Mất** — server không có payload |
| **Sau cài mới** | Đơn **mới** | Đẩy được nếu có mạng + token → `OUTBOX_POST_REINSTALL_READY` / `REALIGNED` trước test offline dài |

1. Trước gỡ (bắt buộc nếu còn đơn quan trọng): online, `businessOutboxPending=0`, grep `OFFLINE_SALE_REPLAY_ACCEPTED`.
2. Sau cài lại: đợi `GRPC_BOOTSTRAP` + `SYNC_RUN_STARTED` + `OUTBOX_POST_REINSTALL_READY` (hoặc `PHANTOM_GAP_CLOSED` / `REALIGNED`).
3. Không chỉnh `last_sequence` prod sau mỗi vòng test — chỉ theo mục 5 khi `OUTBOX_SEQUENCE_HOLE`.

### Sự cố 2026-10-04 (log ~20:00–20:01, AVF000195)

| Mục | Giá trị |
|-----|---------|
| Đơn offline (cash 10k, A1+A4, vend OK local) | `5e06a32b-5274-4b9d-9150-becc41d2ebc9` |
| Log | `OUTBOX_SEQUENCE_HOLE serverLast=36 minPending=38 expectedNext=37`, `OUTBOX_DRAIN_DEFERRED reason=sequence_hole` |
| Nguyên nhân | **Phantom gap:** seq 37 đã bị allocator tiêu thụ / row local mất (sau sự cố `54a327c2`); head pending = 38 → khác pattern stale-lag 19:39 (`minPending < 37`) |
| Ops unblock | `target_last_sequence = minPending - 1` → **37** (align cursor prod, script mục 5) |
| APK fix | `OUTBOX_SEQUENCE_PHANTOM_GAP_CLOSED` — compact pending xuống `floor=37` khi không còn row trong `[expectedNext, minPending-1]` |
| Verify | `PRIMARY_ORDER=5e06a32b-…`, `MIN_CURSOR=36` — `OFFLINE_SALE_REPLAY_ACCEPTED` sau reconnect (APK mới hoặc sau align 37) |

## 6. Điều tra seq thiếu trên device (forensics)

Mẫu: server `last_sequence=0`, pending bắt đầu từ 4+ — seq 1–3 không còn trong `business_outbox` / `sync_queue` (PENDING, DELIVERED, FAILED).

Trên backup DB hoặc technician dump:

```sql
-- Thay tên bảng theo schema app (business_outbox / sync_queue)
SELECT sequence_no, status, event_type, entity_id, updated_at
FROM business_outbox
WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'
ORDER BY sequence_no;
```

Logcat lịch sử (grep): `OUTBOX_RECONCILE`, `markProcessed`, `OUTBOX_SEQUENCE_REWIND_DELIVERED`, `OUTBOX_RECONCILE_SKIP_CURSOR_LAG`, clear data / migration technician.

**Giả thuyết thường gặp:** row DELIVERED/PROCESSED reconcile xóa local trong khi server cursor chưa tăng; hoặc reset partial DB không kèm cursor align. Guard `OUTBOX_RECONCILE_SKIP_CURSOR_LAG` ngăn xóa khi `seq > serverLast` — nếu seq 1–3 đã bị xóa trước guard thì chỉ ops align cursor (mục 5) mới unblock.

**Phòng tái diễn:** sau align cursor, theo dõi `OUTBOX_SEQUENCE_HOLE`; không reset allocator seq trên máy; khi đổi máy/restore DB luôn đối chiếu `machine_sync_cursors` với `MIN(pending sequence_no)`.

**15:46 hole 4–36:** log `OUTBOX_SEQUENCE_GAP_WAIT expected=4 gotHead=37` — align `36`; ADB `run-as` trên release có thể `not debuggable` (dùng log + prod verify).

## 7. Stuck `PROCESSING` (tách khỏi sequence hole)

`OUTBOX_READY_DECISION ready=false` + `stuckCritical` và `BILL_VEND_RESTORE_DEFERRED reason=readiness_gate` **không** thay `OUTBOX_DRAIN_DEFERRED reason=sequence_hole`. Ưu tiên align cursor; recovery PROCESSING là runbook riêng, không auto-skip seq.

## 8. Prod app-node B + verify GitHub Actions

1. **Redis trên B:** chạy workflow `Production bootstrap REDIS_URL secret from app node A` (`confirm=BOOTSTRAP_PRODUCTION_REDIS_URL`), rồi `Production repair app node B` — B nhận `REDIS_URL` qua `SYNC_REDIS_URL`.
2. **Repair B:** `confirm_repair=REPAIR_APP_NODE_B`; bật `reset_app_node_b_caddy_tls` nếu Caddy ACME/DNS hỏng.
3. **Verify DB (không cần đơn trên web):** `Production verify AVF000195 offline replay` — `min_cursor=37`, `require_orders=false`.
4. **Sau kiosk replay:** cùng workflow với `require_orders=true`, `primary_order=42f63de0-2bc1-46c5-8797-a00111f5538b` (hoặc `86c726f8-…`).
