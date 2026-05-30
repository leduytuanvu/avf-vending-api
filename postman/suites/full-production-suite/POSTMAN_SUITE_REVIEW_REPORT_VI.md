# POSTMAN_SUITE_REVIEW_REPORT_VI

## Thông tin audit

- **Timestamp (UTC):** 2026-05-27T08:41:13.720064+00:00
- **git commit:** 527dda0058596a880cccbb413f77f8d8220c5102
- **git branch:** qa/market-readiness-full-flow-validation

## Đếm từ source of truth / artifact

| Layer | Source count | Artifact count |
|-------|--------------|----------------|
| REST operations (swagger) | 329 | collection requests 329; matrix rows 329 |
| gRPC methods (proto inventory) | 86 | templates 86; matrix rows 86 |
| MQTT rows (topics.go + contract) | 28 | templates/matrix 28 |

## AVF FULL 100 deliverable (canonical operator pack)

| Item | Value |
|------|-------|
| `AVF_FULL_100.postman_collection.json` requests | **329** (expect **329**) |
| `avf_full_100_postman_suite.zip` | generated after variable audit |
| Final verdict (tooling-only) | **READY_TO_IMPORT_POSTMAN_REST_AND_RUN_GRPC_MQTT_ADJACENT** |

> gRPC/MQTT: **grpcurl** + **mosquitto** adjacent scripts — not Newman HTTP.

## Mismatch / blockers (generator pass này)

- Không có blocker count (xem validator nếu FAIL).

## Fixes applied (generator + validator)

- REST URL: luôn object Postman (`raw`, `host`=[`{{baseUrl}}`], `path`[], `query`[]); không còn `url` kiểu chuỗi; folder **99** chỉ mục lục (description), bỏ request GET giả.
- Packaging: bỏ qua thư mục `avf_full_postman_suite/` khi băm manifest + zip (tránh nhân bản do giải nén nhầm zip trong OUT_DIR); validator FAIL nếu thư mục đó tồn tại.
- REST JSON body: ưu tiên `requestBody.content.application/json.example`, sanitize placeholder (email/password/JWT/UUID/Id…), fallback `schema_to_example`; `Content-Type` + `body.options.raw.language=json`.
- OpenAPI: resolve `#/components/parameters/*` cho matrix + request (query/header/path); Idempotency-Key chỉ trên POST/PUT/PATCH/DELETE khi swagger khai báo (GET dù có ref vẫn bỏ qua).
- Auth: `AUTH_PUBLIC_WRITE` cho login/refresh — request bật mặc định, không ép allow_destructive.
- An toàn: pre-request `throw` nếu write (không auth công khai) thiếu một trong allow_destructive | canaryMode | readiness.
- Env: đủ `allow_destructive` / `canaryMode` / `readiness` / `idempotencyKey` / `requestId` / `correlationId` (defaults an toàn).
- Cleanup: xóa `postman/suites/full-production-suite/avf_full_postman_suite/` trước khi generate (tránh suite pollution + validator FAIL).
- REST capture: test script ghi env khi một trong ba gate true; không ghi giá trị rỗng; context `id` cho site/product/machine/order/payment/command/operator.
- REST headers: collection prerequest set `_runtimeRequestId` / `_runtimeCorrelationId`; Idempotency (+ alias `X-Idempotency-Key`) từ env `idempotencyKey` hoặc auto.
- Canary body: chuẩn hoá field `code`/`name`/… với `{{$guid}}`; OpenAPI single-company không thêm query partition ẩn.
- Docs: đồng bộ `docs/testing/05_PRODUCTION_TEST_EXECUTION_ORDER.md`, `AVF_POSTMAN_PRODUCTION.md`, `POSTMAN_VARIABLE_AUDIT_REPORT.md`.
- Audit: `audit_postman_variables.py` sinh báo cáo Markdown (Postman vs OpenAPI vs env).
- Validator: operationId parity (swagger/collection/CSV); REST matrix CSV method/path/tag/summary vs swagger từng operationId; GET /auth/me Bearer; login capture accessToken+refreshToken; canaryMode mặc định false; idempotency; fullMethod unique; env keys; manifest sha256; quét secret (.py/.sh); URL đầy đủ (raw/host/path, {{baseUrl}}, không {param} đơn).
- Docs VI: README chính, 05, gRPC, MQTT — giải thích metrics/swagger, login, gate, giới hạn tuyên bố.

## Files changed

- `postman/suites/full-production-suite/generate_full_postman_suite.py`
- `postman/suites/full-production-suite/validate_generated_assets.py`
- Toàn bộ artefact dưới `postman/suites/full-production-suite/` sau regenerate.

## Lệnh validation đã dùng

```text
python postman/suites/full-production-suite/generate_full_postman_suite.py
python postman/suites/full-production-suite/validate_generated_assets.py
python -m json.tool postman/suites/full-production-suite/AVF_REST_365_FULL.postman_collection.json
python -m json.tool postman/suites/full-production-suite/AVF_PRODUCTION.postman_environment.json
python -m json.tool postman/suites/full-production-suite/AVF_FULL_100.postman_collection.json
python -m json.tool postman/suites/full-production-suite/AVF_FULL_100.postman_environment.json
python -m json.tool postman/suites/full-production-suite/manifest.json
python -m json.tool postman/suites/full-production-suite/grpc/grpc_request_templates.json
python -m json.tool postman/suites/full-production-suite/mqtt/mqtt_request_templates.json
```

## Kết quả validation (stdout/stderr snapshot)

```text
VALIDATION_PASS
openapi_operations: 329
postman_requests: 329
grpc_templates: 86
mqtt_templates: 28
manifest_finalStatus: PASS_IMPORT_ASSETS_COMPLETE
openapi_idempotency_ops: 92
```

## Cổng an toàn

- **Destructive gate:** pre-request throw + request ghi disabled mặc định (trừ login/refresh).
- **Secret scan:** `validate_generated_assets.py` — FAIL nếu khớp mẫu nhạy cảm (validator source được loại trừ).

## Còn tồn đọng

- Không tuyên bố PASS production; cần evidence vận hành (PSP, broker, thiết bị, RBAC).

## Final claim

**PASS_AFTER_FIXES**

> Lưu ý: Báo cáo này phản ánh **validator chạy ngay sau khi ghi manifest** (trước file báo cáo này). Nếu `final claim` là **PASS_IMPORT_ASSETS_COMPLETE**, số liệu REST/gRPC/MQTT khớp và `VALIDATION_PASS`.
