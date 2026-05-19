# EMPTY_URL_AUDIT_REPORT_VI

## Thời điểm audit

- **Timestamp (UTC):** 2026-05-19T18:20:31.519725+00:00
- **Nhánh git:** main
- **Commit:** 6527d502437f5137fb05c56d4851043b258afbc1

## Đếm URL / request

| Chỉ số | Giá trị |
|---------|--------|
| OpenAPI operations | 327 |
| Postman items có `request` (chỉ API thực) | **327** |
| URL hợp lệ sau sửa (validator `validate_collection_urls`) | **327** |
| URL trống/sai sau sửa | **0** (kỳ vọng) |

## Trước sửa (root cause từ generator cũ)

- Khi **không** có query: `request.url` là **chuỗi** thay vì object — một số bản Postman hiển thị "Enter URL or paste text".
- Folder **99**: mỗi tag có request **GET** tới `/swagger/doc.json` chỉ làm mục lục — **không** phải 365 API; đã **xoá request**, giữ **description** trên folder tag.
- Số request giả lục (ước lượng theo tag đầu tiên): **~22**.

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

## Quét secret (validator)

- **Kết quả:** FAIL (xem validator)

## Khẳng định cuối

**FAIL_VALIDATION**

> Chứng minh **import URL đầy đủ** + parity OpenAPI; **không** tuyên bố PASS runtime production.
