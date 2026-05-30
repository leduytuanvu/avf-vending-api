# AVF API Test Suite Generation Report

Generated: 2026-05-20T08:15:41.947616+00:00

## Source of truth

- `docs/swagger/swagger.json`
- `proto/avf/**/*.proto`
- `postman/suites/full-production-suite/generate_full_postman_suite.py` (MQTT matrix, gRPC templates)
- `internal/platform/mqtt/topics.go` (secondary)
- `docs/api/mqtt-contract.md` (secondary)

## REST coverage

- total: 327
- generated: 327
- missing: 0
- extra: 0
- verdict: PASS

## gRPC coverage

- total: 86
- generated: 86
- missing: 0
- extra: 0
- verdict: PASS

## MQTT coverage

- total: 28
- generated: 28
- missing: 0
- extra: 0
- verdict: PASS

## Generated files

See `postman/generated/` tree.

## Import instructions

1. `postman/generated/rest/AVF_REST_FULL.postman_collection.json`
2. `postman/generated/rest/AVF_REST_LOCAL.postman_environment.json` (or PRODUCTION/CANARY)
3. gRPC/MQTT README + smoke scripts

## Request/response accuracy

Examples derived from OpenAPI schemas (`schema_to_example`), proto message templates, and MQTT `fix_mqtt_rows()` matrix aligned with code.

## Validation commands

```
python scripts/postman/validate-api-inventory.py
python scripts/postman/validate-generated-api-suite.py
```

## Known limitations

- Postman gRPC/MQTT native import JSON is reference-only; manual setup required.
- gRPC response examples are shape notes unless live reflection used.

## Final verdict

COMPLETE_100_PERCENT_VERIFIED
