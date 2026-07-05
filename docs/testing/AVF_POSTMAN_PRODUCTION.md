# Postman production suite (AVF vending API)

## Dual-output layout

| Format | Path | Use |
|--------|------|-----|
| **JSON v2.1** | [`postman/collections/`](../../postman/collections/) | Newman, CI, classic Postman import |
| **YAML v3** | [`postman/v3/`](../../postman/v3/) | Postman v12 Local Mode / Native Git |
| **Full production suite (JSON)** | [`postman/suites/production-full/`](../../postman/suites/production-full/) | Full OpenAPI + gRPC/MQTT documentation |
| **Full production suite (v3)** | [`postman/v3/suites/production-full/`](../../postman/v3/suites/production-full/) | Local Mode full suite |

## Primary collections (JSON)

- `postman/collections/avf-vending-api.postman_collection.json`
- `postman/collections/avf-vending-api-function-path.postman_collection.json`

Regenerate: `make postman-generate-json` (after `make swagger`).

## Local Mode import

See [`docs/postman/POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md`](../postman/POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md).

Historical `postman/suites/full-production-suite/` was removed in the 2026 cleanup — use `postman/suites/production-full/` (JSON) or `postman/v3/suites/production-full/` (YAML).

Execution order: [05_PRODUCTION_TEST_EXECUTION_ORDER.md](05_PRODUCTION_TEST_EXECUTION_ORDER.md)
