# Dữ liệu seed dev (PostgreSQL) và test Swagger

Tài liệu này tóm tắt dữ liệu **cố định** do migration [`migrations/00003_seed_dev.sql`](../../migrations/00003_seed_dev.sql) chèn vào DB sau `make migrate-up` / goose. UUID trùng với [`internal/testfixtures/ids.go`](../../internal/testfixtures/ids.go).

---

## Tổ chức và địa điểm

| Thực thể | UUID | Ghi chú |
|----------|------|---------|
| Organization “Local Dev Org”, slug `local-dev` | `11111111-1111-1111-1111-111111111111` | `status`: active |
| Region “HQ”, code `hq` | `22222222-2222-2222-2222-222222222222` | |
| Site “Main DC” | `33333333-3333-3333-3333-333333333333` | `address`: `{"city": "DevCity"}` |

---

## Máy và phần cứng

| Thực thể | UUID | Ghi chú |
|----------|------|---------|
| Machine hardware profile “Generic VMC” | `44444444-4444-4444-4444-444444444444` | `spec`: `{"slots": 60}` |
| Machine “Dev Machine 1” | `55555555-5555-5555-5555-555555555555` | serial `SN-DEV-001`, `status`: online |

Dùng **`55555555-5555-5555-5555-555555555555`** làm `machineId` trên Swagger cho các route `/v1/machines/{machineId}/...`.

---

## Kỹ thuật viên và gán máy

| Thực thể | UUID | Ghi chú |
|----------|------|---------|
| Technician “Pat Technician” | `66666666-6666-6666-6666-666666666666` | email `pat@example.com` (bảng `technicians`, **không** phải tài khoản `/v1/auth/login`) |
| Gán máy | — | `technician_machine_assignments`: Pat → Dev Machine 1, role `maintainer` |

---

## Sản phẩm và giá

| Thực thể | UUID | Ghi chú |
|----------|------|---------|
| Product “Cola 330ml”, SKU `SKU-COLA` | `aaaaaaaa-aaaa-aaaa-aaaa-000000000001` | |
| Product “Still water 500ml”, SKU `SKU-WATER` | `aaaaaaaa-aaaa-aaaa-aaaa-000000000002` | |
| Price book “Default USD” | `bbbbbbbb-bbbb-bbbb-bbbb-000000000001` | currency USD, default |
| Giá | — | Cola 150 minor, Water 120 minor (USD cents) |

---

## Planogram, slot, tồn máy

| Thực thể | UUID | Ghi chú |
|----------|------|---------|
| Planogram “Default Planogram” | `cccccccc-cccc-cccc-cccc-000000000001` | revision 1, published |
| Slots | — | index 0 → Cola, index 1 → Water, `max_quantity` 10 |
| `machine_slot_state` | — | máy dev: slot 0 qty 5 giá 150; slot 1 qty 8 giá 120 |
| `machine_shadow` | — | desired có `planogram_id` trên; reported `temperature_c`: 4 |

---

## OTA (mẫu)

| Thực thể | UUID | Ghi chú |
|----------|------|---------|
| Artifact firmware `1.0.0` | `dddddddd-dddd-dddd-dddd-000000000001` | `storage_key`: `dev/firmware/1.0.0.bin` |
| Campaign “Pilot rollout” | `eeeeeeee-eeee-eeee-eeee-000000000001` | draft, strategy rolling |
| Target | — | máy dev, state `pending` |

---

## Đăng nhập Swagger (`POST /v1/auth/login`)

Migration seed **không** tạo dòng trong `platform_auth_accounts`. Đăng nhập cần **email + mật khẩu** khớp bảng đó (mật khẩu lưu dạng bcrypt). Chi tiết luồng: [`docs/vi/huong-dan-api-tu-dang-nhap-den-ban-hang.md`](../vi/huong-dan-api-tu-dang-nhap-den-ban-hang.md).

### Vì sao bị 401 sau khi chèn SQL?

1. **Chuỗi “bcrypt” phải là hash thật** (60 ký tự, dạng `$2a$` / `$2b$` / `$2y$`). Các chuỗi kiểu `...8K8K8K...` hoặc hash “giả lập” **không hợp lệ** — Go sẽ không verify được và trả sai mật khẩu (401).
2. **API và DB phải cùng một nơi**: bạn gọi `https://api.ldtv.dev` thì tài khoản phải tồn tại trong **PostgreSQL mà server đó đang dùng**. Chỉ chạy `INSERT` trên máy local / DBeaver trỏ nhầm instance vẫn sẽ 401 trên host public.
3. **Tổ chức `11111111-...` phải có trong DB đó**: nếu môi trường deploy **chưa** chạy migration seed `00003`, `INSERT` có thể lỗi FK hoặc bạn đang dùng sai `organization_id` so với dữ liệu thật. Kiểm tra: `SELECT id, slug FROM organizations;`

### Cặp dev mẫu (chỉ local / lab — không dùng production)

| Trường | Giá trị |
|--------|---------|
| Email | `admin@local.test` |
| Password (plaintext) | `password123` |
| Bcrypt (đã generate bằng Python `bcrypt`, cost 10, khớp `password123`) | `$2b$10$0oWtyzdsMgQ.BGN/KTludOb0XdHh/Q0i2XLzEj9WVCBs1y0M07ne2` |

Go `golang.org/x/crypto/bcrypt` chấp nhận tiền tố `$2b$` giống `$2a$`.

**SQL** (nếu đã có dòng trùng org + email, xóa trước hoặc đổi email):

```sql
INSERT INTO platform_auth_accounts (organization_id, email, password_hash, roles, status)
VALUES (
  '11111111-1111-1111-1111-111111111111',
  'admin@local.test',
  '$2b$10$0oWtyzdsMgQ.BGN/KTludOb0XdHh/Q0i2XLzEj9WVCBs1y0M07ne2',
  ARRAY['org_admin']::text[],
  'active'
);
```

**Body login** (JSON), ví dụ Swagger:

```json
{
  "organizationId": "11111111-1111-1111-1111-111111111111",
  "email": "admin@local.test",
  "password": "password123"
}
```

### Tạo hash mới (mật khẩu khác)

- Trong repo: `go run ./tools/dev-bcrypt <plaintext>` (cần Go chạy được trên máy bạn).
- Hoặc Python: `py -3 -m pip install bcrypt` rồi  
  `py -3 -c "import bcrypt; print(bcrypt.hashpw(b'mat_khau_cua_ban', bcrypt.gensalt(10)).decode())"`

Role hợp lệ gồm `platform_admin`, `org_admin`, `org_member` (xem `internal/platform/auth/principal.go`).

---

## Gỡ seed (goose Down)

Migration `00003` có `Down`: xóa theo thứ tự FK (OTA → shadow → slots → … → organization). Chỉ chạy khi bạn chủ động rollback migration tương ứng.
