# AVF FULL 100 — Import và chạy (Tiếng Việt)

## Import Postman

- Collection: `AVF_FULL_100.postman_collection.json`
- Environment: `AVF_FULL_100.postman_environment.json`

## Login platform admin

- Điền `platformAdminEmail` / `platformAdminPassword` và dùng body login theo swagger (`adminEmail`/`password` trong JSON — map thủ công sang biến Postman).

## Gate ghi

- `allow_destructive=true` **hoặc** `canaryMode=true` **hoặc** `readiness=true` cho mọi write (trừ login/refresh).

## gRPC / MQTT

- `grpc/README_GRPC_TESTS.md`, `mqtt/README_MQTT_TESTS.md` — **grpcurl** và **mosquitto**, không phải Newman.

## Thứ tự

- Xem `05_PRODUCTION_TEST_EXECUTION_ORDER.md`.
