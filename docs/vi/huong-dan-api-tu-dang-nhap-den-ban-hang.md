# Hướng dẫn gọi API: từ đăng nhập đến cài máy và bán hàng

Tài liệu này mô tả **thứ tự các bước** gọi REST API của AVF Vending API (`/v1/...`), phù hợp với mã nguồn hiện tại. Chi tiết schema, ví dụ request/response và mã lỗi xem thêm file OpenAPI đã generate: [`docs/swagger/swagger.json`](../swagger/swagger.json). Nếu server bật Swagger UI, có thể thử trực tiếp trên trình duyệt.

---

## 1. Quy ước chung

### 1.1. Base URL

Thay `https://<host>` bằng địa chỉ API thực tế (ví dụ `https://api.example.com`).

### 1.2. Header

| Header | Khi nào cần |
|--------|-------------|
| `Content-Type: application/json` | Hầu hết request có body JSON |
| `Authorization: Bearer <accessToken>` | Các route sau khi đăng nhập (trừ `POST /v1/auth/login` và `POST /v1/auth/refresh`) |
| `Idempotency-Key` hoặc `X-Idempotency-Key` | **Bắt buộc** cho các thao tác ghi được ghi chú trong OpenAPI (commerce tạo đơn, thanh toán, vend, điều chỉnh tồn, publish planogram, v.v.) — dùng cùng một giá trị khi client retry an toàn |
| `X-Request-ID` / `X-Correlation-ID` | Tùy chọn; server có thể echo lại để truy vết |

### 1.3. Lỗi JSON chuẩn

Nhiều handler trả về dạng:

```json
{"error":{"code":"...","message":"...","details":{},"requestId":"..."}}
```

Một số tính năng chưa cấu hình đủ biến môi trường có thể trả **503** với `error.code` kiểu `capability_not_configured` (ví dụ commerce persistence, MQTT dispatch, v.v.).

### 1.4. Vai trò và phạm vi tổ chức

- JWT chứa **organization** (tổ chức) và **roles** (ví dụ `platform_admin`, `org_admin`, `org_member`).
- **`platform_admin`**: khi gọi API theo tenant (máy, catalog admin, báo cáo, …) thường phải thêm query **`organization_id=<uuid>`** để chọn tổ chức (xem mô tả từng path trong OpenAPI).
- **`org_admin` / `org_member`**: thường bị giới hạn theo organization trong token; không cần (hoặc không dùng) `organization_id` giống platform admin — cụ thể từng route xem Swagger.

---

## 2. Điều kiện tiên quyết

Trước khi đi theo luồng dưới đây, cần có:

- Một **tổ chức** (`organizationId`) hợp lệ.
- Tài khoản người dùng (**email / mật khẩu**) đã được gán vào tổ chức đó trong hệ thống.
- Ít nhất một **máy** (`machineId`) và dữ liệu catalog/planogram phù hợp với môi trường triển khai (việc tạo máy/tài khoản trong DB có thể do quy trình nội bộ hoặc công cụ khác — không nhất thiết qua các route public trong tài liệu này).

---

## 3. Bước 1 — Đăng nhập lần đầu (tài khoản nền tảng)

### 3.1. `POST /v1/auth/login`

**Không** cần header `Authorization`.

Body (JSON):

| Trường | Kiểu | Mô tả |
|--------|------|--------|
| `organizationId` | UUID | ID tổ chức |
| `email` | string | Email đăng nhập |
| `password` | string | Mật khẩu |

Response (rút gọn): `accessToken`, `refreshToken`, thời hạn, `accountId`, `organizationId`, `roles`, …

**Lưu `accessToken`** (và `refreshToken` nếu client cần gia hạn phiên).

### 3.2. (Tuỳ chọn) `GET /v1/auth/me`

Header: `Authorization: Bearer <accessToken>`

Dùng để xác nhận `accountId`, `organizationId`, `roles` sau khi đăng nhập.

### 3.3. (Tuỳ chọn) `POST /v1/auth/refresh`

Body: `refreshToken` — nhận cặp token mới khi access token hết hạn.

### 3.4. `POST /v1/auth/logout`

Header: Bearer. Body có thể có `refreshToken` hoặc `revokeAll` — xem schema trong OpenAPI.

---

## 4. Bước 2 — (Tuỳ chọn) Tìm và chọn máy

Cần quyền **admin** (thường `org_admin` hoặc `platform_admin`).

### 4.1. `GET /v1/admin/machines`

- **platform_admin**: thêm query **`organization_id=<uuid>`** (bắt buộc để chọn tenant).
- Có thể lọc `site_id`, `machine_id`, `status`, cửa sổ thời gian `from`/`to` trên `updated_at`, phân trang `limit`/`offset`.

Từ kết quả, lấy **`machineId`** (UUID) của máy cần cấu hình hoặc bán hàng.

Các route admin khác (kỹ thuật viên, gán máy, lệnh, OTA, …) cũng nằm dưới `/v1/admin/...` — xem OpenAPI nếu cần.

---

## 5. Bước 3 — Phiên vận hành tại máy (operator session)

Nhiều thao tác **cài đặt máy** (topology, planogram, đồng bộ, điều chỉnh tồn) yêu cầu **`operator_session_id`** trỏ tới phiên **ACTIVE** trên đúng máy đó. Vì vậy thường **mở phiên** trước khi gọi các API admin tương ứng.

### 5.1. `POST /v1/machines/{machineId}/operator-sessions/login`

- Header: `Authorization: Bearer <accessToken>` (cùng user đã đăng nhập; quyền truy cập máy theo JWT và assignment).
- **Danh tính kỹ thuật viên lấy từ JWT**, không gửi trong body dưới dạng `technician_id` tùy tiện.

Body (tất cả tùy chọn trừ khi cần):

| Trường | Ghi chú |
|--------|---------|
| `auth_method` | Để trống server có thể mặc định |
| `expires_at` | Thời hạn phiên (nếu có) |
| `client_metadata` | JSON metadata phía client |
| `force_admin_takeover` | Chỉ org/platform admin khi cần chiếm phiên |

Response: object `session` gồm ít nhất **`id`** (UUID phiên). **Dùng `session.id` làm `operator_session_id`** trong các request admin sau (chuỗi UUID).

### 5.2. Heartbeat (khuyến nghị khi phiên dài)

`POST /v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat`

Giữ phiên không bị coi là idle/stale (chi tiết timeout xem tài liệu vận hành).

### 5.3. `GET /v1/machines/{machineId}/operator-sessions/current`

Xem phiên hiện tại trên máy.

### 5.4. `POST /v1/machines/{machineId}/operator-sessions/logout`

Kết thúc phiên; body có thể gồm `session_id`, `ended_reason`, …

---

## 6. Bước 4 — Cài đặt máy (topology, planogram, đồng bộ, tồn)

Tất cả các path dưới đây nằm trong nhóm **admin** và có prefix:

`/v1/admin/machines/{machineId}/...`

Header: `Authorization: Bearer <accessToken>`

- **platform_admin**: hầu hết request cần query **`organization_id=<uuid>`** của máy (chọn tenant).
- Các thao tác **ghi** quan trọng cần **`Idempotency-Key`** (giá trị unique mỗi hành động logic).

### 6.1. Đọc snapshot cài đặt (ứng dụng kỹ thuật / màn hình setup)

`GET /v1/setup/machines/{machineId}/bootstrap`

- Trả về thông tin máy, topology (cabinet/slot), catalog sản phẩm gắn assortment (theo quyền `RequireMachineURLAccess`).
- Không nằm dưới `/admin` nhưng vẫn cần Bearer và quyền đọc máy.

### 6.2. Topology (tủ và layout ô)

`PUT /v1/admin/machines/{machineId}/topology`

- Body yêu cầu **`operator_session_id`** (phiên ACTIVE) và cấu trúc `cabinets` / `layouts` — chi tiết xem OpenAPI và ví dụ trong `swagger.json`.

### 6.3. Planogram nháp và publish

1. `PUT /v1/admin/machines/{machineId}/planograms/draft` — lưu bản nháp slot config; có thể đồng bộ read model legacy tùy cờ.
2. `POST /v1/admin/machines/{machineId}/planograms/publish` — áp dụng bản hiện tại, tăng phiên bản cấu hình, enqueue lệnh thiết bị (MQTT) khi hạ tầng đủ; **bắt buộc `Idempotency-Key`** và **`operator_session_id`**.

### 6.4. Đồng bộ thiết lập máy

`POST /v1/admin/machines/{machineId}/sync`

- Dispatch lệnh kiểu **machine_setup_sync**; cần **`operator_session_id`**, **`Idempotency-Key`**, body tùy payload (xem OpenAPI).

### 6.5. Kiểm tra tồn theo ô (slot)

`GET /v1/admin/machines/{machineId}/slots`

- Gộp trạng thái tồn / giá / ngưỡng cảnh báo theo từng ô (UI nhập hàng / kiểm kê).

### 6.6. Điều chỉnh tồn (nhập hàng, kiểm kê, …)

`POST /v1/admin/machines/{machineId}/stock-adjustments`

- **`operator_session_id`** bắt buộc (phiên ACTIVE).
- **`Idempotency-Key`** bắt buộc (replay an toàn).
- `reason`: một trong `restock`, `cycle_count`, `manual_adjustment`, `machine_reconcile`.
- `items[]`: gồm `planogramId`, `slotIndex`, `quantityBefore`, `quantityAfter`, và các trường tùy chọn như `cabinetCode`, `slotCode`, `productId`.

### 6.7. (Tuỳ chọn) Đọc inventory khác

`GET /v1/admin/machines/{machineId}/inventory` — xem OpenAPI.

---

## 7. Bước 5 — Bán hàng hằng ngày (commerce)

Các route nằm dưới **`/v1/commerce/...`**. JWT phải có **phạm vi organization** (non-platform user thường đã có từ token; platform admin cần cơ chế scope phù hợp — xem hành vi `RequireOrganizationScope` và tài liệu OpenAPI).

### 7.1. Tạo đơn

`POST /v1/commerce/orders`

- **`Idempotency-Key`** bắt buộc.
- Body gồm `machine_id`, `product_id`, `slot_index`, tiền tệ, `subtotal_minor`, `tax_minor`, `total_minor`, … (đúng schema OpenAPI).

Response thường có `order_id`, `vend_session_id`, trạng thái đơn / vend.

### 7.2. Thanh toán

**Tiền mặt (nếu bật outbox commerce đủ cấu hình):**

`POST /v1/commerce/cash-checkout`

- Cùng ý nghĩa tạo đơn + ghi nhận thanh toán **captured** + mark paid — **Idempotency-Key** bắt buộc.
- Nếu thiếu cấu hình outbox topic/event type, API có thể trả **503** `capability_not_configured`.

**Tích hợp cổng thanh toán / ví:**

`POST /v1/commerce/orders/{orderId}/payment-session` — cần outbox được cấu hình; xem body `provider`, `amountMinor`, v.v.

Webhook (nếu dùng):

`POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks`

### 7.3. Theo dõi đơn

- `GET /v1/commerce/orders/{orderId}` — query `slot_index` nếu cần.
- `GET /v1/commerce/orders/{orderId}/reconciliation` — snapshot đối soát.

**Liệt kê** đơn hàng / thanh toán theo tenant (phạm vi organization trong JWT) dùng các path **ở cấp `/v1`**, không có tiền tố `commerce`:

- `GET /v1/orders` — danh sách đơn (pagination / filter theo OpenAPI).
- `GET /v1/payments` — danh sách thanh toán.

### 7.4. Chu trình vend (sau khi thanh toán phù hợp trạng thái đơn)

1. `POST /v1/commerce/orders/{orderId}/vend/start` — body có `slot_index`; **Idempotency-Key** bắt buộc.
2. Kết quả vật lý:
   - Thành công: `POST /v1/commerce/orders/{orderId}/vend/success`
   - Thất bại: `POST /v1/commerce/orders/{orderId}/vend/failure`

Chi tiết mã **409** `illegal_transition` hoặc thanh toán chưa settled xem mô tả trong OpenAPI.

---

## 8. Bước 6 — (Tuỳ vai trò) Thiết bị qua HTTP bridge

Dành cho firmware / gateway gọi thay MQTT trong một số tình huống.

| Method | Path | Ghi chú ngắn |
|--------|------|----------------|
| `POST` | `/v1/device/machines/{machineId}/commands/poll` | Lấy hàng đợi lệnh từ xa (HTTP fallback) |
| `POST` | `/v1/device/machines/{machineId}/vend-results` | Báo cáo kết quả vend (`outcome`: success/failed, …) |

Cả hai yêu cầu Bearer và quyền truy cập máy phù hợp (org/platform admin theo code hiện tại). **`Idempotency-Key`** áp dụng cho `vend-results` theo contract trong OpenAPI.

---

## 9. Phụ lục — Thứ tự endpoint gợi ý (tóm tắt)

| Thứ tự | Mục đích | Method | Path |
|--------|----------|--------|------|
| 1 | Đăng nhập | POST | `/v1/auth/login` |
| 2 | Xác nhận profile | GET | `/v1/auth/me` |
| 3 | (Admin) Liệt kê máy | GET | `/v1/admin/machines` |
| 4 | Mở phiên vận hành | POST | `/v1/machines/{machineId}/operator-sessions/login` |
| 5 | (Tuỳ chọn) Bootstrap UI | GET | `/v1/setup/machines/{machineId}/bootstrap` |
| 6 | Cấu hình topology | PUT | `/v1/admin/machines/{machineId}/topology` |
| 7 | Planogram nháp | PUT | `/v1/admin/machines/{machineId}/planograms/draft` |
| 8 | Publish planogram | POST | `/v1/admin/machines/{machineId}/planograms/publish` |
| 9 | Đồng bộ máy | POST | `/v1/admin/machines/{machineId}/sync` |
| 10 | Xem tồn ô | GET | `/v1/admin/machines/{machineId}/slots` |
| 11 | Nhập hàng / kiểm kê | POST | `/v1/admin/machines/{machineId}/stock-adjustments` |
| 12 | Tạo đơn | POST | `/v1/commerce/orders` |
| 13 | Thanh toán (cash hoặc payment-session) | POST | `/v1/commerce/cash-checkout` hoặc `.../payment-session` |
| 14 | Vend | POST | `.../vend/start` → `.../vend/success` hoặc `.../vend/failure` |
| 15 | Đóng phiên vận hành | POST | `/v1/machines/{machineId}/operator-sessions/logout` |

---

## 10. Tài liệu tham chiếu trong repo

- OpenAPI: [`docs/swagger/swagger.json`](../swagger/swagger.json)
- Đăng ký route HTTP: [`internal/httpserver/server.go`](../../internal/httpserver/server.go)
- Auth: [`internal/httpserver/auth_http.go`](../../internal/httpserver/auth_http.go)
- Operator session: [`internal/httpserver/operator_http.go`](../../internal/httpserver/operator_http.go)
- Admin inventory / máy: [`internal/httpserver/admin_inventory_http.go`](../../internal/httpserver/admin_inventory_http.go)
- Commerce: [`internal/httpserver/commerce_http.go`](../../internal/httpserver/commerce_http.go)
- Thiết bị: [`internal/httpserver/device_http.go`](../../internal/httpserver/device_http.go)

Nếu hành vi thực tế khác môi trường (thiếu commerce, thiếu MQTT, v.v.), luôn đọc `error.code` và `details` trong response JSON.
