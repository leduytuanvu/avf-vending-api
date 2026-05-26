# Postman enterprise full surface audit (20260525T192300Z-1196-5901)

- Branch: `postman/production-enterprise-project`
- SHA: `775e51caae904717ad667aeb6e288d2e8912957f`
- Generated: 2026-05-26T20:07:30Z

## Sources scanned

- `docs/swagger/swagger.json` (OpenAPI)
- `tests/e2e/production/generated/rest-route-matrix.json`
- `tests/e2e/production/rest-route-overrides.yaml`
- `internal/httpserver/server.go` + mount\* handlers
- `proto/avf/machine/v1/*.proto`
- `internal/grpcserver/machine_grpc_services.go`
- `internal/platform/mqtt/topics.go`
- `postman/production-enterprise/AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json`

## Counts

| Metric | Value |
|--------|------:|
| REST OpenAPI routes | 329 |
| REST covered / skipped | 244 / 85 |
| REST missing (runnable) | 0 |
| REST enterprise requests | 368 |
| gRPC proto RPCs | 69 |
| gRPC registered services | 13 |
| gRPC implemented (unique) | 82 |
| gRPC missing from docs | 0 |
| MQTT canonical rel topics | 13 |
| MQTT missing from docs | 0 |
| Coverage checker | ENTERPRISE_COVERAGE_OK |
| Newman | NEWMAN_BLOCKED_BY_OPERATOR_CREDENTIALS |
| Full E2E | FULL_E2E_BLOCKED_BY_OPERATOR_CREDENTIALS |

## Remaining blockers

- Local private Postman env with production credentials for Newman/import parity
- Optional: `bash tests/e2e/production/run_production_e2e.sh --mode live --suite all-no-online-payment`
- None