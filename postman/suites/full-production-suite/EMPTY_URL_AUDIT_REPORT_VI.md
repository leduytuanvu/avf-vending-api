# EMPTY_URL_AUDIT_REPORT_VI

## Thời điểm audit

- **Timestamp (UTC):** 2026-05-27T08:41:13.720064+00:00
- **Nhánh git:** qa/market-readiness-full-flow-validation
- **Commit:** 527dda0058596a880cccbb413f77f8d8220c5102

## Đếm URL / request

| Chỉ số | Giá trị |
|---------|--------|
| OpenAPI operations | 329 |
| Postman items có `request` (chỉ API thực) | **329** |
| URL hợp lệ sau sửa (validator `validate_collection_urls`) | **329** |
| URL trống/sai sau sửa | **0** (kỳ vọng) |

## Trước sửa (root cause từ generator cũ)

- Khi **không** có query: `request.url` là **chuỗi** thay vì object — một số bản Postman hiển thị "Enter URL or paste text".
- Folder **99**: mỗi tag có request **GET** tới `/swagger/doc.json` chỉ làm mục lục — **không** phải 365 API; đã **xoá request**, giữ **description** trên folder tag.
- Số request giả lục (ước lượng theo tag đầu tiên): **~23**.

## Sau sửa

- Mọi operation: `url` = object với `raw`, `host`, `path` (+ `query` khi có).
- `raw` luôn bắt đầu `{{baseUrl}}`; tham số path OpenAPI `{name}` được chuyển sang biến Postman `{{…}}` (không để sót một ngoặc như `{siteId}`).

## Placeholder / doc đã gỡ hoặc chuyển

- **Đã gỡ:** item `INDEX — mỗi operation chỉ có một request trong 00–15` (GET + url string) trong từng folder tag dưới **99**.
- **Giữ:** folder tag với `description` + `item`: [] (chỉ tài liệu).

## Lệnh validation đã chạy

```text
python postman/suites/full-production-suite/generate_full_postman_suite.py
python postman/suites/full-production-suite/validate_generated_assets.py
python -m json.tool postman/suites/full-production-suite/AVF_REST_365_FULL.postman_collection.json
python -m json.tool postman/suites/full-production-suite/AVF_PRODUCTION.postman_environment.json
python -m json.tool postman/suites/full-production-suite/manifest.json
python -m json.tool postman/suites/full-production-suite/grpc/grpc_request_templates.json
python -m json.tool postman/suites/full-production-suite/mqtt/mqtt_request_templates.json
```

## Kết quả validation (snapshot)

```text
VALIDATION_PASS
openapi_operations: 329
postman_requests: 329
grpc_templates: 86
mqtt_templates: 28
manifest_finalStatus: PASS_IMPORT_ASSETS_COMPLETE
openapi_idempotency_ops: 92
```

## Quét secret (validator)

- **Kết quả:** PASS

## Khẳng định cuối

**PASS_AFTER_FIXES**

> Chứng minh **import URL đầy đủ** + parity OpenAPI; **không** tuyên bố PASS runtime production.
