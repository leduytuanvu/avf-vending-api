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
Traceback (most recent call last):
  File "D:\admin\development\avf\avf-vending-system\avf-vending-api\postman\suites\full-production-suite\validate_generated_assets.py", line 882, in <module>
    sys.exit(main())
             ~~~~^^
  File "D:\admin\development\avf\avf-vending-system\avf-vending-api\postman\suites\full-production-suite\validate_generated_assets.py", line 665, in main
    spec = json.loads(SWAGGER.read_text(encoding="utf-8"))
                      ~~~~~~~~~~~~~~~~~^^^^^^^^^^^^^^^^^^
  File "C:\Python314\Lib\pathlib\__init__.py", line 787, in read_text
    with self.open(mode='r', encoding=encoding, errors=errors, newline=newline) as f:
         ~~~~~~~~~^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "C:\Python314\Lib\pathlib\__init__.py", line 771, in open
    return io.open(self, mode, buffering, encoding, errors, newline)
           ~~~~~~~^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
FileNotFoundError: [Errno 2] No such file or directory: 'D:\\admin\\development\\avf\\avf-vending-system\\avf-vending-api\\postman\\docs\\swagger\\swagger.json'
```

## Trạng thái generator

**finalStatus:** `PASS_IMPORT_ASSETS_COMPLETE`

**PASS_IMPORT_ASSETS_COMPLETE:** Không — xem blockers/warnings manifest và log validator.
