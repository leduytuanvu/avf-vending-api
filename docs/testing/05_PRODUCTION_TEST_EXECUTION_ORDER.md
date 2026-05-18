# Thứ tự thực thi kiểm thử Production — Postman REST (VI)

Tài liệu này là **gợi ý thứ tự**, không phải kết luận PASS production. **Luôn import** `AVF_REST_365_FULL.postman_collection.json` và `AVF_PRODUCTION.postman_environment.json` (đủ biến gate: `allow_destructive`, `canaryMode`, `readiness`).

## Gate trước khi chạy write

Trước khi **bật** bất kỳ request ghi nào (trừ `POST /v1/auth/login` và `POST /v1/auth/refresh`), đặt **một trong**:

- `allow_destructive=true` — kiểm thử ghi chủ đích trên môi trường được phép;
- `canaryMode=true` — luồng ghi canary có kiểm soát;
- `readiness=true` — readiness / canary checklist (thường kèm assertion 2xx).

Mặc định cả ba là `false`. Nếu thiếu, pre-request của collection sẽ **`throw`** với `[GATED]` (kể cả khi request đã enabled trong Postman).

## Vai trò và phạm vi công ty (JWT)

- **platform_admin**: các endpoint admin single-company không yêu cầu tham số truy vấn lọc theo công ty ngoài JWT.
- **admin**: company được suy ra từ JWT; giữ các query tùy chọn trong Postman **disabled** trừ khi OpenAPI bắt buộc.

## Biến tự capture sau response (collection test script)

| Bước | Khi HTTP 2xx và đã gate (một trong ba flag `true`) |
|------|------------------------------------------------------|
| Login | `accessToken`, `refreshToken`. |
| GET `/v1/auth/me` | principal fields. |
| POST site / product / machine (admin) … | Ưu tiên field `_id`; fallback `id` → `siteId` / `productId` / `machineId` tùy path. |
| Commerce / payment / command / operator … | `orderId`, `paymentSessionId`, `commandId`, `operatorSessionId`, v.v. theo payload. |

Script **không** ghi đè env bằng giá trị rỗng; **không** log JWT/token.

## Headers

- `X-Request-ID` / `X-Correlation-ID`: mặc định mỗi request lấy UUID mới; có thể **pin** bằng env `requestId` / `correlationId` nếu cần.
- `Idempotency-Key` và alias `X-Idempotency-Key`: có thể pin bằng env `idempotencyKey`; nếu để trống, pre-request sinh giá trị an toàn kiểu `avf-postman-…`.

## Luồng đề xuất (REST)

### A. Smoke read-only

1. `GET /health/live`
2. `GET /health/ready`
3. `GET /version`
4. `GET /swagger/doc.json` — có thể **404** khi OpenAPI JSON tắt tại prod.
5. `GET /metrics` — test trong collection **chấp nhận 200/401/404**.

### B. Auth

6. `POST /v1/auth/login` (enabled sẵn) — nhập `adminEmail`, `adminPassword` trong env.
7. `GET /v1/auth/me` — xác nhận JWT và org.

### C. Admin / RBAC

8. (Optional) Companies / invitations / RBAC trong folder **02** — chỉ sau khi bật gate.

### D. Site → Product → Machine

9. **Create/list site** — folder **03**; kiểm tra `siteId` được set sau create.
10. **Create product/catalog** — folder **05**; kiểm tra `productId`.
11. **Create / activate machine** — folder **04**; `activationCode`, `machineToken`, `machineId`.

### E. Commerce / payment / vend

12. Folder **08** — cần `machineId|canary_machine_id`, PSP/sandbox đúng.

### F. Telemetry / inventory / commands / operator

13. Folder **07**, **11**, **12**, **10** — ưu tiên máy canary; không publish telemetry giả vào fleet thật.

### G. Negative & idempotency

14. Folder **15** — token sai, thiếu header, idempotency replay.

### H. gRPC / MQTT

- gRPC và MQTT không nằm trong collection REST — xem `grpc/README_GRPC_POSTMAN_IMPORT_VI.md` và `mqtt/README_MQTT_POSTMAN_IMPORT_VI.md`.

## Giới hạn tuyên bố

- Asset chứng minh **import + đầy đủ operation** khớp OpenAPI/proto/topic trong repo — **không** thay cho xác nhận production PASS.
