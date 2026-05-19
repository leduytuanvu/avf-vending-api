# 00 — Kết luận kiểm tra độ đầy đủ (VI)

## Số liệu so với kỳ vọng

| Giao thức | Kỳ vọng | Thực tế (repo) |
|-------------|---------|----------------|
| REST operations | 327 | **327** |
| gRPC methods (proto avf) | 85 | **86** |
| MQTT topic/flow rows | 28 | **28** |

## Phạm vi đã bao phủ

- OpenAPI: `docs/swagger/swagger.json` → **327** REST requests + matrices (`AVF_REST_365_*` legacy naming + `AVF_FULL_100_*`).
- gRPC: toàn bộ RPC trong `proto/avf` (85, gồm bản `avf.v1` song song `avf.internal.v1`) — cột `registeredOnListener`.
- MQTT: 12 legacy ingest + 13 enterprise ingest + 3 outbound API publish (từ `internal/platform/mqtt/topics.go` + `docs/api/mqtt-contract.md`).

## Cần bằng chứng thực tế trên production/canary

- PSP sandbox/live và chữ ký webhook.
- **Hardware / máy canary** thực.
- Broker MQTT + credential (không lưu trong repo).
- Endpoint gRPC (thường private listener).
- DB migration + tài khoản admin đã xác minh.

## Không kết luận PASS production cho đến khi có evidence

- Chỉ ghi **PASS_IMPORT_ASSETS_COMPLETE** khi số khớp và validator **PASS** (manifest `finalStatus`).

## Tài liệu đầy đủ theo repo

- **Chỉ** khẳng định đầy đủ tài liệu/import khi REST=327, gRPC=86, MQTT=28 và validator không phát hiện secret — xem `manifest.json`.

## Output validator

```text
VALIDATION_PASS
openapi_operations: 327
postman_requests: 327
grpc_templates: 86
mqtt_templates: 28
manifest_finalStatus: PASS_IMPORT_ASSETS_COMPLETE
openapi_idempotency_ops: 91
```

## Trạng thái generator

**finalStatus:** `PASS_IMPORT_ASSETS_COMPLETE`

**PASS_IMPORT_ASSETS_COMPLETE:** Có
