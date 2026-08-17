# Runbook: Production setup máy AVF111111 (cài app lần đầu → bán hàng cash + QR live)

Runbook này mô tả **từng bước API** trên production `https://api.ldtv.dev` để có máy mới **TCN, 60 slot**, `machineCode` **AVF111111**, catalog + ảnh sản phẩm + planogram + tồn kho, và **activation code 6 chữ số** cho app cài lần đầu.

Tài liệu bám sát code trong repo:

- Upload ảnh: [`internal/httpserver/admin_media_http.go`](../../internal/httpserver/admin_media_http.go) — `POST /v1/admin/product-images`
- Layout 60 slot: [`scripts/e2e/examples/machine-layout-AVF111111-60slots.json`](../../scripts/e2e/examples/machine-layout-AVF111111-60slots.json)
- Apply topology/planogram/stock: [`scripts/e2e/setup-machine-sellable-layout-apply.sh`](../../scripts/e2e/setup-machine-sellable-layout-apply.sh)
- Upload ảnh hàng loạt: [`scripts/e2e/upload-avf111111-product-images.ps1`](../../scripts/e2e/upload-avf111111-product-images.ps1)
- Activation: [`docs/runbooks/machine-activation.md`](machine-activation.md)

---

## 0. Thông số mục tiêu

| Hạng mục | Giá trị |
|----------|---------|
| Base URL | `https://api.ldtv.dev` |
| Admin | `admin@avf.com` |
| Machine code | `AVF111111` |
| Hardware | TCN, cabinet A, lưới 10×6 = **60 slot** |
| Thanh toán | Production **live** QR (MoMo / ZaloPay / VietQR) + cash |
| SKU | `AVF111111-A1` … `AVF111111-F10` |
| Giá | 15.000 VND / slot (`price_minor: 15000`) |
| Ảnh | 4 URL, chia đều 15 slot/ảnh theo `slot_index` |

### Ảnh nguồn (tải về local trước khi upload)

| Nhóm slot (`slot_index`) | File local | URL |
|--------------------------|------------|-----|
| 1–15 | `phuc-long.png` (convert từ `.jpg`) | https://upload.urbox.vn/strapi/phuc_long_5_c188a69da5.jpg |
| 16–30 | `macchiato.webp` | https://spacet-release.s3.ap-southeast-1.amazonaws.com/img/blog/2023-11-06/macchiato-the-coffee-house-65486a865d22b5a617c3bfa0.webp |
| 31–45 | `tra-sua-koi.webp` | https://hunufa.vn/wp-content/uploads/2025/10/tra-sua-koi-mon-nao-ngon-nhat-7.webp |
| 46–60 | `tra-sua-dep.webp` | https://hunufa.vn/wp-content/uploads/2024/10/hinh-ly-tra-sua-dep.webp |

**Lưu ý:** JPEG gốc từ urbox.vn có thể bị API từ chối (`invalid image bytes`). Script upload tự convert sang PNG.

---

## 0.1 Biến môi trường (PowerShell)

```powershell
$BaseUrl = "https://api.ldtv.dev"
$AdminEmail = "admin@avf.com"
$AdminPassword = "1@Avfvietnam"   # không commit file chứa mật khẩu
$MachineCode = "AVF111111"
```

Mọi **write** (POST/PATCH/PUT) bắt buộc header:

```
Authorization: Bearer {accessToken}
Idempotency-Key: {uuid-or-stable-key}
Content-Type: application/json   # trừ multipart upload ảnh
```

---

## Luồng tổng thể

```
B1 version → B2 login → B3 planogram org → B4 site → B5/B6 machine
→ B7–B8 catalog (inactive) → B9–B10 ảnh → B11–B15 layout
→ B16 machine active → B17 activation code → App: B18 claim → B19 bootstrap (gRPC)
```

**Thứ tự quan trọng (production):**

1. Tạo sản phẩm **`active: false`** trước (active yêu cầu `primaryMediaId`).
2. Upload ảnh qua `POST /v1/admin/product-images` với `productId` + `isPrimary=true`.
3. `PATCH` sản phẩm `active: true`.
4. Set máy **`status: "active"`** (không chỉ `online`) trước `planograms/publish` — MQTT dispatch yêu cầu máy commandable.
5. Tạo activation code **sau** khi catalog/planogram xong.

---

## B1 — Kiểm tra API / version

**Request**

```http
GET /version
```

**PowerShell**

```powershell
Invoke-RestMethod -Uri "$BaseUrl/version"
```

**Response mẫu (rút gọn)**

```json
{
  "git_sha": "999b9e93",
  "payment_runtime": {
    "payment_mode": "live_psp",
    "cash_enabled": true,
    "qr_card_enabled": true
  }
}
```

**Lưu:** production examples mặc định `PAYMENT_ENV=live` với `COMMERCE_PAYMENT_PROVIDER=momo` và allowlist `momo,zalopay,vietqr`. Cash-only vẫn dùng được qua `apply_cash_only_payment_app_node_env.sh`.

---

## B2 — Admin login

**Request**

```http
POST /v1/auth/login
Content-Type: application/json

{"email":"admin@avf.com","password":"1@Avfvietnam"}
```

**Response mẫu**

```json
{
  "accountId": "019f16b1-da40-755b-a67b-c9427ae278ca",
  "email": "admin@avf.com",
  "roles": ["platform_admin", "admin"],
  "tokens": {
    "accessToken": "eyJ...",
    "accessExpiresAt": "2026-07-06T20:00:00Z",
    "refreshToken": "...",
    "tokenType": "Bearer"
  }
}
```

**Lưu biến:** `$Token = $resp.tokens.accessToken`

**Optional — B2b:** `GET /v1/auth/me` với Bearer token.

---

## B3 — Kiểm tra planogram org (bắt buộc)

Script layout cần ít nhất một planogram org (`GET /v1/admin/planograms`).

**Request**

```http
GET /v1/admin/planograms?limit=20
Authorization: Bearer {token}
```

**Response mẫu**

```json
{
  "items": [
    {
      "id": "cccccccc-cccc-cccc-cccc-000000000001",
      "name": "Default",
      "revision": 1,
      "status": "published"
    }
  ],
  "meta": { "limit": 20, "returned": 1, "totalCount": 1 }
}
```

**Lưu:** `planogramId`, `planogramRevision`. Nếu `items` rỗng → **blocker** (API hiện chỉ GET planogram org; cần seed DB / liên hệ ops).

---

## B4 — Site

**List (dùng site có sẵn nếu phù hợp)**

```http
GET /v1/admin/sites?limit=50
```

**Create (máy mới)**

```http
POST /v1/admin/sites
Idempotency-Key: {stable-idempotency-key}
Content-Type: application/json

{
  "name": "AVF111111 Pilot Site",
  "code": "AVF111111-SITE",
  "timezone": "Asia/Ho_Chi_Minh",
  "address": {}
}
```

**Response 200/201**

```json
{
  "id": "019f38cb-fad8-7a15-bcac-f145ba4f1eb0",
  "name": "AVF111111 Pilot Site",
  "code": "AVF111111-SITE",
  "status": "active"
}
```

**Lưu:** `siteId`

---

## B5 — Kiểm tra máy AVF111111 đã tồn tại

```http
GET /v1/admin/machines?limit=200
```

Tìm `code == "AVF111111"`. Nếu có → lưu `machineId`. Nếu `retired`/`compromised` → `POST .../enable` hoặc tạo máy mới.

---

## B6 — Tạo máy (nếu chưa có)

**Request**

```http
POST /v1/admin/machines
Idempotency-Key: {stable-idempotency-key}
Content-Type: application/json

{
  "siteId": "019f38cb-fad8-7a15-bcac-f145ba4f1eb0",
  "code": "AVF111111",
  "serialNumber": "SN-AVF111111",
  "name": "AVF111111 TCN 60-slot",
  "model": "TCN",
  "cabinetType": "ambient",
  "timezone": "Asia/Ho_Chi_Minh",
  "status": "draft"
}
```

**Response 201**

```json
{
  "id": "019f38cc-1509-7249-94a6-a99bfdb1f997",
  "code": "AVF111111",
  "status": "draft",
  "siteId": "019f38cb-fad8-7a15-bcac-f145ba4f1eb0"
}
```

**Lưu:** `machineId`. Cập nhật `machine_id` trong [`machine-layout-AVF111111-60slots.json`](../../scripts/e2e/examples/machine-layout-AVF111111-60slots.json).

---

## B7 — Category + Brand

**Category** — slug `avf111111-beverages`

```http
GET /v1/admin/categories?limit=200
POST /v1/admin/categories
Idempotency-Key: {stable-idempotency-key}

{"name":"AVF111111 Beverages","slug":"avf111111-beverages","parentId":null,"active":true}
```

**Brand** — slug `avf111111`

```http
POST /v1/admin/brands
Idempotency-Key: {stable-idempotency-key}

{"name":"AVF111111","slug":"avf111111","active":true}
```

**Production (2026-07-06):**

- `categoryId`: `019f38cc-441d-7a5a-8d7e-9a3b195a447b`
- `brandId`: `019f38cc-481b-7ec4-af9b-dec451d5511b`

---

## B8 — Tạo 60 sản phẩm (inactive trước)

Production **từ chối** `active: true` khi chưa có `primaryMediaId`:

```json
{"error":{"code":"invalid_argument","message":"catalogadmin: invalid argument: active products require primaryMediaId"}}
```

**Request (mỗi SKU, ví dụ A1)**

```http
POST /v1/admin/products
Idempotency-Key: {stable-idempotency-key}
Content-Type: application/json

{
  "name": "Phuc Long Tra sua A1",
  "sku": "AVF111111-A1",
  "description": "AVF111111 slot A1",
  "active": false,
  "categoryId": "019f38cc-441d-7a5a-8d7e-9a3b195a447b",
  "brandId": "019f38cc-481b-7ec4-af9b-dec451d5511b",
  "ageRestricted": false,
  "allergenCodes": []
}
```

**Response 200**

```json
{
  "id": "019f38cf-1677-7120-8778-d143ff14d3e9",
  "sku": "AVF111111-A1",
  "active": false,
  "status": "inactive"
}
```

Lặp cho 60 SKU (xem layout JSON). Hoặc để script layout tạo — **nhưng** script mặc định `active:true` sẽ fail; dùng loop inactive như trên, hoặc chạy layout sau khi đã có ảnh.

---

## B9 — Tải 4 ảnh về local

**PowerShell**

```powershell
$ImageDir = "tmp/avf111111-images"
New-Item -ItemType Directory -Force -Path $ImageDir
Invoke-WebRequest -Uri "https://upload.urbox.vn/strapi/phuc_long_5_c188a69da5.jpg" -OutFile "$ImageDir/phuc-long.jpg"
# ... 3 URL còn lại
# Convert JPEG → PNG (bắt buộc cho production)
Add-Type -AssemblyName System.Drawing
$img = [System.Drawing.Image]::FromFile("$ImageDir/phuc-long.jpg")
$img.Save("$ImageDir/phuc-long.png", [System.Drawing.Imaging.ImageFormat]::Png)
$img.Dispose()
```

**Git Bash**

```bash
mkdir -p tmp/avf111111-images
curl.exe -L -o tmp/avf111111-images/phuc-long.jpg "https://upload.urbox.vn/strapi/phuc_long_5_c188a69da5.jpg"
# ...
```

---

## B10 — Upload ảnh + gắn primary (`POST /v1/admin/product-images`)

**Endpoint:** `POST /v1/admin/product-images` (alias: `/v1/admin/media/product-images`)

**Multipart form**

| Field | Bắt buộc | Mô tả |
|-------|----------|--------|
| `file` | yes | File ảnh |
| `productId` | khuyến nghị | UUID sản phẩm |
| `isPrimary` | `true` | Gắn làm ảnh chính |
| `purpose` | optional | `product_image` |

**curl (Windows — dùng forward slash cho path)**

```bash
curl.exe -X POST "https://api.ldtv.dev/v1/admin/product-images" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: avf111111-img-{productId}" \
  -F "file=@D:/path/tmp/avf111111-images/phuc-long.png;type=image/png" \
  -F "purpose=product_image" \
  -F "productId=019f38cf-1677-7120-8778-d143ff14d3e9" \
  -F "isPrimary=true"
```

**Response 201**

```json
{
  "mediaId": "019f38d1-c573-7e0c-83ba-a968a8aca523",
  "provider": "cloudinary",
  "displayUrl": "https://res.cloudinary.com/.../phuc-long.png",
  "thumbnailUrl": "https://res.cloudinary.com/.../w_300,...",
  "productId": "019f38cf-1677-7120-8778-d143ff14d3e9",
  "attached": true,
  "isPrimary": true,
  "status": "ready"
}
```

**Automation (60 lần):**

```powershell
.\scripts\e2e\upload-avf111111-product-images.ps1 `
  -AdminEmail "admin@avf.com" `
  -AdminPassword "1@Avfvietnam" `
  -ArtifactDir "reports/e2e/avf111111-setup/{timestamp}/image-upload"
```

Script tự: download → convert phuc-long → upload theo `slot_index` → `PATCH active:true`.

**Verify một sản phẩm**

```http
GET /v1/admin/products/{productId}
```

Phải có `primaryMediaId` và `media.primary.variants[]`.

**Lỗi thường gặp**

| HTTP | Code | Xử lý |
|------|------|--------|
| 400 | `invalid_image_file` | Convert JPEG → PNG; kiểm tra file không phải HTML |
| 503 | `capability_not_configured` | Cloudinary chưa bật trên API |
| 400 | `missing_idempotency_key` | Thêm header Idempotency-Key |

---

## B11 — Kích hoạt sản phẩm (nếu chưa active sau upload)

```http
PATCH /v1/admin/products/{productId}
Idempotency-Key: {stable-idempotency-key}{productId}
Content-Type: application/json

{"active": true}
```

---

## B12 — Operator session

```http
POST /v1/admin/machines/{machineId}/operator-sessions/start
Content-Type: application/json

{"force_admin_takeover": true, "auth_method": "oidc"}
```

**Response 200**

```json
{
  "session": {
    "id": "019f38d8-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "status": "ACTIVE"
  }
}
```

**Lưu:** `operatorSessionId = session.id`

---

## B13 — Topology 60 slot TCN

```http
PUT /v1/admin/machines/{machineId}/topology
Content-Type: application/json

{
  "operator_session_id": "{operatorSessionId}",
  "cabinets": [{
    "code": "A",
    "title": "Cabinet A",
    "sortOrder": 1,
    "metadata": {
      "board_protocol": "Tcn",
      "bill_protocol": "Ict_BC_V1",
      "cash_topology": "DIRECT_BILL",
      "transport_type": "RS485"
    }
  }],
  "layouts": [{
    "cabinetCode": "A",
    "layoutKey": "grid-10x6",
    "revision": 1,
    "layoutSpec": {"rows": 6, "cols": 10},
    "status": "published"
  }]
}
```

**Response:** `204 No Content`

Body đầy đủ lấy từ [`machine-layout-AVF111111-60slots.json`](../../scripts/e2e/examples/machine-layout-AVF111111-60slots.json).

---

## B14 — Planogram draft

```http
PUT /v1/admin/machines/{machineId}/planograms/draft
Content-Type: application/json

{
  "operator_session_id": "{operatorSessionId}",
  "planogramId": "cccccccc-cccc-cccc-cccc-000000000001",
  "planogramRevision": 1,
  "syncLegacyReadModel": true,
  "items": [
    {
      "cabinetCode": "A",
      "slotCode": "A1",
      "productId": "019f38cf-1677-7120-8778-d143ff14d3e9",
      "maxQuantity": 5,
      "priceMinor": 15000,
      "layoutKey": "grid-10x6",
      "layoutRevision": 1,
      "legacySlotIndex": 1,
      "metadata": {}
    }
  ]
}
```

60 phần tử `items` (một item/slot sellable). Script layout build JSON tự động.

**Response:** `204`

---

## B15 — Planogram publish

**Trước publish:** `PATCH /v1/admin/machines/{machineId}` → `{"status":"active"}`  
(Nếu để `online`, publish có thể trả `500 dispatch_failed`: *machine is not commandable*)

```http
POST /v1/admin/machines/{machineId}/planograms/publish
Idempotency-Key: {stable-idempotency-key}
Content-Type: application/json

{ ... cùng body với draft ... }
```

**Response 200 (mẫu)**

```json
{
  "desiredConfigVersion": 3,
  "planogramId": "cccccccc-cccc-cccc-cccc-000000000001",
  "planogramRevision": 1,
  "command": {
    "commandId": "...",
    "sequence": 1,
    "dispatchState": "dispatched"
  }
}
```

---

## B16 — Stock adjustment

```http
GET /v1/admin/machines/{machineId}/slots
```

```http
POST /v1/admin/machines/{machineId}/stock-adjustments
Idempotency-Key: {stable-idempotency-key}
Content-Type: application/json

{
  "operator_session_id": "{operatorSessionId}",
  "reason": "restock",
  "items": [{
    "cabinetCode": "A",
    "slotCode": "A1",
    "slotIndex": 1,
    "productId": "...",
    "planogramId": "cccccccc-cccc-cccc-cccc-000000000001",
    "quantityBefore": 0,
    "quantityAfter": 5
  }]
}
```

**Automation một lệnh (B12–B16):**

```powershell
# Cập nhật machine_id trong layout JSON, rồi:
$env:E2E_ALLOW_WRITES = "true"
$env:E2E_PRODUCTION_WRITE_CONFIRMATION = "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION"
bash scripts/e2e/setup-machine-sellable-layout-apply.sh reports/.../machine-layout-AVF111111-60slots.json
```

**Verify admin**

```http
GET /v1/admin/machines/{machineId}/slots
```

Kỳ vọng: **60 slot**, mỗi slot có `productId`, `currentQuantity` > 0.

---

## B17 — Activation code (giao cho app — tạo sau cùng)

```http
POST /v1/admin/machine-codes/AVF111111/activation-codes
Idempotency-Key: {stable-idempotency-key}
Content-Type: application/json

{
  "expiresInMinutes": 1440,
  "maxUses": 1,
  "notes": "first install AVF111111"
}
```

**Response 200**

```json
{
  "activationCode": "290711",
  "activationCodeId": "019f38de-e713-7c22-bc2e-e77806fa78a6",
  "expiresAt": "2026-07-07T19:00:00Z",
  "maxUses": 1
}
```

**Quan trọng:** Plaintext `activationCode` **chỉ trả về lúc create**. Giao qua kênh bảo mật cho kỹ thuật viên. **Không commit** mã này vào git.

Production setup **2026-07-06** đã tạo mã cho app; xem file local `reports/e2e/avf111111-setup/20260706T185808Z/raw/B17-activation-code.json` (gitignored).

---

## B18 — App claim activation (public, không Bearer admin)

```http
POST /v1/setup/activation-codes/claim
Content-Type: application/json

{
  "activationCode": "290711",
  "deviceFingerprint": {
    "androidId": "android-device-id",
    "serialNumber": "SN-DEVICE-001",
    "manufacturer": "SUNMI",
    "model": "K2",
    "packageName": "com.avf.vending",
    "versionName": "1.0.0",
    "versionCode": 1
  }
}
```

**Response 200**

```json
{
  "machineId": "019f38cc-1509-7249-94a6-a99bfdb1f997",
  "machineCode": "AVF111111",
  "machineToken": "eyJ...",
  "bootstrapUrl": "/v1/setup/machines/019f38cc-1509-7249-94a6-a99bfdb1f997/bootstrap",
  "bootstrapRequired": true,
  "mqtt": {
    "brokerUrl": "tls://mqtt.ldtv.dev:8883",
    "topicPrefix": "avf/devices",
    "topicLayout": "enterprise"
  },
  "mqttUsername": "019f38cc-1509-7249-94a6-a99bfdb1f997",
  "mqttPassword": "..."
}
```

**Lỗi:** `400 activation_invalid` — mã sai/hết hạn/đã dùng.

---

## B19 — Bootstrap (machine JWT)

Production thường **tắt legacy HTTP bootstrap**; app dùng **gRPC** `MachineBootstrapService/GetBootstrap`.

Nếu HTTP bật:

```http
GET /v1/setup/machines/{machineId}/bootstrap
Authorization: Bearer {machineToken}
```

Kỳ vọng: `topology.cabinets[0].slots` (60), `catalog.products` (60).

---

## B20 — Sale catalog + ảnh (runtime)

```http
GET /v1/machines/{machineId}/sale-catalog?include_images=true
Authorization: Bearer {machineToken}
```

Hoặc gRPC `MachineCatalogService.GetSaleCatalog`. Mỗi item có `image.displayUrl`, `cacheKey`, `version` cho offline cache.

---

## B21 — Xác nhận bán (optional)

Cash: gRPC `ConfirmCashPayment` hoặc `POST /v1/commerce/cash-checkout` với machine token trên slot thử (A1).
QR: gRPC `CreatePaymentSession` (provider `momo` / `zalopay` / `vietqr`) rồi poll `GetOrderStatus` tới `payment_state=captured`.

---

## Phụ lục A — Thứ tự automation khuyến nghị

```powershell
# 1. Login + tạo site/machine (B4–B6) — thủ công hoặc script
# 2. Cập nhật machine_id trong layout JSON
# 3. Tạo 60 product inactive (B8)
# 4. Upload ảnh + activate (B9–B11)
.\scripts\e2e\upload-avf111111-product-images.ps1 -AdminEmail ... -AdminPassword ...

# 5. PATCH machine status active
Invoke-RestMethod -Method Patch -Uri "$BaseUrl/v1/admin/machines/$MachineId" `
  -Headers @{ Authorization="Bearer $Token"; "Idempotency-Key"="avf111111-active" } `
  -ContentType "application/json" -Body '{"status":"active"}'

# 6. Layout apply (topology + planogram + stock)
bash scripts/e2e/setup-machine-sellable-layout-apply.sh path/to/machine-layout-AVF111111-60slots.json

# 7. Activation code (B17)
```

---

## Phụ lục B — Checklist GO trước khi cài app

- [ ] `GET /version` → `payment_mode=live_psp` và `qr_card_enabled=true` (hoặc `cash_only` nếu cố ý pilot cash)
- [ ] Máy `AVF111111` tồn tại, `status=active`
- [ ] `GET .../slots` → 60 slot, có `productId`, `currentQuantity` > 0
- [ ] 60 SKU active + `primaryMediaId`
- [ ] Activation code 6 chữ số còn hiệu lực, `maxUses` chưa hết
- [ ] MQTT broker reachable từ thiết bị (`tls://mqtt.ldtv.dev:8883`)

---

## Phụ lục C — Production evidence (2026-07-06)

Setup đã chạy trên `https://api.ldtv.dev`. Artifact: `reports/e2e/avf111111-setup/20260706T185808Z/`

| Entity | ID |
|--------|-----|
| Site | `019f38cb-fad8-7a15-bcac-f145ba4f1eb0` |
| Machine | `019f38cc-1509-7249-94a6-a99bfdb1f997` |
| Machine code | `AVF111111` |
| Category | `019f38cc-441d-7a5a-8d7e-9a3b195a447b` |
| Brand | `019f38cc-481b-7ec4-af9b-dec451d5511b` |
| Org planogram | `cccccccc-cccc-cccc-cccc-000000000001` rev `1` |
| Slots + stock | 60/60 có product + quantity |
| Layout apply | `SETUP_READINESS=PASS` (layout-apply3) |

Activation code cho app lần đầu: lưu trong `raw/B17-activation-code.json` (local only).

Claim verification (mã riêng, không dùng mã app): `raw/B18-claim-verify.json` — xác nhận MQTT + machineToken OK.

---

## Phụ lục D — Rollback (nếu setup sai)

```http
DELETE /v1/admin/machine-codes/AVF111111/activation-codes/{activationCodeId}
POST /v1/admin/machines/{machineId}/retire
```

Không xóa SQL trực tiếp trừ quy trình incident đã phê duyệt.
