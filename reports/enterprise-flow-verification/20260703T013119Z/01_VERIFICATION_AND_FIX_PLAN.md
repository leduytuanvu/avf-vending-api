# Verification and Fix Plan — Execution Checklist

**Timestamp:** 20260703T013119Z

- [x] P0.1 Re-audit (`00_IMPLEMENTATION_REAUDIT.md`)
- [x] P0.2 Inventories (`REST/GRPC/MQTT/POSTMAN/REQUIREMENTS`)
- [x] P0.3 Plan implementation coverage matrix
- [x] P0.4a OpenAPI stubs for 9 payment/media routes
- [x] P0.4b ClaimContext wiring on activation claim
- [x] P0.4c Operator ended_reason normalization on close
- [x] P0.4d Unified timeline — Option B documented (PARTIAL)
- [x] P0.5 Codegen (`python tools/build_openapi.py`)
- [x] P0.6 Security + enterprise HTTP tests
- [x] P0.7 Coverage runners + matrices
- [x] P1 Fix loop (0 test failures in targeted suites)
- [x] P2 Multi-pass validation (3x Go test green)

## Rerun bundle

```bash
export ENTERPRISE_FLOW_VERIFICATION_UTC=20260703T013119Z
python tools/build_openapi.py
python tools/enterprise_flow/inventory_rest.py
python tools/enterprise_flow/inventory_grpc.py
python tools/enterprise_flow/inventory_mqtt.py
python tools/enterprise_flow/inventory_postman.py
python tools/enterprise_flow/inventory_enterprise_flow_requirements.py --verify
python tools/enterprise_flow/validate_enterprise_flow_contract.py
go test ./internal/httpserver/... ./internal/grpcserver/... ./internal/platform/mqtt/... ./internal/app/activation/... -count=1
python tools/enterprise_flow/test_rest_full_coverage.py --contract-only
python tools/enterprise_flow/test_grpc_full_coverage.py
python tools/enterprise_flow/test_mqtt_full_coverage.py
```
