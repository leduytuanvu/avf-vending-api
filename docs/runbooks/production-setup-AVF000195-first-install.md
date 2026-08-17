# Runbook: Production setup máy AVF000195 (cài app lần đầu → bán hàng cash)

Thiết lập máy **TCN, 60 slot**, `machineCode` **AVF000195** trên `https://api.ldtv.dev`. Ảnh sản phẩm **tái sử dụng** `primaryMediaId` từ catalog **AVF111111** (Cloudinary đã có trên server).

**Automation:** [`scripts/e2e/setup-production-machine-60slot.ps1`](../../scripts/e2e/setup-production-machine-60slot.ps1)

```powershell
$env:E2E_ALLOW_WRITES = "true"
$env:E2E_PRODUCTION_WRITE_CONFIRMATION = "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION"
.\scripts\e2e\setup-production-machine-60slot.ps1 `
  -MachineCode AVF000195 `
  -ReuseMediaFromPrefix AVF111111 `
  -AdminEmail admin@avf.com `
  -AdminPassword '***'
```

Layout: [`scripts/e2e/examples/machine-layout-AVF000195-60slots.json`](../../scripts/e2e/examples/machine-layout-AVF000195-60slots.json)

---

## B1 — Version

`GET /version` — kỳ vọng `payment_runtime.payment_mode = cash_only`.

## B2 — Login

`POST /v1/auth/login` → lưu `tokens.accessToken`.

## B3 — Planogram org

`GET /v1/admin/planograms?limit=20` — cần ít nhất 1 item `status=published`.

## B4 — Site

`POST /v1/admin/sites` body `{ "name": "AVF000195 Pilot Site", "code": "AVF000195-SITE", "timezone": "Asia/Ho_Chi_Minh", "address": {} }`

## B5/B6 — Machine

`POST /v1/admin/machines`:

```json
{
  "siteId": "{siteId}",
  "code": "AVF000195",
  "serialNumber": "SN-AVF000195",
  "name": "AVF000195 TCN 60-slot",
  "model": "TCN",
  "cabinetType": "ambient",
  "timezone": "Asia/Ho_Chi_Minh",
  "status": "draft"
}
```

Sau đó `PATCH` → `{ "status": "active" }` (**bắt buộc** trước planogram publish / MQTT dispatch).

Sau khi publish planogram + stock xong, `PATCH` → `{ "status": "online" }` (**bắt buộc** trước cài app — inventory gRPC yêu cầu `online` hoặc `offline`).

## B7 — Category + Brand

- Category slug: `avf000195-beverages`
- Brand slug: `avf000195`

## B8 — 60 sản phẩm (reuse media)

**Không upload lại ảnh** nếu AVF111111 còn trên server. Lấy `primaryMediaId` từ:

| slot_index AVF000195 | Reference AVF111111 |
|----------------------|---------------------|
| 1–15 | AVF111111-A1 |
| 16–30 | AVF111111-B6 |
| 31–45 | AVF111111-C1 |
| 46–60 | AVF111111-D1 |

```json
POST /v1/admin/products
{
  "sku": "AVF000195-A1",
  "name": "Phuc Long Tra sua A1",
  "active": false,
  "primaryMediaId": "{mediaIdFromAVF111111-A1}",
  "categoryId": "...",
  "brandId": "..."
}
```

Rồi `PATCH { "active": true }`.

## B9–B10 — Ảnh (fallback)

Chỉ khi AVF111111 không còn: `POST /v1/admin/product-images` multipart (xem runbook AVF111111).

## B11–B15 — Layout (topology + planogram + stock)

Chạy `setup-machine-sellable-layout-apply.sh` với layout JSON đã gán `machine_id`.

Verify: `GET /v1/admin/machines/{machineId}/slots` → 60 slot, có `productId`, `currentQuantity` > 0.

## B17 — Activation code

```http
POST /v1/admin/machine-codes/AVF000195/activation-codes
{"expiresInMinutes":1440,"maxUses":1,"notes":"first install AVF000195"}
```

Plaintext 6 chữ số **chỉ trả về lúc create** — lưu artifact local, không commit.

## B18 — App claim

```http
POST /v1/setup/activation-codes/claim
{
  "activationCode": "XXXXXX",
  "deviceFingerprint": {
    "androidId": "...",
    "serialNumber": "SN-DEVICE-001",
    "manufacturer": "SUNMI",
    "model": "K2",
    "packageName": "com.avf.vending",
    "versionName": "1.0.0",
    "versionCode": 1
  }
}
```

Nhận `machineToken`, `mqtt`, `machineCode: AVF000195`.

## B19–B20 — Bootstrap / sale catalog

Production app dùng **gRPC** (`MachineBootstrapService`, `MachineCatalogService`). Legacy HTTP bootstrap có thể 404.

## Checklist GO

- [ ] Máy `AVF000195` status **`online`** (không để `active` sau khi setup xong)
- [ ] 60 slot có product + stock
- [ ] 60 SKU active + `primaryMediaId`
- [ ] Activation code còn hiệu lực, chưa claim

## Troubleshooting — app `CATALOG_EMPTY` / `CATALOG_SYNC_FAIL`

Triệu chứng trên app (`com.avf.vending.tcn`):

- `PERMISSION_DENIED: machine not operational for inventory`
- `liveSellReady=false`, blocker `CATALOG_EMPTY`
- Log có thể vẫn hiện `LOCAL_CATALOG_COUNT=60` (cache bootstrap) nhưng inventory sync thất bại

**Nguyên nhân:** `machines.status` đang là `active` (hoặc trạng thái khác ngoài `online`/`offline`). Gate inventory gRPC (`GetInventorySnapshot`) chỉ cho phép `online` hoặc `offline` — xem [`internal/grpcserver/machine_gate.go`](../../internal/grpcserver/machine_gate.go).

**Sửa:**

```http
PATCH /v1/admin/machines/{machineId}
Idempotency-Key: fix-machine-online-{date}

{"status": "online"}
```

Sau PATCH: đợi app retry catalog sync (~30s) hoặc restart app. Kỳ vọng `INVENTORY_SYNC` thành công, `CATALOG_EMPTY` biến mất.

**Lifecycle tóm tắt:**

| Status | Dùng khi |
|--------|----------|
| `active` | Planogram publish / MQTT command dispatch (tạm thời) |
| `online` | Runtime app: catalog sync, inventory, bán hàng |

Khi cần gửi MQTT command mới sau khi máy đã `online`, PATCH tạm `active` → dispatch → PATCH lại `online` (theo [`tests/e2e/production/lib/mqtt_handlers.sh`](../../tests/e2e/production/lib/mqtt_handlers.sh)).

## Production evidence (2026-07-31)

| Entity | ID |
|--------|-----|
| Site | `019fb731-6490-7f32-9ed6-c1c5530e7e44` |
| Machine | `019fb731-6eec-725c-9179-6201a99c09f7` |
| Machine code | `AVF000195` (status **`online`** — sửa 2026-07-31 từ `active`) |
| Category | `019fb731-7658-7e68-9a36-2573cfaebc79` |
| Brand | `019fb731-7b09-7531-8380-e7191bb62a51` |
| Org planogram | `019fb6d2-7e2d-7818-898d-c43e43f0fd45` rev `1` |
| Media reused | `019fb70b-85ad-73c7-9fa4-d61273049a31` (product `coca` on server) |
| Slots + stock | 60/60 |

Activation code: `reports/e2e/avf000195-setup/20260731T080128Z/raw/B17-activation-code.json` (local, không commit).

**Lưu ý:** Catalog `AVF111111` không còn trên production lúc setup; script fallback quét media sẵn có trên server.
